// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Installing a real Tool backend removes the reference synthetic catalog.
// The explicit MCP compatibility route must preserve its labelled sample data,
// not silently substitute unrelated incidents or add an implicit fallback.
func TestMCPCompatibilityFilesPreserveLegacySyntheticArtifacts(t *testing.T) {
	for name, want := range mockArtifacts {
		data, err := os.ReadFile(filepath.Join("..", "..", "harness", "demo", "incident-mcp", "data", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSuffix(string(data), "\n") != want {
			t.Errorf("explicit compatibility artifact %q differs from the existing labelled fixture", name)
		}
	}
}
