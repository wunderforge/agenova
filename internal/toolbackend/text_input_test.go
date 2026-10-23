// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package toolbackend

import (
	"strings"
	"testing"
)

func TestExplicitTextModeNeverWidensLegacyAllowlist(t *testing.T) {
	text := Descriptor{Operation: "web.search", Description: "Search public text", ResourceScope: "web:public-search", Parameter: "query", MaxBytes: 64, InputMode: "text"}
	c, err := NewCatalog([]Descriptor{descriptor(), text})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"Zoom vs Teams", "澳大利亚 小企业 软件", "ordinary query / with punctuation?"} {
		if c.Validate("web.search", text.ResourceScope, map[string]string{"query": q}) != nil {
			t.Fatal("free text rejected", q)
		}
	}
	for _, q := range []string{"", "  ", "x\ny", "x\ty", strings.Repeat("x", 65), string([]byte{255})} {
		if c.Validate("web.search", text.ResourceScope, map[string]string{"query": q}) == nil {
			t.Fatal("invalid text accepted")
		}
	}
	if c.Validate("web.search", "crm:customer-retention", map[string]string{"query": "Zoom"}) == nil || c.Validate("repo.read", "repo:example/a", map[string]string{"file": "arbitrary search"}) == nil || c.Validate("web.search", text.ResourceScope, map[string]string{"query": "Zoom", "url": "http://localhost"}) == nil {
		t.Fatal("scope/legacy/extra argument widened")
	}
	for _, mode := range []string{"", "unknown"} {
		bad := text
		bad.InputMode = mode
		if _, err := NewCatalog([]Descriptor{bad}); err == nil {
			t.Fatal("unconfigured mode allowed")
		}
	}
	bad := text
	bad.AllowedValues = []string{"Zoom"}
	if _, err := NewCatalog([]Descriptor{bad}); err == nil {
		t.Fatal("mixed text and enum allowed")
	}
}
