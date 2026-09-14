// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980

package main

import (
	"log"
	"net/http"
)

func main() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("got request: %s %s", r.Method, r.Host)
		http.Error(w, "crudeproxy: not implemented yet", http.StatusNotImplemented)
	})

	log.Println("crudeproxy listening :8888")
	if err := http.ListenAndServe(":8888", handler); err != nil {
		log.Fatal(err)
	}
}
