// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package bundled

import (
	"github.com/wunderforge/agenova/internal/platform"
	"testing"
)

func TestRuntimeNetworkConfigurationDoesNotInventVerifiedEnforcement(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"connection": map[string]any{"mode": "in-cluster", "namespace": "agenova-system"}, "compatible-worker-protocol": "controlled-v1"}
	}
	legacy, err := canonicalizeAgentSandboxInstance(platform.CapabilityRuntime, base())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := legacy["network-policy-mode"]; ok {
		t.Fatal("omission cannot attest startup isolation")
	}
	strict := base()
	strict["network-policy-mode"] = "strict"
	got, err := canonicalizeAgentSandboxInstance(platform.CapabilityRuntime, strict)
	if err != nil || got["network-policy-mode"] != "strict" {
		t.Fatalf("strict declaration: %v %v", got, err)
	}
	for _, v := range []any{"standard", "disabled", "", true} {
		bad := base()
		bad["network-policy-mode"] = v
		if _, err := canonicalizeAgentSandboxInstance(platform.CapabilityRuntime, bad); err == nil {
			t.Fatalf("accepted unsupported declaration %v", v)
		}
	}
}
