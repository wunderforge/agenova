// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import "testing"

func TestSafeDiagnosticURL(t *testing.T) {
	for _, value := range []string{"/grafana", "https://diagnostics.example.test/tempo", "http://127.0.0.1:3000"} {
		if SafeDiagnosticURL(value) == "" {
			t.Fatal("safe navigation rejected", value)
		}
	}
	for _, value := range []string{"//evil.test", "/login", "https://user:password@host.test", "https://host.test?token=abc", "https://host.test#token=abc", "http://remote.test", "javascript:alert(1)"} {
		if SafeDiagnosticURL(value) != "" {
			t.Fatal("unsafe navigation accepted")
		}
	}
}
