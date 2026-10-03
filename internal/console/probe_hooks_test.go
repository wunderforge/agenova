// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build agenovaprobe

package console

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
)

// The probe bridge fails one tool-path append and must stop the call before
// the provider, while every other fact still reaches the real journal.
func TestProbeAppendWrapperFailsBeforeProviderCall(t *testing.T) {
	for _, kind := range []string{"ToolDecision", "ProviderAttempt"} {
		t.Run(kind, func(t *testing.T) {
			provider := &toolDouble{}
			service, err := NewServiceWithOptions(&verticalBackend{}, reactExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: boundTools(t, provider, "git.read", "repo:acme/payments")})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			var failed atomic.Int32
			if err := WrapToolAppendForProbe(service, func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) {
				return func(f facts.Fact) (facts.Fact, error) {
					if f.Kind == kind {
						failed.Add(1)
						return facts.Fact{}, errors.New("probe journal failure")
					}
					return next(f)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Submit(verticalRequest(t, "probe")); err != nil {
				t.Fatal(err)
			}
			view := awaitVertical(t, service, "probe")
			if provider.calls.Load() != 0 || failed.Load() != 1 || view.Outcome.Status != "Failed" {
				t.Fatalf("calls=%d failed=%d status=%q", provider.calls.Load(), failed.Load(), view.Outcome.Status)
			}
			for _, f := range view.Facts {
				if f.Operation == "tool.invoke" && (f.Kind == kind || f.Kind == "ProviderOutcome") {
					t.Fatalf("a failed append was recorded or the call completed: %+v", f)
				}
			}
		})
	}
}

func TestProbeAppendWrapperRefusedOnceWorkExists(t *testing.T) {
	service, err := NewServiceWithOptions(&verticalBackend{}, reactExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: boundTools(t, &toolDouble{}, "git.read", "repo:acme/payments")})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Submit(verticalRequest(t, "probe")); err != nil {
		t.Fatal(err)
	}
	awaitVertical(t, service, "probe")
	identity := func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) { return next }
	if err := WrapToolAppendForProbe(service, identity); err == nil {
		t.Fatal("wrapper installed after Work existed")
	}
}
