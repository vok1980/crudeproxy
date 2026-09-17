// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"bufio"
	"net"
	"os"
	"strings"
	"sync"
)

var (
	blockMutex sync.RWMutex
	blockList  []string
)

// loadBlockList reads a list of blocked domains from a text file. One
// domain per line, lines starting with # are comments, empty lines are
// ignored. Leading and trailing dots are stripped. Domains are matched
// case-insensitively.
func loadBlockList(path string) error {
	list, err := readBlockList(path)
	if err != nil {
		return err
	}
	blockMutex.Lock()
	blockList = list
	blockMutex.Unlock()
	return nil
}

// readBlockList parses a block list file and returns the entries without
// mutating global state.
func readBlockList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var list []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ToLower(line)
		line = strings.TrimPrefix(line, ".")
		line = strings.TrimSuffix(line, ".")
		list = append(list, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// isBlocked reports whether hostport matches any entry in the block list.
// A domain entry blocks the domain itself and all of its subdomains.
func isBlocked(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))

	blockMutex.RLock()
	defer blockMutex.RUnlock()
	for _, b := range blockList {
		if host == b || strings.HasSuffix(host, "."+b) {
			return true
		}
	}
	return false
}
