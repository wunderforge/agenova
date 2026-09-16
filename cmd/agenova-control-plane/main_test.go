// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReferenceEndpointsExposeOnlySafeStatus(t *testing.T) {
	t.Setenv("AGENOVA_PLATFORM_NAME", "reference-local")
	t.Setenv("AGENOVA_PLATFORM_REVISION", "sha256:test")
	t.Setenv("AGENOVA_POLICY_REF", "reference-default-deny@1")

	ready := httptest.NewRecorder()
	handler().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK || ready.Body.String() != "ready\n" {
		t.Fatalf("ready = %d %q", ready.Code, ready.Body.String())
	}
	result := httptest.NewRecorder()
	handler().ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"revision":"sha256:test"`) || strings.Contains(strings.ToLower(result.Body.String()), "credential") {
		t.Fatalf("status = %d %q", result.Code, result.Body.String())
	}
}
