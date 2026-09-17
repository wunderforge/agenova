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
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"revision":"sha256:test"`) || !strings.Contains(result.Body.String(), `"state":"installation-ready"`) || !strings.Contains(result.Body.String(), `"readinessScope":"installation-components"`) || !strings.Contains(result.Body.String(), `"providerHealth":"not-checked"`) || strings.Contains(strings.ToLower(result.Body.String()), "credential") {
		t.Fatalf("status = %d %q", result.Code, result.Body.String())
	}
}

func TestCompatibleWorkerImageMustMatchInstalledRuntime(t *testing.T) {
	for _, test := range []struct {
		image, allowed string
		wantErr        bool
	}{
		{"agenova-testworker:kind", "agenova-testworker:kind", false},
		{"other/worker:latest", "agenova-testworker:kind", true},
		{"agenova-testworker:kind ", "agenova-testworker:kind", true},
		{"agenova-testworker:kind", "", true},
	} {
		err := requireCompatibleWorkerImage(test.image, test.allowed)
		if (err != nil) != test.wantErr {
			t.Fatalf("image=%q allowed=%q: err=%v, wantErr=%t", test.image, test.allowed, err, test.wantErr)
		}
	}
}
