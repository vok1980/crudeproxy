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

// This file contains only the entry point (main) and the top-level
// dispatcher (dispatch). Everything else lives in sibling files in the
// same package: blocklist.go, log.go, headers.go, tunnel.go, proxy.go,
// util.go.

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
