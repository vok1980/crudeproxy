// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"os"
	"path/filepath"
	"testing"
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
