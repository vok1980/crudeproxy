// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"net/http"
	"testing"
)

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
