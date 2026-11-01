// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1_test

import (
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"strings"
	"testing"
)

func TestAccessGuidePackagingAndCeiling(t *testing.T) {
	raw := []byte("---\nversion: '1'\nprofiles:\n- id: read\n  taskType: investigation\n  description: Read repository evidence\n  requestedAccess:\n    tools: [git.read]\n    resourceScopes: ['repo:acme/payments']\n---\n# Read only\nNo writes required.\n")
	guide, e := v0.ParseAccessGuideMarkdown(raw)
	if e != nil {
		t.Fatal(e)
	}
	tmp := app.ReferenceTemplate()
	tmp.Spec.AccessGuide = guide
	if e := v0.ValidateAgentTemplate(tmp); e != nil {
		t.Fatal(e)
	}
	hash := v0.AccessGuideDigest(tmp)
	tmp.Spec.AccessGuide.Markdown += "more"
	if hash == v0.AccessGuideDigest(tmp) {
		t.Fatal("guide drift not pinned")
	}
	tmp.Spec.AccessGuide.Profiles[0].RequestedAccess.Tools = []string{"arbitrary-admin"}
	if v0.ValidateAgentTemplate(tmp) == nil {
		t.Fatal("guide bypassed ceiling")
	}
	for _, data := range [][]byte{[]byte("not frontmatter"), []byte("---\nversion: '1'\nstatus: granted\n---\n"), []byte(strings.Repeat("x", (32<<10)+1))} {
		if _, e := v0.ParseAccessGuideMarkdown(data); e == nil {
			t.Fatal("bad guide accepted")
		}
	}
}
func TestAdvisorConfigurationBounds(t *testing.T) {
	for _, cfg := range []*v0.AccessAdvisorConfig{{Enabled: true, Engine: "unknown"}, {Enabled: true, Engine: "model"}, {Enabled: true, MaxCalls: -1}, {Enabled: true, MaxConcurrent: 5}, {Enabled: true, Timeout: "2m"}, {Enabled: true, AllowedModes: []string{"grant-admin"}}} {
		if _, e := v0.NormalizeAccessAdvisor(cfg); e == nil {
			t.Fatal("bad config accepted", cfg)
		}
	}
	cfg, e := v0.NormalizeAccessAdvisor(nil)
	if e != nil || cfg.Enabled || cfg.DefaultMode != "manual" {
		t.Fatal(cfg, e)
	}
}
