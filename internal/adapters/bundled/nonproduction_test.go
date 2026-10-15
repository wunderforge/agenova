// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"os"
	"path/filepath"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestNonproductionBAUSREPlatformExampleResolves(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "nonproduction", "platform.yaml.example")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, validation := v0.ParsePlatformYAML(data)
	if validation != nil {
		t.Fatal(validation)
	}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	resolved, _, failure := platform.Resolve(document, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(resolved.ToolRoutes) != 7 {
		t.Fatalf("tool routes = %d, want 7", len(resolved.ToolRoutes))
	}
	for _, route := range resolved.ToolRoutes {
		operation := route.Tool.Operation
		if !gateway.SupportedToolOperation(operation) || gateway.ForbiddenToolOperation(operation) {
			t.Fatalf("example contains unsupported operation %q", operation)
		}
		scope, err := gateway.ParseResourceScope(route.Tool.ResourceScope)
		if err != nil {
			t.Fatalf("route %q: %v", route.Profile, err)
		}
		if err := gateway.ValidateToolOperationResource(operation, scope); err != nil {
			t.Fatalf("route %q: %v", route.Profile, err)
		}
	}
}
