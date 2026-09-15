// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"io"
	"sync/atomic"
	"time"
)

// tunnelIdleTimeout is the maximum time either direction of a CONNECT
// tunnel may stay idle before the tunnel is closed. Zero disables the
// watchdog. Set from the -tunnel-idle-timeout flag in main.
var tunnelIdleTimeout *time.Duration

// tunnel tracks activity across both directions of a CONNECT tunnel.
// touch() signals a reset channel so the watchdog can restart its idle
// timer; the timer therefore measures time since the *last* activity,
// not time since an arbitrary tick.
type tunnel struct {
	lastActivity atomic.Int64
	reset        chan struct{}
}

func newTunnel() *tunnel {
	return &tunnel{reset: make(chan struct{}, 1)}
}

func (t *tunnel) touch() {
	t.lastActivity.Store(time.Now().UnixNano())
	select {
	case t.reset <- struct{}{}:
	default:
		// A reset is already pending; the watchdog will consume it
		// on the next iteration. No need to queue another.
	}
}

func (t *tunnel) idleFor() time.Duration {
	return time.Since(time.Unix(0, t.lastActivity.Load()))
}

// tunnelConn wraps a reader/writer and touches the shared tunnel activity
// on every successful operation. It does not set deadlines itself; the
// watchdog goroutine is responsible for closing an idle tunnel.
type tunnelConn struct {
	io.Reader
	io.Writer
	tunnel *tunnel
}

func (c *tunnelConn) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	if n > 0 {
		c.tunnel.touch()
	}
	return n, err
}

func (c *tunnelConn) Write(p []byte) (int, error) {
	n, err := c.Writer.Write(p)
	if n > 0 {
		c.tunnel.touch()
	}
	return n, err
}
