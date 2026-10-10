// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

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
