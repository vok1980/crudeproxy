// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	authMutex sync.RWMutex
	authUsers map[string]string
)

// loadAuthFile reads a "user:password" file into memory. An empty path
// disables authentication entirely.
func loadAuthFile(path string) error {
	users, err := readAuthUsers(path)
	if err != nil {
		return err
	}
	authMutex.Lock()
	authUsers = users
	authMutex.Unlock()
	return nil
}

// readAuthUsers parses a user:password file and returns the map without
// mutating global state. An empty path returns an empty map and no error,
// meaning "authentication disabled".
func readAuthUsers(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	users := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		user, pass, ok := strings.Cut(line, ":")
		if !ok || user == "" {
			return nil, fmt.Errorf("%s:%d: expected user:password", path, lineNo)
		}
		users[user] = pass
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Fail-closed: an empty but existing file is a configuration error.
	// The operator explicitly enabled auth by passing -auth-file.
	if len(users) == 0 {
		return nil, fmt.Errorf("%s: no users configured", path)
	}
	return users, nil
}

// parseProxyBasicAuth parses a Proxy-Authorization header value using the
// Basic scheme. It returns the decoded username and password.
func parseProxyBasicAuth(header string) (user, pass string, ok bool) {
	const prefix = "Basic "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return "", "", false
	}
	user, pass, ok = strings.Cut(string(decoded), ":")
	if !ok {
		return "", "", false
	}
	return user, pass, true
}

// checkAuth reports whether the request carries valid proxy credentials.
// If no users are configured, it returns true (auth disabled).
func checkAuth(r *http.Request) bool {
	authMutex.RLock()
	users := authUsers
	authMutex.RUnlock()

	if len(users) == 0 {
		return true
	}

	user, pass, ok := parseProxyBasicAuth(r.Header.Get("Proxy-Authorization"))
	if !ok {
		return false
	}

	expected, exists := users[user]
	if !exists {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pass), []byte(expected)) == 1
}

// authRequired writes a 407 response and returns false if the request
// carries no valid credentials. It returns true otherwise.
func authRequired(w http.ResponseWriter, r *http.Request, client string) bool {
	if checkAuth(r) {
		return true
	}
	reason := "missing credentials"
	if r.Header.Get("Proxy-Authorization") != "" {
		reason = "bad credentials"
	}
	logEvent("AUTHFAIL", client, r.Method, targetHost(r), reason)
	w.Header().Set("Proxy-Authenticate", `Basic realm="crudeproxy"`)
	http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
	return false
}
