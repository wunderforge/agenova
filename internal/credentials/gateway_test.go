// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/credentials"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/toolgateway"
)

type modelCredentialAdapter struct {
	binding  *credentials.Binding
	external int
}

func (a *modelCredentialAdapter) Invoke(string, modelgateway.Request) error {
	return a.binding.Use(context.Background(), func(context.Context, []byte) error { a.external++; return nil })
}

type toolCredentialAdapter struct {
	binding  *credentials.Binding
	external int
}

func (a *toolCredentialAdapter) Invoke(string, toolgateway.Request) error {
	return a.binding.Use(context.Background(), func(context.Context, []byte) error { a.external++; return nil })
}

func TestModelAdmissionPrecedesCredentialAndExternalCalls(t *testing.T) {
	for _, test := range []struct {
		name         string
		phase        v0.ClaimPhase
		modify       func(*modelgateway.Request)
		failEvidence bool
		unavailable  bool
		allow        bool
	}{
		{name: "positive control", phase: v0.ClaimPhaseRunning, allow: true},
		{name: "Pending", phase: v0.ClaimPhasePending}, {name: "Bound", phase: v0.ClaimPhaseBound},
		{name: "Succeeded", phase: v0.ClaimPhaseSucceeded}, {name: "Failed", phase: v0.ClaimPhaseFailed}, {name: "Expired", phase: v0.ClaimPhaseExpired},
		{name: "unknown claim", phase: v0.ClaimPhaseRunning, modify: func(r *modelgateway.Request) { r.ClaimID = "unknown" }},
		{name: "ungranted profile", phase: v0.ClaimPhaseRunning, modify: func(r *modelgateway.Request) { r.Profile = "foreign" }},
		{name: "worker credential", phase: v0.ClaimPhaseRunning, modify: func(r *modelgateway.Request) { r.Parameters = map[string]string{"apiKey": "synthetic-worker-value"} }},
		{name: "evidence failure", phase: v0.ClaimPhaseRunning, failEvidence: true},
		{name: "missing admitted credential", phase: v0.ClaimPhaseRunning, unavailable: true, allow: true},
		{name: "worker cannot retarget host binding", phase: v0.ClaimPhaseRunning, modify: func(r *modelgateway.Request) { r.Parameters = map[string]string{"credentialRef": "unrelated"} }, allow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			const sentinel = "synthetic-host-material-never-in-facts"
			m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte(sentinel)}}
			if test.unavailable {
				m.err = errors.New("synthetic-private-provider-error")
			}
			_, b := binding(t, m)
			adapter := &modelCredentialAdapter{binding: b}
			claims := gatewaytest.NewClaims()
			claims.Put("claim:worker", test.phase)
			store := facts.NewStore()
			options := []modelgateway.Option{modelgateway.WithAdapter(adapter)}
			if test.failEvidence {
				options = append(options, modelgateway.WithObserver(func(modelgateway.Request, gateway.Decision) error { return errors.New("recording failed") }))
			}
			gw := modelgateway.NewGateway(claims, nil, store, options...)
			req := modelgateway.Request{ClaimID: "claim:worker", Profile: "approved-coding-model"}
			if test.modify != nil {
				test.modify(&req)
			}
			decision, err := gw.Invoke(req)
			if test.allow {
				if decision.Result != gateway.ResultAllow || m.calls != 1 || m.seen[0] != testRef {
					t.Fatal("admitted host binding changed")
				}
				if test.unavailable {
					if err == nil || adapter.external != 0 {
						t.Fatal("missing credential reached provider")
					}
				} else if err != nil || adapter.external != 1 {
					t.Fatal("allowed provider control failed")
				}
			} else if m.calls != 0 || adapter.external != 0 || !test.failEvidence && decision.Result == gateway.ResultAllow {
				t.Fatal("denial or evidence failure reached credential/provider")
			}
			encoded, marshalErr := json.Marshal(store.ModelInvocations(req.ClaimID))
			if marshalErr != nil || strings.Contains(string(encoded), sentinel) || err != nil && strings.Contains(err.Error(), "synthetic-private") {
				t.Fatal("credential escaped host boundary")
			}
		})
	}
}

func TestToolAdmissionPrecedesCredentialAndExternalCalls(t *testing.T) {
	for _, test := range []struct {
		name        string
		phase       v0.ClaimPhase
		tool, scope string
		allow       bool
	}{
		{"positive control", v0.ClaimPhaseRunning, "git", "repo:acme/payments", true},
		{"terminal", v0.ClaimPhaseFailed, "git", "repo:acme/payments", false},
		{"ungranted tool", v0.ClaimPhaseRunning, "foreign", "repo:acme/payments", false},
		{"ungranted scope", v0.ClaimPhaseRunning, "git", "repo:unrelated", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte("synthetic-tool-material")}}
			_, b := binding(t, m)
			adapter := &toolCredentialAdapter{binding: b}
			claims := gatewaytest.NewClaims()
			claims.Put("claim:worker", test.phase)
			store := facts.NewStore()
			gw := toolgateway.NewGateway(claims, nil, store, toolgateway.WithAdapter(adapter))
			decision, err := gw.Invoke(toolgateway.Request{ClaimID: "claim:worker", Tool: test.tool, Action: "read", ResourceScope: test.scope})
			if err != nil || (decision.Result == gateway.ResultAllow) != test.allow {
				t.Fatal("tool control decision mismatch")
			}
			want := 0
			if test.allow {
				want = 1
			}
			if m.calls != want || adapter.external != want {
				t.Fatal("tool denial reached credential/provider")
			}
			encoded, err := json.Marshal(store.ToolInvocations("claim:worker"))
			if err != nil || strings.Contains(string(encoded), "synthetic-tool-material") {
				t.Fatal("credential entered tool facts")
			}
		})
	}
}
