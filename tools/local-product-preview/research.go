// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func startMCP() (*exec.Cmd, error) {
	// Never accept the health response of an unrelated, already-running server.
	listener, err := net.Listen("tcp", "127.0.0.1:5216")
	if err != nil {
		return nil, errors.New("local MCP port already in use")
	}
	listener.Close()
	data, err := filepath.Abs("harness/demo/incident-mcp/data")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(".tmp/local-mcp.exe")
	cmd.Env = append(os.Environ(), "FIXTURE_ADDR=127.0.0.1:5216", "FIXTURE_DATA_DIR="+data, "FIXTURE_RESEARCH_ENABLED=true")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: time.Second}
	for i := 0; i < 30; i++ {
		r, e := client.Get("http://127.0.0.1:5216/healthz")
		if e == nil {
			r.Body.Close()
			if r.StatusCode == http.StatusOK {
				return cmd, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	cmd.Wait()
	return nil, os.ErrDeadlineExceeded
}
func researchTools() (*toolbackend.Set, error) {
	return researchToolsAt("http://127.0.0.1:5216/mcp")
}
func researchToolsAt(endpoint string) (*toolbackend.Set, error) {
	backend := map[string]any{"transport": "streamable-http", "protocol-version": "2025-06-18", "endpoint": endpoint, "timeout": "12s", "max-request-bytes": "8192", "max-response-bytes": "65536", "max-observation-bytes": "8192", "max-concurrent-calls": "2"}
	descriptors := []toolbackend.Descriptor{
		{Operation: "web.read", Description: "Read a public HTTPS page by URL. No API key required. Private addresses and redirects are refused.", ResourceScope: "web:public-pages", Parameter: "url", MaxBytes: 1024, InputMode: "text"},
		{Operation: "web.fetch", Description: "Fetch approved public product documentation by source ID. Returns untrusted public text.", ResourceScope: "web:product-comparison", Parameter: "source_id", MaxBytes: 64, AllowedValues: []string{"product-a", "product-b"}},
		{Operation: "crm.read", Description: "Read the synthetic customer-retention dataset.", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", MaxBytes: 64, AllowedValues: []string{"customer-retention"}},
		{Operation: "git.read", Description: "Read synthetic repository evidence.", ResourceScope: "repo:acme/payments", Parameter: "file", MaxBytes: 128, AllowedValues: []string{"README.md", "logs/timeout.log", "src/retry.txt"}},
		{Operation: "git.read", Description: "Read synthetic billing incident evidence.", ResourceScope: "repo:acme/billing", Parameter: "file", MaxBytes: 128, AllowedValues: []string{"billing/INC-B2081/logs.txt", "billing/INC-B2081/job-config.txt"}},
	}
	profiles := []map[string]any{}
	for _, d := range descriptors {
		native := "read_file"
		if d.Operation == "web.read" {
			native = "web_read"
		}
		if d.Operation == "web.fetch" {
			native = "web_fetch"
		}
		if d.Operation == "crm.read" {
			native = "crm_read"
		}
		allowed := ""
		for i, v := range d.AllowedValues {
			if i > 0 {
				allowed += "\n"
			}
			allowed += v
		}
		profile := map[string]any{"logical-operation": d.Operation, "resource-scope": d.ResourceScope, "mcp-tool": native, "parameter-name": d.Parameter, "parameter-max-bytes": strconv.Itoa(d.MaxBytes), "parameter-allowed-values": allowed, "description": d.Description}
		if d.InputMode == "text" {
			profile["parameter-mode"] = "text"
		}
		profiles = append(profiles, profile)
	}
	provider, err := (&bundled.MCPHTTPTool{}).NewToolProvider(backend, profiles)
	if err != nil {
		return nil, err
	}
	bindings := []toolbackend.Binding{}
	for _, d := range descriptors {
		maxObservation := 4096
		if d.InputMode == "text" {
			maxObservation = 8192
		}
		bindings = append(bindings, toolbackend.Binding{Descriptor: d, Provider: provider, Backend: "local-real-mcp", MaxObservationBytes: maxObservation, MaxConcurrentCalls: 2})
	}
	return toolbackend.NewSet(bindings)
}
func seedResearch(ctx context.Context, store identitydb.Store, p v0.Principal) error {
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO agenova_projects VALUES('team-a','research') ON CONFLICT DO NOTHING;INSERT INTO agenova_project_grants VALUES('local-viewer','team-a','research','viewer'),('local-member','team-a','research','member') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	t := app.ReferenceTemplate()
	t.Metadata.Name = "researcher"
	t.Spec.CapabilityCeiling.Tools = []string{"web.fetch", "crm.read"}
	t.Spec.CapabilityCeiling.ResourceScopes = []string{"web:product-comparison", "crm:customer-retention"}
	t.Spec.CapabilityCeiling.NetworkProfiles = nil
	t.Spec.Artifact.Image = "example.invalid/agenova/local-synthetic:acceptance"
	data, _ := json.Marshal(t)
	if _, err := store.ApplyGovernance(ctx, p, "research", "template", data, app.ValidateControlledTemplate); err != nil {
		return err
	}
	// Registrations are immutable. Publish a new identity; never overwrite the
	// original Researcher or its historical authorization snapshots.
	var searchTemplate v0.AgentTemplate
	if err := json.Unmarshal(data, &searchTemplate); err != nil {
		return err
	}
	searchTemplate.Metadata.Name = "researcher-v2"
	searchTemplate.Spec.CapabilityCeiling.Tools = []string{"web.read", "crm.read"}
	searchTemplate.Spec.CapabilityCeiling.ResourceScopes = []string{"web:public-pages", "crm:customer-retention"}
	searchData, _ := json.Marshal(searchTemplate)
	if _, err := store.ApplyGovernance(ctx, p, "research", "template", searchData, app.ValidateControlledTemplate); err != nil {
		return err
	}
	var count int
	if err := store.DB.QueryRowContext(ctx, `SELECT count(*) FROM agenova_project_policy_active WHERE team=$1 AND project='research'`, p.Team).Scan(&count); err != nil {
		return err
	}
	// Upgrade only our initial, known local bootstrap version; compare-and-set
	// prevents a concurrent owner's new activation from being overwritten.
	active := ""
	_ = store.DB.QueryRowContext(ctx, `SELECT version FROM agenova_project_policy_active WHERE team=$1 AND project='research' AND id='research-governance'`, p.Team).Scan(&active)
	if count == 0 || active == "1" || active == "2" {
		doc := []byte(`{"apiVersion":"agenova.io/v1alpha1","kind":"PolicyBundle","metadata":{"name":"research-governance","version":"3"},"spec":{"rules":[{"team":"team-a","project":"research","templateRef":"researcher","action":"claim.create","constraints":{"networkProfiles":{"values":[]}}},{"team":"team-a","project":"research","templateRef":"researcher-v2","action":"claim.create","constraints":{"networkProfiles":{"values":[]}}}]}}`)
		var expected *v0.PolicyReference
		if active != "" {
			expected = &v0.PolicyReference{ID: "research-governance", Version: active}
		}
		_, err := store.ApplyGovernance(ctx, p, "research", "policy", doc, nil, expected)
		return err
	}
	return nil
}
func localPlatformInfo() *console.PlatformInfo {
	return &console.PlatformInfo{Connections: []console.ConnectionInfo{
		{Category: "tool", Name: "public-web-reader", Backend: "local-real-mcp", Operation: "web.read", Scope: "web:public-pages", Tool: "web_read", Status: "Public HTTPS page reader; no API key"},
		{Category: "model", Name: "coding-standard", Backend: "synthetic-local-provider", Model: "Synthetic local response · not a real LLM"},
		{Category: "runtime", Name: "standard-isolated", Backend: "local-synthetic-no-infra-proof"},
		{Category: "tool", Name: "public-research", Backend: "local-real-mcp", Operation: "web.fetch", Scope: "web:product-comparison", Tool: "web_fetch", Endpoint: "http://127.0.0.1:5216/mcp"},
		{Category: "tool", Name: "customer-retention", Backend: "local-real-mcp", Operation: "crm.read", Scope: "crm:customer-retention", Tool: "crm_read", Endpoint: "http://127.0.0.1:5216/mcp"},
		{Category: "tool", Name: "legacy-payment", Backend: "local-real-mcp", Operation: "git.read", Scope: "repo:acme/payments", Tool: "read_file"},
	}}
}
