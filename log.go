// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"log"
	"net"
)

var accessLog *log.Logger

// clientIP extracts the IP address from a "host:port" remote address.
func clientIP(remoteAddr string) string {
	if ip, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return ip
	}
	return remoteAddr
}

// logEvent writes one tab-separated line to the access log. The extra
// argument is optional; when nil, the line ends after the host.
func logEvent(status, client, user, method, host string, extra interface{}) {
	if extra != nil {
		accessLog.Printf("%s\t%s\t%s\t%s\t%s\t%v", status, client, user, method, host, extra)
	} else {
		accessLog.Printf("%s\t%s\t%s\t%s\t%s", status, client, user, method, host)
	}
}
