// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import "testing"

func TestClientIP(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"IPv4 with port", "192.168.1.10:54321", "192.168.1.10"},
		{"IPv4 without port", "192.168.1.10", "192.168.1.10"},
		{"IPv6 with port", "[::1]:54321", "::1"},
		{"IPv6 in brackets without port", "[::1]", "[::1]"},
		{"IPv6 bare without port", "::1", "::1"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientIP(tt.in); got != tt.want {
				t.Errorf("clientIP(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
