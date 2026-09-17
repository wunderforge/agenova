// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFileUsesInstalledServiceAndCanonicalDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work.yaml")
	data := []byte(`apiVersion: agenova.io/v1alpha1
kind: ClaimRequest
metadata: {name: demo}
spec:
  templateRef: engineer
  projectRef: payments
  task: {type: investigation, input: {objective: Investigate a synthetic issue}}
  requestedAccess: {modelProfile: coding-standard}
  runtime: {profileRef: standard-isolated, timeout: 1m}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", Invoke: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		calls++
		if !strings.Contains(strings.Join(args, " "), "exec deployment/agenova-control-plane") {
			t.Fatalf("not installed service: %v", args)
		}
		if calls == 1 {
			var document map[string]any
			if err := json.Unmarshal(input, &document); err != nil {
				t.Fatalf("not JSON: %v", err)
			}
			runtime := document["spec"].(map[string]any)["runtime"].(map[string]any)
			if runtime["timeout"] != "1m" {
				t.Fatalf("duration changed: %#v", runtime)
			}
			return []byte(`{"version":"agenova.evidence/v0","requestRef":"demo","facts":[],"outcome":{"status":"Deny"}}`), nil
		}
		t.Fatal("denied submission was polled")
		return nil, nil
	}}
	view, err := client.RunFile(path)
	if err != nil || view.Outcome == nil || view.Outcome.Status != "Deny" || calls != 1 {
		t.Fatalf("RunFile = %#v, %v, calls %d", view, err, calls)
	}
}
