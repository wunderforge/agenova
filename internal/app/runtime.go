// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

// MemoryBackend is the only backend this composition root can construct.
const MemoryBackend = "memory"

// NewRuntime constructs the reduced-contract RuntimeBackend for the agenova
// executable. An empty name selects the in-memory reference backend, which
// also remains the claim state owner behind the gateways' ClaimReader.
// Unknown names fail without loading a provider adapter.
func NewRuntime(backendName string) (runtime.RuntimeBackend, string, error) {
	name := strings.TrimSpace(backendName)
	if name == "" {
		name = MemoryBackend
	}
	switch name {
	case MemoryBackend:
		backend := operator.NewRuntime()
		if err := backend.AddTemplate(v1alpha1.AgentSandboxTemplate{
			Metadata: v1alpha1.ObjectMeta{Name: ReferenceRuntimeTemplateRef},
			Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agenova/reference-worker:dev"},
		}); err != nil {
			return nil, "", fmt.Errorf("configure reference runtime template: %w", err)
		}
		if err := backend.AddWarmPool(v1alpha1.SandboxWarmPool{
			Metadata: v1alpha1.ObjectMeta{Name: "reference-engineer-pool"},
			Spec: v1alpha1.SandboxWarmPoolSpec{
				TemplateRef: ReferenceRuntimeTemplateRef,
				Replicas:    1,
			},
		}); err != nil {
			return nil, "", fmt.Errorf("configure reference runtime pool: %w", err)
		}
		return backend, MemoryBackend, nil
	default:
		return nil, "", fmt.Errorf("unknown runtime backend %q\nThis composition root supports %q (the in-memory reference backend).\nProvider backends are not selected from the CLI", name, MemoryBackend)
	}
}
