// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setBlockList replaces the global block list for the duration of the test
// and restores it afterwards. Tests that call isBlocked must use this helper.
func setBlockList(t *testing.T, list []string) {
	t.Helper()
	blockMutex.Lock()
	old := blockList
	blockList = list
	blockMutex.Unlock()
	t.Cleanup(func() {
		blockMutex.Lock()
		blockList = old
		blockMutex.Unlock()
	})
}

func TestIsBlocked(t *testing.T) {
	setBlockList(t, []string{"facebook.com", "twitter.com", "example.org"})

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"exact", "facebook.com", true},
		{"subdomain", "www.facebook.com", true},
		{"deep subdomain", "m.static.facebook.com", true},
		{"with port", "example.org:443", true},
		{"uppercase", "EXAMPLE.ORG", true},
		{"trailing dot", "example.org.", true},
		{"unrelated", "example.com", false},
		{"suffix without dot", "notfacebook.com", false},
		{"suffix after dot", "facebook.com.evil.com", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlocked(tt.in); got != tt.want {
				t.Errorf("isBlocked(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadBlockList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.txt")
	content := "# comment line\n" +
		"\n" +
		"facebook.com\n" +
		".Twitter.com\n" +
		"  example.org  \n" +
		"# another comment\n" +
		"doubleclick.net.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadBlockList(path); err != nil {
		t.Fatal(err)
	}

	blockMutex.RLock()
	got := append([]string(nil), blockList...)
	blockMutex.RUnlock()

	want := []string{"facebook.com", "twitter.com", "example.org", "doubleclick.net"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadBlockListMissingFile(t *testing.T) {
	if err := loadBlockList("/nonexistent/path/blocked.txt"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestStripHopByHop(t *testing.T) {
	h := http.Header{
		"Connection":          {"keep-alive, X-Custom"},
		"Keep-Alive":          {"timeout=5"},
		"X-Custom":            {"secret"},
		"Proxy-Authorization": {"Basic abc"},
		"Proxy-Connection":    {"keep-alive"},
		"Upgrade":             {"h2c"},
		"Content-Type":        {"application/json"},
		"User-Agent":          {"test/1.0"},
	}
	stripHopByHop(h)

	for _, gone := range []string{
		"Connection", "Keep-Alive", "X-Custom",
		"Proxy-Authorization", "Proxy-Connection", "Upgrade",
	} {
		if _, ok := h[gone]; ok {
			t.Errorf("header %q should have been removed", gone)
		}
	}
	for _, kept := range []string{"Content-Type", "User-Agent"} {
		if _, ok := h[kept]; !ok {
			t.Errorf("header %q should have been kept", kept)
		}
	}
}

func TestIsLoopbackListen(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8888", true},
		{"localhost:8888", true},
		{"[::1]:8888", true},
		{":8888", false},
		{"0.0.0.0:8888", false},
		{"192.168.1.1:8888", false},
		{"example.com:8888", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		if got := isLoopbackListen(tt.addr); got != tt.want {
			t.Errorf("isLoopbackListen(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestTunnelTouchCoalesces(t *testing.T) {
	tun := newTunnel()

	select {
	case <-tun.reset:
		t.Fatal("unexpected reset before any touch")
	default:
	}

	tun.touch()
	select {
	case <-tun.reset:
		// expected
	default:
		t.Fatal("expected reset signal after first touch")
	}

	// Multiple touches without a consumer must not block.
	tun.touch()
	tun.touch()
	select {
	case <-tun.reset:
		// expected
	default:
		t.Fatal("expected reset signal after second touch")
	}

	// And the channel must coalesce: no extra signals queued.
	select {
	case <-tun.reset:
		t.Fatal("reset signal should have been coalesced")
	default:
	}
}

func TestTunnelIdleFor(t *testing.T) {
	tun := newTunnel()
	tun.touch()
	if d := tun.idleFor(); d > 100*time.Millisecond {
		t.Errorf("idleFor immediately after touch = %v, want near zero", d)
	}
}

func TestTunnelConnTouchesOnRead(t *testing.T) {
	tun := newTunnel()
	tc := &tunnelConn{Reader: strings.NewReader("x"), tunnel: tun}

	select {
	case <-tun.reset:
		t.Fatal("unexpected reset before any read")
	default:
	}

	buf := make([]byte, 1)
	if _, err := tc.Read(buf); err != nil {
		t.Fatal(err)
	}

	select {
	case <-tun.reset:
		// expected
	default:
		t.Fatal("expected reset signal after non-zero read")
	}
}

func TestTunnelConnDoesNotTouchOnZeroRead(t *testing.T) {
	tun := newTunnel()
	tc := &tunnelConn{Reader: strings.NewReader(""), tunnel: tun}

	buf := make([]byte, 1)
	n, err := tc.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("got n=%d err=%v, want n=0 err=io.EOF", n, err)
	}

	select {
	case <-tun.reset:
		t.Fatal("zero-byte read should not touch the tunnel")
	default:
		// expected
	}
}

func TestTunnelConnTouchesOnWrite(t *testing.T) {
	tun := newTunnel()
	var sink strings.Builder
	tc := &tunnelConn{Writer: &sink, tunnel: tun}

	n, err := tc.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("got n=%d err=%v", n, err)
	}
	if sink.String() != "hello" {
		t.Fatalf("sink = %q", sink.String())
	}

	select {
	case <-tun.reset:
		// expected
	default:
		t.Fatal("expected reset signal after write")
	}
}

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
