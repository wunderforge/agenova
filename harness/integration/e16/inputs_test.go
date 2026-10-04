// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package e16 holds the #178 Slice 3 kind acceptance inputs. This test keeps
// them consistent offline; it never contacts a cluster.
package e16

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// kubelet can enforce runAsNonRoot only for a numeric image user; a named one
// ("nonroot") leaves the container in CreateContainerConfigError. Every image
// whose manifest requires it must end on a numeric, non-zero USER.
func TestNonRootManifestsUseNumericImageUsers(t *testing.T) {
	for manifest, dockerfile := range map[string]string{
		"../mcpfixture/deploy.yaml": "../mcpfixture/Dockerfile",
		"probe-job.yaml":            "probe.Dockerfile",
	} {
		if !strings.Contains(string(read(t, manifest)), "runAsNonRoot: true") {
			t.Fatalf("%s no longer requires runAsNonRoot; update this test", manifest)
		}
		user := ""
		for _, line := range strings.Split(string(read(t, dockerfile)), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && strings.EqualFold(fields[0], "USER") {
				user = fields[1]
			}
		}
		uid, _, _ := strings.Cut(user, ":")
		if n, err := strconv.Atoi(uid); err != nil || n <= 0 {
			t.Errorf("%s ends on USER %q, but %s sets runAsNonRoot, which needs a numeric non-zero user", dockerfile, user, manifest)
		}
	}
}

// Each route reaches only its own case's files through its own backend: the
// positive route holds only the positive task's files, the faults route only
// the fault files, and each Slice 4 token route only README.md through its own
// token backend. No Work can reach a file or backend meant for another case.
func TestAcceptanceInputsResolveToSeparateRoutes(t *testing.T) {
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
	type route struct {
		backend string
		files   []string
	}
	routes := map[string]route{}
	for _, r := range resolved.ToolRoutes {
		if r.Tool.Operation != "repo.read" {
			t.Fatalf("unexpected route %+v", r)
		}
		if _, ok := routes[r.Tool.ResourceScope]; ok {
			t.Fatalf("two routes for %s", r.Tool.ResourceScope)
		}
		routes[r.Tool.ResourceScope] = route{backend: r.BackendRef, files: r.Tool.AllowedValues}
	}
	want := map[string]route{
		"repo:agenova/e16-fixture":       {backend: "e16-mcp", files: []string{"README.md", "logs/timeout.log", "src/retry.txt"}},
		"repo:agenova/e16-faults":        {backend: "e16-mcp", files: []string{"logs/slow.log", "logs/full-trace.log", "notes/incident-timeline.md"}},
		"repo:agenova/e16-token":         {backend: "e16-mcp-token", files: []string{"README.md"}},
		"repo:agenova/e16-token-missing": {backend: "e16-mcp-token-missing", files: []string{"README.md"}},
		"repo:agenova/e16-token-wrong":   {backend: "e16-mcp-token-wrong", files: []string{"README.md"}},
	}
	if len(routes) != len(want) {
		t.Fatalf("routes %v, want %v", routes, want)
	}
	for scope, w := range want {
		got := slices.Clone(routes[scope].files)
		slices.Sort(got)
		slices.Sort(w.files)
		if !slices.Equal(got, w.files) {
			t.Fatalf("%s allows %v, want %v", scope, routes[scope].files, w.files)
		}
		if routes[scope].backend != w.backend {
			t.Fatalf("%s uses backend %q, want %q", scope, routes[scope].backend, w.backend)
		}
	}
	// evidence/work.go judges N8 against this cap.
	for _, route := range resolved.ToolRoutes {
		if route.MaxObservationBytes != 4096 {
			t.Fatalf("route %s observation cap %d, the Work oracle assumes 4096", route.Profile, route.MaxObservationBytes)
		}
	}
	namespace, _ := input.Spec.Infrastructure.Deployment.Config["namespace"].(string)
	if namespace != "agenova-e16-system" {
		t.Fatalf("control plane namespace %q", namespace)
	}

	// e16-mcp stays credential-free on /mcp. Each token backend names its own
	// Secret, is otherwise configured exactly like e16-mcp and uses the
	// token-required path. The runner creates and checks these Secret names.
	const endpoint = "http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp"
	const reference = "provisional-token-secret"
	backends := map[string]struct{ endpoint, secret string }{
		"e16-mcp":               {endpoint: endpoint},
		"e16-mcp-token":         {endpoint: endpoint + "-token", secret: "e16-mcp-token/token"},
		"e16-mcp-token-missing": {endpoint: endpoint + "-token", secret: "e16-mcp-token-absent/token"},
		"e16-mcp-token-wrong":   {endpoint: endpoint + "-token", secret: "e16-mcp-token-wrong/token"},
	}
	configs := map[string]map[string]any{}
	for _, backend := range input.Spec.Services.ToolBackends {
		configs[backend.Name] = backend.Config
	}
	if len(configs) != len(backends) || len(input.Spec.Services.ToolBackends) != len(backends) {
		t.Fatalf("tool backends %v, want exactly %v", configs, backends)
	}
	base := configs["e16-mcp"]
	for name, w := range backends {
		config, ok := configs[name]
		if !ok {
			t.Fatalf("no tool backend %s", name)
		}
		if got, _ := config["endpoint"].(string); got != w.endpoint {
			t.Fatalf("%s endpoint %q, want %q", name, got, w.endpoint)
		}
		secret, has := config[reference]
		if w.secret == "" && has {
			t.Fatalf("%s must stay credential-free, has %s %v", name, reference, secret)
		}
		if got, _ := secret.(string); w.secret != "" && got != w.secret {
			t.Fatalf("%s %s %q, want %q", name, reference, got, w.secret)
		}
		for key, value := range base {
			if key != "endpoint" && config[key] != value {
				t.Fatalf("%s %s is %v, e16-mcp has %v", name, key, config[key], value)
			}
		}
		for key := range config {
			if _, ok := base[key]; !ok && key != reference {
				t.Fatalf("%s sets %s, which e16-mcp does not", name, key)
			}
		}
	}
	// Resolution keeps each reference on its own backend and adds none to e16-mcp.
	instances := 0
	for _, instance := range resolved.Instances {
		if instance.Category != platform.CapabilityTool {
			continue
		}
		instances++
		w, ok := backends[instance.Name]
		if !ok {
			t.Fatalf("unexpected resolved tool backend %s", instance.Name)
		}
		secret, has := instance.Config[reference]
		if got, _ := secret.(string); has != (w.secret != "") || got != w.secret {
			t.Fatalf("resolved %s %s %v, want %q", instance.Name, reference, secret, w.secret)
		}
	}
	if instances != len(backends) {
		t.Fatalf("%d resolved tool backends, want %d", instances, len(backends))
	}

	template, verr := v0.ParseAgentTemplateYAML(read(t, "template.yaml"))
	if verr != nil {
		t.Fatal(verr)
	}
	ceiling := template.Spec.CapabilityCeiling
	if !slices.Equal(ceiling.Tools, []string{"repo.read"}) || !slices.Equal(ceiling.ResourceScopes, []string{"repo:agenova/e16-fixture", "repo:agenova/e16-faults", "repo:agenova/e16-token", "repo:agenova/e16-token-missing", "repo:agenova/e16-token-wrong"}) {
		t.Fatalf("template ceiling %+v", ceiling)
	}
	bundle, err := policy.ParseDocumentYAML(read(t, "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Rules) != 1 || bundle.Rules[0].TemplateRef != template.Metadata.Name || bundle.Rules[0].Project != "payments" {
		t.Fatalf("policy rules %+v", bundle.Rules)
	}

	works := map[string]struct {
		scope   string
		project string
		file    string
		facts   []string
	}{
		"work-positive.yaml": {scope: "repo:agenova/e16-fixture", project: "payments", file: "logs/timeout.log", facts: []string{
			"deadline_resets_each_attempt", "first_attempt_seconds", "backoff_seconds", "total_deadline_seconds", "budget_exceeded", "stable_idempotency_key_needed",
			"fix_shares_one_deadline_across_attempts", "fix_reuses_one_idempotency_key_across_retries"}},
		"work-n6-timeout.yaml":     {scope: "repo:agenova/e16-faults", project: "payments", file: "logs/slow.log"},
		"work-n7-oversize.yaml":    {scope: "repo:agenova/e16-faults", project: "payments", file: "logs/full-trace.log"},
		"work-n8-truncation.yaml":  {scope: "repo:agenova/e16-faults", project: "payments", file: "notes/incident-timeline.md", facts: []string{"timeline_complete"}},
		"work-admission-deny.yaml": {scope: "repo:agenova/e16-fixture", project: "billing"},
		"work-token-valid.yaml":    {scope: "repo:agenova/e16-token", project: "payments", file: "README.md"},
		"work-token-missing.yaml":  {scope: "repo:agenova/e16-token-missing", project: "payments", file: "README.md"},
		"work-token-wrong.yaml":    {scope: "repo:agenova/e16-token-wrong", project: "payments", file: "README.md"},
	}
	// The runner runs any work-<case>.yaml, so every one is checked here.
	files, err := filepath.Glob("work-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(works) {
		t.Fatalf("work files %v, want exactly the %d checked here", files, len(works))
	}
	tokenObjectives := map[string]bool{}
	for name, tc := range works {
		request, verr := v0.ParseClaimRequestYAML(read(t, name))
		if verr != nil {
			t.Fatalf("%s: %v", name, verr)
		}
		access := request.Spec.RequestedAccess
		if request.Spec.TemplateRef != template.Metadata.Name || request.Spec.ProjectRef != tc.project || !slices.Equal(access.ResourceScopes, []string{tc.scope}) {
			t.Fatalf("%s: %+v", name, request.Spec)
		}
		// campaign.sh render_work renames exactly this request name per attempt.
		if want := "e16-" + strings.TrimSuffix(strings.TrimPrefix(name, "work-"), ".yaml"); request.Metadata.Name != want {
			t.Fatalf("%s names its request %q, want %q", name, request.Metadata.Name, want)
		}
		if !slices.Equal(access.Tools, []string{"repo.read"}) {
			t.Fatalf("%s requests tools %v", name, access.Tools)
		}
		objective, _ := request.Spec.Task.Input["objective"].(string)
		if tc.file != "" && !strings.Contains(objective, tc.file) {
			t.Fatalf("%s objective does not name %s", name, tc.file)
		}
		for _, key := range tc.facts {
			if !strings.Contains(objective, "\n"+key+": ") {
				t.Fatalf("%s objective does not ask for the %s fact line", name, key)
			}
		}
		if strings.HasPrefix(name, "work-token-") {
			tokenObjectives[objective] = true
		}
	}
	// The token cases differ only in their backend, never in their task.
	if len(tokenObjectives) != 1 {
		t.Fatalf("the token Works ask %d different objectives, want one", len(tokenObjectives))
	}
}
