// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIEmbedsGuideWithoutRuntimePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ACCESS.md")
	raw := []byte("---\nversion: '1'\nprofiles:\n- id: read\n  taskType: investigation\n  description: Read only\n  requestedAccess:\n    tools: [git.read]\n    resourceScopes: ['repo:acme/payments']\n---\nOnly read the evidence.\n")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	base, _ := json.Marshal(app.ReferenceTemplate())
	doc, e := packageGuide(base, path)
	if e != nil {
		t.Fatal(e)
	}
	tmp, validationErr := v0.ParseAgentTemplateYAML(doc)
	if validationErr != nil || tmp.Spec.AccessGuide == nil || tmp.Spec.AccessGuide.Markdown != "Only read the evidence." {
		t.Fatal(validationErr)
	}
	if _, e := packageGuide(doc, path); e == nil {
		t.Fatal("guide replaced implicitly")
	}
	if _, e := parseArgs([]string{"run", "--access-guide", path, "-f", "x"}); e == nil {
		t.Fatal("guide flag accepted on Work")
	}
	if _, e := parseArgs([]string{"agent-template", "apply", "--access-guide", path, "-f", "x"}); e != nil {
		t.Fatal(e)
	}
}
