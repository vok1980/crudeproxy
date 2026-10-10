// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"testing"
)

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
