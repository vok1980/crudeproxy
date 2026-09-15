// SPDX-License-Identifier: MIT
// Copyright (c) 2026 vok1980
//
// This file was written with assistance from Claude (Anthropic), used as a
// coding assistant. All code was reviewed and tested by the copyright holder.

package main

import (
	"net/http"
	"strings"
)

// hopHeaders is the fixed set of hop-by-hop headers defined by RFC 7230
// section 6.1. They are meaningful for a single transport leg and must
// not be forwarded.
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// stripHopByHop removes Connection-listed headers and the fixed
// hop-by-hop set from h. Tokens named in the Connection header are
// themselves hop-by-hop, per RFC 7230 section 6.1.
func stripHopByHop(h http.Header) {
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = strings.TrimSpace(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
	for _, k := range hopHeaders {
		h.Del(k)
	}
}
