// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/platform"
	"strings"
	"testing"
)

type catalogAccess struct{}

func (catalogAccess) Authorize(_ context.Context, _ v0.Principal, action, project string) error {
	if project == "payments" && action == "work.submit" {
		return nil
	}
	return errors.New("forbidden")
}
func TestCatalogScopesDiscoverySeparatelyFromSubmission(t *testing.T) {
	p := v0.Principal{Subject: "viewer", Team: "team-a"}
	s := &Service{access: catalogAccess{}, setupFor: func(_ context.Context, _ v0.Principal, project, name string) (Setup, error) {
		if project == "" {
			return Setup{Projects: []string{"payments", "research"}}, nil
		}
		if project != "payments" && project != "research" {
			return Setup{}, errors.New("forbidden")
		}
		t := app.ReferenceTemplate()
		if project == "research" {
			t.Metadata.Name = "researcher"
		}
		return Setup{Project: project, Template: t, Templates: []string{t.Metadata.Name}}, nil
	}}
	entries, err := s.catalog(context.Background(), p)
	if err != nil || len(entries) != 2 {
		t.Fatal(entries, err)
	}
	for _, e := range entries {
		if e.CanSubmit != (e.Project == "payments") {
			t.Fatal("view confused with submit", e)
		}
	}
	s.setupFor = func(context.Context, v0.Principal, string, string) (Setup, error) {
		return Setup{}, errors.New("revoked")
	}
	if _, err = s.catalog(context.Background(), p); err == nil {
		t.Fatal("failed read returned stale catalog")
	}
}
func TestPlatformProjectionNeverCopiesSecretsAndScopesTopology(t *testing.T) {
	resolved := platform.ResolvedPlatform{Instances: []platform.ResolvedInstance{{Category: platform.CapabilityTool, Name: "mcp", Config: map[string]any{"endpoint": "https://user:password@example.test/mcp", "api-key": "do-not-copy", "provisional-token-secret": "private/key"}}}}
	info := ProjectPlatform(resolved)
	encoded, _ := json.Marshal(info)
	for _, s := range []string{"password", "do-not-copy", "private/key"} {
		if strings.Contains(string(encoded), s) {
			t.Fatal("unsafe projection")
		}
	}
	view := Setup{PlatformInfo: info, Observability: &ObservabilityInfo{Endpoint: "http://internal.example:4318"}}
	ScopePlatform(&view, false)
	if len(view.PlatformInfo.Connections) != 0 || view.Observability.Endpoint != "" {
		t.Fatal("member topology leaked")
	}
	if len(info.Connections) != 1 {
		t.Fatal("projection mutated shared configuration")
	}
	withoutTopology := Setup{Observability: &ObservabilityInfo{Endpoint: "http://internal.example:4318", DiagnosticURL: "https://internal.example/diagnostics"}}
	ScopePlatform(&withoutTopology, false)
	if withoutTopology.Observability.Endpoint != "" || withoutTopology.Observability.DiagnosticURL != "" {
		t.Fatal("missing topology bypassed observability redaction")
	}
}
