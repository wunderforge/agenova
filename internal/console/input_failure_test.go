// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
)

type missingSourceExecutor struct{}

func (missingSourceExecutor) Execute(context.Context, v0.SandboxClaimBackendIdentity, workerprotocol.Task, workerprotocol.Handler) (string, error) {
	return "", workerprotocol.ErrSourceURLRequired
}

func TestPreCallInputFailureHasSpecificEvidenceWithoutInventedInvocation(t *testing.T) {
	s, err := NewService(&verticalBackend{}, missingSourceExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "missing-source")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "missing-source")
	if v.Outcome.Status != "Failed" || v.Outcome.Failure != "This Agent requires a public HTTPS source URL in its task input." {
		t.Fatal(v.Outcome)
	}
	if v.Diagnosis == nil || v.Diagnosis.Code != "agent-source-url-required" || v.Diagnosis.Stage != "execution" {
		t.Fatal(v.Diagnosis)
	}
	for _, f := range v.Facts {
		if f.Kind == "ProviderAttempt" || f.Kind == "ToolDecision" || f.Kind == "ModelDecision" {
			t.Fatal("input failure invented an invocation", f)
		}
	}
}
