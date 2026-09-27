// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseProxyBasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		wantUser string
		wantPass string
		wantOK   bool
	}{
		{"valid", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret")), "alice", "secret", true},
		{"password with colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:pa:ss")), "alice", "pa:ss", true},
		{"lowercase scheme", "basic " + base64.StdEncoding.EncodeToString([]byte("a:b")), "a", "b", true},
		{"no header", "", "", "", false},
		{"wrong scheme", "Bearer abc", "", "", false},
		{"bad base64", "Basic !!!", "", "", false},
		{"no colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice")), "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, p, ok := parseProxyBasicAuth(tt.header)
			if u != tt.wantUser || p != tt.wantPass || ok != tt.wantOK {
				t.Errorf("got (%q,%q,%v), want (%q,%q,%v)", u, p, ok, tt.wantUser, tt.wantPass, tt.wantOK)
			}
		})
	}
}

func TestLoadAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	content := "# comment\n\nalice:secret\nbob:pa:ss\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := loadAuthFile(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loadAuthFile("") })

	authMutex.RLock()
	got := authUsers
	authMutex.RUnlock()

	if len(got) != 2 || got["alice"] != "secret" || got["bob"] != "pa:ss" {
		t.Fatalf("unexpected users: %#v", got)
	}
}

func TestLoadAuthFileBadLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	if err := os.WriteFile(path, []byte("no-colon-here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadAuthFile(path); err == nil {
		t.Fatal("expected error for line without colon")
	}
}

func TestLoadAuthFileEmptyDisables(t *testing.T) {
	if err := loadAuthFile(""); err != nil {
		t.Fatal(err)
	}
	authMutex.RLock()
	n := len(authUsers)
	authMutex.RUnlock()
	if n != 0 {
		t.Fatalf("expected empty map, got %d users", n)
	}
}

func TestCheckAuthDisabled(t *testing.T) {
	_ = loadAuthFile("")
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	_, ok := checkAuth(req)
	if !ok {
		t.Fatal("auth should be disabled when no users are loaded")
	}
}

func TestCheckAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.txt")
	if err := os.WriteFile(path, []byte("alice:secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadAuthFile(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loadAuthFile("") })

	mkReq := func(user, pass string) *http.Request {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		if user != "" {
			cred := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			req.Header.Set("Proxy-Authorization", "Basic "+cred)
		}
		return req
	}

	user, ok := checkAuth(mkReq("alice", "secret"))
	if !ok {
		t.Error("valid credentials should pass")
	}
	if user != "alice" {
		t.Errorf("expected user alice, got %q", user)
	}

	user, ok = checkAuth(mkReq("alice", "wrong"))
	if ok {
		t.Error("wrong password should fail")
	}
	if user != "alice" {
		t.Errorf("expected user alice even on wrong password, got %q", user)
	}

	user, ok = checkAuth(mkReq("bob", "secret"))
	if ok {
		t.Error("unknown user should fail")
	}
	if user != "bob" {
		t.Errorf("expected user bob even on unknown, got %q", user)
	}

	_, ok = checkAuth(mkReq("", ""))
	if ok {
		t.Error("missing header should fail")
	}
}
