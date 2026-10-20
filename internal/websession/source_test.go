// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestLoginSourceTrustBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, remote, header, want string
		trusted, tls               bool
	}{
		{"direct ignores spoof", "192.0.2.1:42", "198.51.100.1", "192.0.2.1", false, true},
		{"nonloopback ignores spoof", "192.0.2.1:42", "198.51.100.1", "192.0.2.1", true, true},
		{"unconfigured proxy", "127.0.0.1:42", "198.51.100.1", "127.0.0.1", false, false},
		{"configured proxy", "127.0.0.1:42", "198.51.100.1", "198.51.100.1", true, false},
		{"missing proxy header", "127.0.0.1:42", "", "", true, false},
		{"invalid proxy header", "127.0.0.1:42", "198.51.100.1, 192.0.2.1", "", true, false},
		{"IPv6 network", "[2001:db8:1::1234]:42", "", "2001:db8:1::/64", false, true},
		{"mapped IPv4", "[::ffff:192.0.2.1]:42", "", "192.0.2.1", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler{c: Config{TrustLocalProxyClientIP: tc.trusted}}
			r := httptest.NewRequest("GET", "http://service.example", nil)
			r.RemoteAddr = tc.remote
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if tc.header != "" {
				r.Header.Set("X-Agenova-Client-IP", tc.header)
			}
			r.Header.Set("X-Forwarded-For", "203.0.113.9")
			r.Header.Set("Forwarded", "for=203.0.113.9")
			got, err := h.loginSource(r)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("got %q, error %v", got, err)
			}
		})
	}
}
