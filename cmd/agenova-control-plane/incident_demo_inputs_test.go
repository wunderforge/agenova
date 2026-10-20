// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
)

func incidentPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "harness", "demo", "incident-mcp"}, parts...)...)
}
func incidentRead(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(incidentPath(name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

type incidentTemplate struct{ value *v0.AgentTemplate }

func (r incidentTemplate) Lookup(name string) (*v0.AgentTemplate, error) { return r.value, nil }
func incidentPrepared(t *testing.T, name string) app.PreparedAssignment {
	t.Helper()
	template, validation := v0.ParseAgentTemplateYAML(incidentRead(t, "template.yaml"))
	if validation != nil {
		t.Fatal(validation)
	}
	b, e := policy.ParseDocumentYAML(incidentRead(t, "policy.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	loader := &policy.Loader{}
	if e = loader.Load(b); e != nil {
		t.Fatal(e)
	}
	principal, e := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	if e != nil {
		t.Fatal(e)
	}
	p, e := app.PrepareAssignment(incidentRead(t, name), principal, loader, incidentTemplate{template})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestIncidentDemoInputs(t *testing.T) {
	a, b := incidentPrepared(t, "work-payments.yaml"), incidentPrepared(t, "work-billing.yaml")
	for i, p := range []app.PreparedAssignment{a, b} {
		scope := []string{"repo:demo/payments", "repo:demo/billing"}[i]
		if p.Admission.Decision.Result != v0.DecisionResultAllow || !reflect.DeepEqual(p.Issued.EffectiveAuthority.ResourceScopes, []string{scope}) || !reflect.DeepEqual(p.Issued.EffectiveAuthority.Tools, []string{"repo.read"}) {
			t.Fatalf("unexpected authority for task %d", i)
		}
		if len(p.Request.Spec.RequestedAccess.NetworkProfiles) != 0 || len(p.Issued.EffectiveAuthority.NetworkProfiles) != 0 {
			t.Fatalf("incident task %d unexpectedly requests or receives native networking", i)
		}
	}
	if a.Issued.Principal != b.Issued.Principal || a.Issued.PolicyRef != b.Issued.PolicyRef || a.Request.Spec.TemplateRef != b.Request.Spec.TemplateRef {
		t.Fatal("demo changed user/template/policy")
	}
	denied := incidentPrepared(t, "work-denied.yaml")
	if denied.Admission.Decision.Result != v0.DecisionResultDeny || denied.Issued.Claim != nil {
		t.Fatal("denied request issued a claim")
	}
	input, validation := v0.ParsePlatformYAML(incidentRead(t, "platform.yaml"))
	if validation != nil {
		t.Fatal(validation)
	}
	registry, e := bundled.NewRegistry()
	if e != nil {
		t.Fatal(e)
	}
	resolved, _, failure := platform.Resolve(input, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(resolved.ToolRoutes) != 3 {
		t.Fatal("expected two incident routes and one explicit legacy compatibility route")
	}
	tools, e := buildInstalledTools(resolved, registry, nil)
	if e != nil {
		t.Fatal(e)
	}
	legacy := &v0.EffectiveAuthority{Tools: []string{"git.read"}, ResourceScopes: []string{"repo:acme/payments"}, ModelProfile: "coding-standard", Runtime: v0.EffectiveAuthorityRuntime{ProfileRef: "standard-isolated"}}
	if e := validateInstalledAuthority(legacy, map[string]string{"coding-standard": "local-model"}, map[string]bool{"standard-isolated": true}, tools); e != nil {
		t.Fatalf("MCP installation removed the legacy engineer route: %v", e)
	}
	seen := map[string]bool{}
	for _, route := range resolved.ToolRoutes {
		if route.Tool.Operation == "git.read" {
			if route.Tool.ResourceScope != "repo:acme/payments" || !reflect.DeepEqual(route.Tool.AllowedValues, []string{"README.md", "logs/timeout.log", "src/retry.txt"}) {
				t.Fatal("legacy compatibility route changed scope or files")
			}
			for _, name := range route.Tool.AllowedValues {
				if !strings.HasPrefix(string(incidentRead(t, filepath.Join("data", filepath.FromSlash(name)))), "SYNTHETIC MOCK:") {
					t.Fatal("legacy compatibility content lost its synthetic label")
				}
			}
			continue
		}
		if route.Tool.Operation != "repo.read" || len(route.Tool.AllowedValues) != 2 {
			t.Fatal("unexpected tool catalog")
		}
		directory := "payments/"
		if route.Tool.ResourceScope == "repo:demo/billing" {
			directory = "billing/"
		}
		for _, name := range route.Tool.AllowedValues {
			if seen[name] || !strings.HasPrefix(name, directory) {
				t.Fatal("overlapping/wrong resource paths")
			}
			seen[name] = true
			data := incidentRead(t, filepath.Join("data", filepath.FromSlash(name)))
			if !strings.Contains(string(data), "SYNTHETIC DEMO DATA") || len(data) > 4096 {
				t.Fatal("unlabelled or oversized data")
			}
		}
	}
}
