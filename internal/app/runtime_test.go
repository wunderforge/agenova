// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/operator"
)

func TestNewRuntimeHostsMemoryBackend(t *testing.T) {
	t.Parallel()

	backend, name, err := NewRuntime("")
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if name != MemoryBackend {
		t.Fatalf("resolved name %q, want %q", name, MemoryBackend)
	}
	runtime, ok := backend.(*operator.Runtime)
	if !ok {
		t.Fatalf("hosted %T, want *operator.Runtime", backend)
	}
	if err := runtime.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "cli-host"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agenova/cli:dev"},
	}); err != nil {
		t.Fatalf("reference backend rejected a template: %v", err)
	}
}

func TestNewRuntimeRejectsProviderBackends(t *testing.T) {
	t.Parallel()

	_, _, err := NewRuntime("kubernetes")
	if err == nil {
		t.Fatal("expected unknown backend error")
	}
	if !strings.Contains(err.Error(), `unknown runtime backend "kubernetes"`) {
		t.Fatalf("error %q", err)
	}
	if !strings.Contains(err.Error(), `supports "memory"`) {
		t.Fatalf("error should name the supported backend: %q", err)
	}
}
