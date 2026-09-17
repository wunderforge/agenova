// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/registration"
)

func TestRegistrationCommands(t *testing.T) {
	store := registration.NewMemoryStore()
	services := Services{NewRegistration: func(string) (registration.Service, error) { return registration.Service{Store: store}, nil }}
	policyPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(policyPath, []byte(`apiVersion: agenova.io/v1alpha1
kind: PolicyBundle
metadata: {name: reference, version: "1"}
spec:
  rules:
    - {team: team-a, action: claim.create, project: payments, templateRef: engineer}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"agenova", "policy", "apply", "-f", policyPath}, {"agenova", "policy", "apply", "-f", policyPath}} {
		var out, errs bytes.Buffer
		if code := MainWithServices(command, &out, &errs, services); code != 0 {
			t.Fatalf("%v = %d: %s", command, code, errs.String())
		}
		if !strings.Contains(out.String(), "PolicyBundle reference@1 (active)") {
			t.Fatalf("unexpected output: %s", out.String())
		}
	}
	var out, errs bytes.Buffer
	if code := MainWithServices([]string{"agenova", "agent-template", "apply", "-f", policyPath}, &out, &errs, services); code == 0 || errs.Len() == 0 {
		t.Fatalf("wrong-kind document accepted: %d %s", code, errs.String())
	}
}
