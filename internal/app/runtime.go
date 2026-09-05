// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

// MemoryBackend is the only backend this composition root can construct.
const MemoryBackend = "memory"

// NewRuntime constructs a RuntimeBackend for the agenova executable.
// An empty name selects the in-memory reference backend. Unknown names fail
// without loading a provider adapter.
func NewRuntime(backendName string) (runtime.RuntimeBackend, string, error) {
	name := strings.TrimSpace(backendName)
	if name == "" {
		name = MemoryBackend
	}
	switch name {
	case MemoryBackend:
		return operator.NewRuntime(), MemoryBackend, nil
	default:
		return nil, "", fmt.Errorf("unknown runtime backend %q\nThis composition root supports %q (the in-memory reference backend).\nProvider backends are not selected from the CLI", name, MemoryBackend)
	}
}
