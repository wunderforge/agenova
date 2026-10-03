// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package e16 holds the #178 Slice 3 kind acceptance inputs. This test keeps
// them consistent offline; it never contacts a cluster.
package e16

import (
	"os"
	"slices"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The positive route holds only the positive task's files and the faults route
// only the fault files, so no Work can reach a file meant for another case.
func TestAcceptanceInputsResolveToTwoSeparateRoutes(t *testing.T) {
	input, verr := v0.ParsePlatformYAML(read(t, "platform.yaml"))
	if verr != nil {
		t.Fatal(verr)
	}
	registry, err := bundled.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	resolved, _, failure := platform.Resolve(input, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	routes := map[string][]string{}
	for _, route := range resolved.ToolRoutes {
		if route.Tool.Operation != "repo.read" || route.BackendRef != "e16-mcp" {
			t.Fatalf("unexpected route %+v", route)
		}
		routes[route.Tool.ResourceScope] = route.Tool.AllowedValues
	}
	want := map[string][]string{
		"repo:agenova/e16-fixture": {"README.md", "logs/timeout.log", "src/retry.txt"},
		"repo:agenova/e16-faults":  {"logs/slow.log", "logs/full-trace.log", "notes/incident-timeline.md"},
	}
	if len(routes) != len(want) {
		t.Fatalf("routes %v, want %v", routes, want)
	}
	for scope, files := range want {
		got := slices.Clone(routes[scope])
		slices.Sort(got)
		slices.Sort(files)
		if !slices.Equal(got, files) {
			t.Fatalf("%s allows %v, want %v", scope, routes[scope], files)
		}
	}
	namespace, _ := input.Spec.Infrastructure.Deployment.Config["namespace"].(string)
	if namespace != "agenova-e16-system" {
		t.Fatalf("control plane namespace %q", namespace)
	}

	template, verr := v0.ParseAgentTemplateYAML(read(t, "template.yaml"))
	if verr != nil {
		t.Fatal(verr)
	}
	ceiling := template.Spec.CapabilityCeiling
	if !slices.Equal(ceiling.Tools, []string{"repo.read"}) || !slices.Equal(ceiling.ResourceScopes, []string{"repo:agenova/e16-fixture", "repo:agenova/e16-faults"}) {
		t.Fatalf("template ceiling %+v", ceiling)
	}
	bundle, err := policy.ParseDocumentYAML(read(t, "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Rules) != 1 || bundle.Rules[0].TemplateRef != template.Metadata.Name || bundle.Rules[0].Project != "payments" {
		t.Fatalf("policy rules %+v", bundle.Rules)
	}

	for name, tc := range map[string]struct {
		scope   string
		project string
		file    string
	}{
		"work-positive.yaml":       {scope: "repo:agenova/e16-fixture", project: "payments", file: "logs/timeout.log"},
		"work-n6-timeout.yaml":     {scope: "repo:agenova/e16-faults", project: "payments", file: "logs/slow.log"},
		"work-n7-oversize.yaml":    {scope: "repo:agenova/e16-faults", project: "payments", file: "logs/full-trace.log"},
		"work-n8-truncation.yaml":  {scope: "repo:agenova/e16-faults", project: "payments", file: "notes/incident-timeline.md"},
		"work-admission-deny.yaml": {scope: "repo:agenova/e16-fixture", project: "billing"},
	} {
		request, verr := v0.ParseClaimRequestYAML(read(t, name))
		if verr != nil {
			t.Fatalf("%s: %v", name, verr)
		}
		access := request.Spec.RequestedAccess
		if request.Spec.TemplateRef != template.Metadata.Name || request.Spec.ProjectRef != tc.project || !slices.Equal(access.ResourceScopes, []string{tc.scope}) {
			t.Fatalf("%s: %+v", name, request.Spec)
		}
		objective, _ := request.Spec.Task.Input["objective"].(string)
		if tc.file != "" && !strings.Contains(objective, tc.file) {
			t.Fatalf("%s objective does not name %s", name, tc.file)
		}
	}
}
