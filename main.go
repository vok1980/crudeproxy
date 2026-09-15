// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	transport = &http.Transport{
		Proxy: nil, // do not use HTTP_PROXY from environment
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
)

// ---------- HTTP ----------

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

// ---------- HTTPS (CONNECT) ----------

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

// ---------- entry point ----------

func main() {
	listen := flag.String("listen", "127.0.0.1:8888", "listen address")
	blockFile := flag.String("block", "blocked.txt", "path to block list file")
	logFile := flag.String("log", "", "log file path (empty means stdout)")
	tunnelIdleTimeout = flag.Duration("tunnel-idle-timeout", 10*time.Minute,
		"idle timeout for CONNECT tunnel directions; 0 disables it")
	flag.Parse()

	if err := loadBlockList(*blockFile); err != nil {
		log.Fatalf("failed to load block list: %v", err)
	}
	blockMutex.RLock()
	log.Printf("loaded %d domains into block list", len(blockList))
	blockMutex.RUnlock()

	var logWriter io.Writer = os.Stdout
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to open log file: %v", err)
		}
		defer f.Close()
		logWriter = f
	}
	accessLog = log.New(logWriter, "", log.LstdFlags)

	if !isLoopbackListen(*listen) {
		log.Printf("WARNING: listening on %s without authentication; "+
			"this is an open proxy — anyone who can reach this port can use it", *listen)
	}

	// SIGHUP triggers block list reload without restart
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	go func() {
		for range sigCh {
			if err := loadBlockList(*blockFile); err != nil {
				log.Printf("reload: %v", err)
				continue
			}
			blockMutex.RLock()
			log.Printf("reload: %d domains", len(blockList))
			blockMutex.RUnlock()
		}
	}()

	server := &http.Server{
		Addr:              *listen,
		Handler:           http.HandlerFunc(dispatch),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("crudeproxy listening on %s", *listen)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func dispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		handleConnect(w, r)
		return
	}
	handleHTTP(w, r)
}
