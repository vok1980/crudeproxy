// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"io"
	"net"
	"net/http"
	"time"
)

// transport is the shared HTTP transport for forward-proxied requests.
// Proxy is explicitly nil so we never re-enter ourselves via HTTP_PROXY
// from the environment.
var transport = &http.Transport{
	Proxy: nil,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// handleHTTP forwards a plain HTTP request to the origin server. The
// request target must be in absolute form (http://host/path), which is
// what clients send to a forward proxy.
func handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL == nil || r.URL.Host == "" {
		http.Error(w, "crudeproxy: missing absolute URL", http.StatusBadRequest)
		return
	}

	host := r.URL.Host
	client := clientIP(r.RemoteAddr)

	if isBlocked(host) {
		logEvent("BLOCK", client, r.Method, host, r.URL.String())
		http.Error(w, "Blocked by crudeproxy", http.StatusForbidden)
		return
	}

	stripHopByHop(r.Header)
	r.RequestURI = "" // required for client-side transport

	resp, err := transport.RoundTrip(r)
	if err != nil {
		logEvent("ERROR", client, r.Method, host, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	stripHopByHop(resp.Header)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logEvent("ERROR", client, r.Method, host, err)
		return
	}

	logEvent("ALLOW", client, r.Method, host, resp.StatusCode)
}

// handleConnect establishes a TCP tunnel for an HTTPS CONNECT request.
// The domain is filtered before dialing; the tunnel itself is opaque,
// so no TLS interception happens here.
func handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	client := clientIP(r.RemoteAddr)

	if isBlocked(host) {
		logEvent("BLOCK", client, "CONNECT", host, nil)
		http.Error(w, "Blocked by crudeproxy", http.StatusForbidden)
		return
	}

	destConn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		logEvent("ERROR", client, "CONNECT", host, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Write the 200 via the buffered writer and flush, otherwise
	// buffered bytes already read from the client may be lost.
	if _, err := bufrw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		logEvent("ERROR", client, "CONNECT", host, err)
		return
	}
	if err := bufrw.Flush(); err != nil {
		logEvent("ERROR", client, "CONNECT", host, err)
		return
	}

	logEvent("ALLOW", client, "CONNECT", host, nil)

	idle := *tunnelIdleTimeout
	if idle <= 0 {
		// Idle timeout disabled: preserve the previous behavior.
		done := make(chan struct{}, 2)
		go func() {
			io.Copy(destConn, bufrw)
			if tc, ok := destConn.(*net.TCPConn); ok {
				_ = tc.CloseWrite()
			}
			done <- struct{}{}
		}()
		go func() {
			io.Copy(clientConn, destConn)
			if tc, ok := clientConn.(*net.TCPConn); ok {
				_ = tc.CloseWrite()
			}
			done <- struct{}{}
		}()
		<-done
		<-done
		return
	}

	t := newTunnel()

	clientSide := &tunnelConn{Reader: bufrw, Writer: clientConn, tunnel: t}
	destSide := &tunnelConn{Reader: destConn, Writer: destConn, tunnel: t}

	// Watchdog: fires `idle` after the last touch. Each touch resets
	// the timer, so the timeout is a strict upper bound on inactivity,
	// not on wall-clock since the last tick.
	stopWatchdog := make(chan struct{})
	go func() {
		timer := time.NewTimer(idle)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				if t.idleFor() >= idle {
					_ = clientConn.Close()
					_ = destConn.Close()
					return
				}
				timer.Reset(idle)
			case <-t.reset:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idle)
			case <-stopWatchdog:
				return
			}
		}
	}()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(destSide, clientSide) // client -> origin
		if tc, ok := destConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientSide, destSide) // origin -> client
		if tc, ok := clientConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	close(stopWatchdog)
}
