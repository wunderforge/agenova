// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
)

type directWebExecutor struct{}

func (directWebExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	reply, err := h(ctx, workerprotocol.Operation{Kind: "tool", ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Tool: "web.read", ResourceScope: "web:public-pages", Input: "https://example.com/"})
	if err != nil {
		return "", err
	}
	if reply.Allowed {
		return "", errors.New("unexpected web grant")
	}
	return "", errors.New("expected missing web permission")
}
func TestMissingWebPermissionIsDeniedBeforeProvider(t *testing.T) {
	provider := &toolDouble{}
	tools, err := toolbackend.NewSet([]toolbackend.Binding{{Descriptor: toolbackend.Descriptor{Operation: "web.read", Description: "Read a public HTTPS page", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}, Provider: provider, Backend: "local-mcp", MaxObservationBytes: 8192, MaxConcurrentCalls: 1}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServiceWithOptions(&verticalBackend{}, directWebExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: tools})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "missing-web")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "missing-web")
	denied := false
	for _, f := range v.Facts {
		if f.Kind == "ToolDecision" && f.Target == "web.read" && f.Result == v0.DecisionResultDeny {
			denied = true
		}
		if f.Kind == "ProviderAttempt" && f.Operation == "tool.invoke" {
			t.Fatal("denied call reached provider")
		}
	}
	if !denied || provider.calls.Load() != 0 {
		t.Fatal("missing real Gateway denial or provider invoked", denied, provider.calls.Load())
	}
}
