// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestNULTextDeniedBeforeBackendDispatch(t *testing.T) {
	for _, operation := range []string{v0.MemoryRead, v0.MemoryWrite} {
		for _, encoded := range []string{`"\u0000private-nul-sentinel"`, `"private-nul-sentinel\u0000tail"`, `"private-nul-sentinel\u0000"`, `"\u0000"`} {
			t.Run(operation+"/"+encoded, func(t *testing.T) {
				var text string
				if err := json.Unmarshal([]byte(encoded), &text); err != nil || !strings.ContainsRune(text, 0) {
					t.Fatalf("invalid NUL test input: %v", err)
				}
				s, _, b, journal := setup(t)
				request := searchRequest(s)
				request.Operation = operation
				if operation == v0.MemoryWrite {
					request.Body, request.Query = text, ""
				} else {
					request.Query = text
				}
				result, err := s.Invoke(context.Background(), request)
				if err != nil || result.Status != Denied || result.ReasonCode != "memory-invalid-input" || b.calls != 0 {
					t.Fatalf("result=%+v err=%v calls=%d", result, err, b.calls)
				}
				got := journal.ForClaim(s.state.Claim.ID)
				if len(got) != 1 || got[0].Kind != "MemoryDecision" || got[0].Result != v0.DecisionResultDeny || got[0].InvocationID != result.InvocationID {
					t.Fatalf("expected only a correlated denial: %+v", got)
				}
				export, err := json.Marshal(struct {
					Result Result
					Facts  any
				}{result, got})
				if err != nil || strings.Contains(string(export), "private-nul-sentinel") || strings.Contains(string(export), `\u0000`) {
					t.Fatal("rejected content leaked")
				}
			})
		}
	}
}

func TestTextContractPreservesNonNULData(t *testing.T) {
	for _, text := range []string{"line\n\tsecond", "\u77e5\u8bc6 \U0001f4da", `% _ SQL-like ' \u0000`} {
		t.Run(text, func(t *testing.T) {
			s, _, b, _ := setup(t)
			request := searchRequest(s)
			request.Operation, request.Query, request.Body = v0.MemoryWrite, "", text
			result, err := s.Invoke(context.Background(), request)
			if err != nil || result.Status != Written {
				t.Fatalf("valid body rejected: %+v %v", result, err)
			}
			b.search = func(_ context.Context, n Namespace, in SearchInput) ([]Record, error) {
				if in.Query != text {
					t.Fatal("query text changed")
				}
				return []Record{record(n, 1, text, "previous")}, nil
			}
			request = searchRequest(s)
			request.Query = text
			result, err = s.Invoke(context.Background(), request)
			if err != nil || result.Status != Found || len(result.Entries) != 1 || result.Entries[0].Body != text || b.calls != 2 {
				t.Fatalf("valid query/reply rejected: %+v %v calls=%d", result, err, b.calls)
			}
		})
	}
}

func TestNULBackendReplyWithheld(t *testing.T) {
	s, _, b, journal := setup(t)
	b.search = func(_ context.Context, n Namespace, _ SearchInput) ([]Record, error) {
		return []Record{record(n, 1, "private-reply-sentinel\x00tail", "previous")}, nil
	}
	result, err := s.Invoke(context.Background(), searchRequest(s))
	if err != nil || result.Status != Failed || result.ReasonCode != "memory-failed" || len(result.Entries) != 0 || b.calls != 1 {
		t.Fatalf("invalid reply accepted: %+v %v", result, err)
	}
	export, err := json.Marshal(journal.ForClaim(s.state.Claim.ID))
	if err != nil || strings.Contains(string(export), "private-reply-sentinel") {
		t.Fatal("invalid reply leaked into facts")
	}
}
