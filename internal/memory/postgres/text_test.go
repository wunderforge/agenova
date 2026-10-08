// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/memory"
)

func assertNoDatabaseCalls(t *testing.T, s *script) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connects != 0 || s.queries != 0 || len(s.begins) != 0 || s.commits != 0 || s.rollbacks != 0 {
		t.Fatalf("database called: connects=%d SQL=%d begins=%d commits=%d rollbacks=%d", s.connects, s.queries, len(s.begins), s.commits, s.rollbacks)
	}
}

func TestGovernedDenialsNeverCallDatabase(t *testing.T) {
	for _, operation := range []string{v0.MemoryRead, v0.MemoryWrite} {
		for _, test := range []struct {
			name   string
			change func(*v0.IssuedState, *memory.Request)
		}{
			{"NUL prefix", func(_ *v0.IssuedState, r *memory.Request) { r.Body, r.Query = "\x00"+r.Body, "\x00"+r.Query }},
			{"NUL middle", func(_ *v0.IssuedState, r *memory.Request) { r.Body, r.Query = "private\x00tail", "private\x00tail" }},
			{"NUL suffix", func(_ *v0.IssuedState, r *memory.Request) { r.Body, r.Query = r.Body+"\x00", r.Query+"\x00" }},
			{"blank", func(_ *v0.IssuedState, r *memory.Request) { r.Body, r.Query = "  ", "  " }},
			{"invalid UTF8", func(_ *v0.IssuedState, r *memory.Request) { r.Body, r.Query = string([]byte{255}), string([]byte{255}) }},
			{"oversized", func(_ *v0.IssuedState, r *memory.Request) {
				r.Body, r.Query = strings.Repeat("x", memory.MaxBodyBytes+1), strings.Repeat("x", memory.MaxQueryBytes+1)
			}},
			{"foreign target", func(_ *v0.IssuedState, r *memory.Request) { r.ClaimID = "claim:foreign" }},
			{"foreign scope", func(_ *v0.IssuedState, r *memory.Request) { r.Scope = "foreign" }},
			{"no operations", func(s *v0.IssuedState, _ *memory.Request) { s.EffectiveAuthority.MemoryOperations = nil }},
			{"no scopes", func(s *v0.IssuedState, _ *memory.Request) { s.EffectiveAuthority.MemoryScopes = nil }},
			{"inactive", func(s *v0.IssuedState, _ *memory.Request) { s.Claim.Phase = v0.ClaimPhaseSucceeded }},
			{"wrong owner", func(s *v0.IssuedState, _ *memory.Request) { s.Principal.Team = "team-b" }},
		} {
			t.Run(operation+"/"+test.name, func(t *testing.T) {
				data, err := os.ReadFile("../../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
				if err != nil {
					t.Fatal(err)
				}
				state, validationErr := v0.ParseSystemIssuedState(data)
				if validationErr != nil {
					t.Fatal(validationErr)
				}
				state.EffectiveAuthority.MemoryScopes = []string{namespace.Scope}
				state.EffectiveAuthority.MemoryOperations = []string{v0.MemoryRead, v0.MemoryWrite}
				b, script := backend(t)
				journal := facts.NewJournal()
				if err := journal.RegisterRequest(state.RequestRef, state.Principal); err != nil {
					t.Fatal(err)
				}
				if err := journal.BindClaim(state.RequestRef, *state.Claim); err != nil {
					t.Fatal(err)
				}
				service, err := memory.New(retryStateReader{state: state, deadline: time.Now().Add(time.Minute)}, journal, b, []memory.Namespace{namespace})
				if err != nil {
					t.Fatal(err)
				}
				session, err := service.Bind(context.Background(), state.Claim.ID, *state.Claim.BackendIdentity)
				if err != nil {
					t.Fatal(err)
				}
				request := memory.Request{ClaimID: state.Claim.ID, Operation: operation, Scope: namespace.Scope, Body: "private-text-sentinel", Query: "private-text-sentinel"}
				test.change(state, &request)
				if operation == v0.MemoryWrite {
					request.Query = ""
				} else {
					request.Body = ""
				}
				result, err := session.Invoke(context.Background(), request)
				if err != nil || result.Status != memory.Denied {
					t.Fatalf("not denied: %+v %v", result, err)
				}
				assertNoDatabaseCalls(t, script)
				got := journal.ForClaim(state.Claim.ID)
				if len(got) != 1 || got[0].Kind != "MemoryDecision" || got[0].Result != v0.DecisionResultDeny {
					t.Fatalf("denial admitted a provider attempt: %+v", got)
				}
				export, err := json.Marshal(got)
				if err != nil || strings.Contains(string(export), "private-text-sentinel") || strings.Contains(string(export), `\u0000`) {
					t.Fatal("rejected content leaked into facts")
				}
			})
		}
	}
}

func TestAdapterNULTextRejectedBeforeDatabaseAccess(t *testing.T) {
	for _, text := range []string{"\x00body", "body\x00tail", "body\x00", "\x00"} {
		t.Run(text, func(t *testing.T) {
			b, script := backend(t)
			if _, err := b.Write(context.Background(), namespace, memory.WriteInput{InvocationID: "inv:trusted", ClaimID: "claim:trusted", Body: text}); !errors.Is(err, errFailed) {
				t.Fatalf("invalid write accepted: %v", err)
			}
			if _, err := b.Search(context.Background(), namespace, memory.SearchInput{InvocationID: "inv:trusted", Query: text, Limit: 1}); !errors.Is(err, errFailed) {
				t.Fatalf("invalid query accepted: %v", err)
			}
			assertNoDatabaseCalls(t, script)
		})
	}
}
