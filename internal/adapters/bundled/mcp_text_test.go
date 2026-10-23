// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package bundled

import (
	"github.com/wunderforge/agenova/internal/platform"
	"testing"
)

func TestMCPExplicitTextProfileAndLegacyDefaults(t *testing.T) {
	backend := map[string]any{"transport": "streamable-http", "protocol-version": "2025-06-18", "endpoint": "http://127.0.0.1:5216/mcp", "timeout": "5s", "max-request-bytes": "8192", "max-response-bytes": "65536", "max-observation-bytes": "4096", "max-concurrent-calls": "2"}
	profile := map[string]any{"logical-operation": "web.search", "resource-scope": "web:public-search", "mcp-tool": "web_search", "parameter-name": "query", "parameter-max-bytes": "512", "parameter-mode": "text", "description": "Search public information"}
	canonical, err := canonicalizeMCPProfile(platform.CapabilityTool, backend, profile)
	if err != nil {
		t.Fatal(err)
	}
	d := mcpDescriptor(canonical)
	if d.InputMode != "text" || len(d.AllowedValues) != 0 || d.MaxBytes != 512 {
		t.Fatal(d)
	}
	for _, mode := range []string{"unknown", ""} {
		bad := map[string]any{}
		for k, v := range profile {
			bad[k] = v
		}
		bad["parameter-mode"] = mode
		if _, err := canonicalizeMCPProfile(platform.CapabilityTool, backend, bad); err == nil {
			t.Fatal("invalid mode accepted")
		}
	}
	profile["parameter-allowed-values"] = "README.md"
	if _, err := canonicalizeMCPProfile(platform.CapabilityTool, backend, profile); err == nil {
		t.Fatal("mixed constraints allowed")
	}
}
