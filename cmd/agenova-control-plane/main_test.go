// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
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
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"revision":"sha256:test"`) || !strings.Contains(result.Body.String(), `"initialPolicyRef":"reference-default-deny@1"`) || strings.Contains(result.Body.String(), `"policy":`) || !strings.Contains(result.Body.String(), `"state":"installation-ready"`) || !strings.Contains(result.Body.String(), `"readinessScope":"installation-components"`) || !strings.Contains(result.Body.String(), `"providerHealth":"not-checked"`) || strings.Contains(strings.ToLower(result.Body.String()), "credential") {
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

func TestInstalledGatewayAuthorityRejectsUnimplementedGrantsBeforeWorkerSetup(t *testing.T) {
	models := map[string]string{"coding-standard": "llama3.1:latest"}
	runtimes := map[string]bool{"standard-isolated": true}
	for _, test := range []struct {
		name      string
		modify    func(*v0.EffectiveAuthority)
		wantError string
	}{
		{"supported", func(*v0.EffectiveAuthority) {}, ""},
		{"unsupported tool", func(a *v0.EffectiveAuthority) { a.Tools = []string{"git.write"} }, "granted tool is not supported"},
		{"mixed tools", func(a *v0.EffectiveAuthority) { a.Tools = []string{"git.read", "github.pull-request"} }, "granted tool is not supported"},
		{"memory", func(a *v0.EffectiveAuthority) { a.MemoryScopes = []string{"team-docs"} }, "granted memory scope is not supported"},
		{"missing model", func(a *v0.EffectiveAuthority) { a.ModelProfile = "other" }, "granted model profile is not installed"},
		{"missing runtime", func(a *v0.EffectiveAuthority) { a.Runtime.ProfileRef = "other" }, "granted runtime profile is not installed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			authority := &v0.EffectiveAuthority{Tools: []string{"git.read"}, ModelProfile: "coding-standard", Runtime: v0.EffectiveAuthorityRuntime{ProfileRef: "standard-isolated"}}
			test.modify(authority)
			err := validateInstalledAuthority(authority, models, runtimes)
			if test.wantError == "" && err != nil || test.wantError != "" && (err == nil || !strings.Contains(strings.ToLower(err.Error()), test.wantError)) {
				t.Fatalf("validateInstalledAuthority() = %v, want %q", err, test.wantError)
			}
		})
	}
	if err := validateInstalledAuthority(nil, models, runtimes); err == nil {
		t.Fatal("missing issued authority was accepted")
	}
}
