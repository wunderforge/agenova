// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/memory"
)

func memoryReaderView(t *testing.T) evidence.View {
	t.Helper()
	var vectors struct{ View evidence.View }
	data, err := os.ReadFile("../../work/0179-scoped-memory/memory-reader-vectors.json")
	if err != nil || json.Unmarshal(data, &vectors) != nil {
		t.Fatal("cannot load shared Memory vectors")
	}
	return vectors.View
}

type memoryReaderState struct{ state *v0.IssuedState }

func (r memoryReaderState) State(string) *v0.IssuedState { return r.state }
func (r memoryReaderState) ObserveState(_ string, observe func(*v0.IssuedState)) {
	// This public-reader fixture has no concurrent lifecycle mutations.
	observe(r.state)
}
func (r memoryReaderState) ClaimDeadline(string) (time.Time, bool) {
	return time.Now().Add(time.Minute), true
}

type readerBackend struct{ calls int }

func (b *readerBackend) Write(_ context.Context, n memory.Namespace, in memory.WriteInput) (memory.Record, error) {
	b.calls++
	return memory.Record{Namespace: n, Reference: "memory:00000000000000000000000000000001", Body: in.Body, SourceClaim: in.ClaimID, CreatedAt: time.Now()}, nil
}
func (b *readerBackend) Search(_ context.Context, n memory.Namespace, _ memory.SearchInput) ([]memory.Record, error) {
	b.calls++
	return []memory.Record{{Namespace: n, Reference: "memory:00000000000000000000000000000001", Body: "private-memory-observation", SourceClaim: "claim:previous", CreatedAt: time.Now()}}, nil
}

func TestMemoryServicePublicHTTPAndConnectedReader(t *testing.T) {
	view := memoryReaderView(t)
	journal := facts.NewJournal()
	if err := journal.RegisterRequest(view.RequestRef, view.State.Principal); err != nil {
		t.Fatal(err)
	}
	if err := journal.BindClaim(view.RequestRef, *view.State.Claim); err != nil {
		t.Fatal(err)
	}
	for _, f := range view.Facts[:6] {
		if _, err := journal.Append(f); err != nil {
			t.Fatal(err)
		}
	}
	b := &readerBackend{}
	service, err := memory.New(memoryReaderState{view.State}, journal, b, []memory.Namespace{{Team: "team-a", Project: "payments", Scope: "team-docs"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Bind(context.Background(), view.State.Claim.ID, *view.State.Claim.BackendIdentity)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []memory.Request{
		{ClaimID: view.State.Claim.ID, Scope: "team-docs", Operation: "write", Body: "private-memory-write"},
		{ClaimID: view.State.Claim.ID, Scope: "team-docs", Operation: "read", Query: "private-memory-query"},
		{ClaimID: "claim:victim", Scope: "team-docs", Operation: "read", Query: "private-memory-query"},
	} {
		if _, err := session.Invoke(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if b.calls != 2 {
		t.Fatal("denial reached backend")
	}
	view.Facts = journal.ForRequest(view.RequestRef)
	view.Request.Spec.Task.Input = map[string]any{"objective": "private-task-input"}
	encoded, err := evidence.MarshalPublic(view)
	if err != nil || strings.Contains(string(encoded), "private") {
		t.Fatal("producer public export leaked content")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	client := Client{Context: "test-reader-context", Namespace: "test-reader", OpenTunnel: func(context.Context) (string, func(), error) { return server.URL, func() {}, nil }}
	got, err := client.Show(view.RequestRef)
	if err != nil || len(got.Facts) != 13 || got.Facts[8].Memory.Status != "Written" || got.Facts[11].Memory.Status != "Found" {
		t.Fatalf("producer/reader mismatch: %v", err)
	}
}

func TestMemoryReaderOutcomesAndLifecycle(t *testing.T) {
	for _, status := range []string{"Written", "Found", "Empty", "Unsupported", "Unavailable", "Timeout", "Cancelled", "Failed", "WriteUncertain"} {
		t.Run(status, func(t *testing.T) {
			view := memoryReaderView(t)
			f := &view.Facts[8]
			f.Memory = &facts.MemoryMetadata{Status: status, DurationMilliseconds: 2}
			f.ProviderStatus, f.ReasonCode = "Failed", "memory-"+strings.ToLower(status)
			switch status {
			case "Written", "Found", "Empty":
				f.ProviderStatus, f.ReasonCode = "Succeeded", "memory-completed"
				if status != "Empty" {
					f.Memory.Count = 1
					f.Memory.References = []string{"memory:00000000000000000000000000000001"}
				}
				if status != "Written" {
					for i := 6; i < 9; i++ {
						view.Facts[i].Operation = "memory.read"
					}
				}
			case "Cancelled":
				f.ProviderStatus = "Cancelled"
			case "WriteUncertain":
				f.ReasonCode = "memory-write-uncertain"
			}
			if !validEvidenceView(view, view.RequestRef) {
				t.Fatal("valid result rejected")
			}
		})
	}
	for _, status := range []string{"Written", "WriteUncertain", "Cancelled", "Timeout", "Found"} {
		t.Run("after cancellation/"+status, func(t *testing.T) {
			view := memoryReaderView(t)
			outcome := view.Facts[8]
			view.Facts = view.Facts[:8]
			view.State.Claim.Phase = v0.ClaimPhaseFailed
			view.State.Evidence.RuntimeEvents = append(view.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: "Cancelled"})
			view.Facts = append(view.Facts, facts.Fact{ID: "fact:cancelled", Sequence: 9, Timestamp: outcome.Timestamp, Kind: "Runtime", RequestRef: view.RequestRef, ClaimID: view.State.Claim.ID, Operation: "Cancelled"})
			outcome.Sequence = 10
			if status != "Written" {
				outcome.Memory = &facts.MemoryMetadata{Status: status}
				outcome.ProviderStatus, outcome.ReasonCode = "Failed", "memory-"+strings.ToLower(status)
			}
			if status == "WriteUncertain" {
				outcome.ReasonCode = "memory-write-uncertain"
			}
			if status == "Cancelled" {
				outcome.ProviderStatus = "Cancelled"
			}
			if status == "Cancelled" || status == "Timeout" || status == "Found" {
				view.Facts[6].Operation, view.Facts[7].Operation, outcome.Operation = "memory.read", "memory.read", "memory.read"
			}
			if status == "Found" {
				outcome.ProviderStatus, outcome.ReasonCode = "Succeeded", "memory-completed"
				outcome.Memory.Count = 1
				outcome.Memory.References = []string{"memory:00000000000000000000000000000001"}
			}
			view.Facts = append(view.Facts, outcome)
			if validEvidenceView(view, view.RequestRef) != (status != "Found") {
				t.Fatal("late result violated commit truth or read withholding")
			}
			if status == "Found" {
				return
			}
			deny := view.Facts[6]
			deny.ID, deny.InvocationID, deny.Sequence, deny.Result, deny.ReasonCode = "fact:late-deny", "inv:late-deny", 11, v0.DecisionResultDeny, "memory-claim-inactive"
			view.Facts = append(view.Facts, deny)
			if !validEvidenceView(view, view.RequestRef) {
				t.Fatal("post-terminal denial rejected")
			}
			view.Facts[len(view.Facts)-1].Result, view.Facts[len(view.Facts)-1].ReasonCode = v0.DecisionResultAllow, "memory-allowed"
			if validEvidenceView(view, view.RequestRef) {
				t.Fatal("new post-terminal Allow accepted")
			}
		})
	}
}

func TestMemoryReaderSharedVectors(t *testing.T) {
	data, err := os.ReadFile("../../work/0179-scoped-memory/memory-reader-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		View        json.RawMessage
		Corruptions []struct {
			Name  string
			Path  []string
			Value any
		}
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeView(vectors.View, "memory-reader"); err != nil {
		t.Fatalf("valid Memory view rejected: %v", err)
	}
	for _, test := range vectors.Corruptions {
		t.Run(test.Name, func(t *testing.T) {
			var root map[string]any
			if err := json.Unmarshal(vectors.View, &root); err != nil {
				t.Fatal(err)
			}
			var current any = root
			for _, part := range test.Path[:len(test.Path)-1] {
				switch value := current.(type) {
				case map[string]any:
					current = value[part]
				case []any:
					index, err := strconv.Atoi(part)
					if err != nil {
						t.Fatal(err)
					}
					current = value[index]
				}
			}
			current.(map[string]any)[test.Path[len(test.Path)-1]] = test.Value
			encoded, _ := json.Marshal(root)
			if _, err := decodeView(encoded, "memory-reader"); err == nil {
				t.Fatal("corrupt Memory view accepted")
			}
		})
	}
}

func TestMemoryReaderDenialRequiresPriorRunning(t *testing.T) {
	data, err := os.ReadFile("../../work/0179-scoped-memory/memory-reader-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		DenialLifecycles []struct {
			Name   string
			Phase  v0.ClaimPhase
			Events []string
			Accept bool
		}
	}
	if err := json.Unmarshal(data, &vectors); err != nil || len(vectors.DenialLifecycles) == 0 {
		t.Fatal("cannot load shared Memory denial lifecycles")
	}
	for _, test := range vectors.DenialLifecycles {
		t.Run(test.Name, func(t *testing.T) {
			view := memoryReaderView(t)
			deny := view.Facts[12]
			view.State.Claim.Phase = test.Phase
			view.State.Evidence.RuntimeEvents = []v0.EvidenceRuntimeEvent{}
			view.Facts = view.Facts[:3]
			for _, event := range test.Events {
				f := facts.Fact{Kind: "Runtime", RequestRef: view.RequestRef, ClaimID: view.State.Claim.ID, Operation: event}
				if event == "Deny" {
					f = deny
				} else {
					view.State.Evidence.RuntimeEvents = append(view.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: event})
					if event == "Bound" {
						f.BackendIdentity = view.State.Claim.BackendIdentity
					}
				}
				f.ID, f.Sequence, f.Timestamp = "fact:lifecycle:"+strconv.Itoa(len(view.Facts)+1), uint64(len(view.Facts)+1), view.Facts[0].Timestamp
				view.Facts = append(view.Facts, f)
			}
			encoded, err := evidence.MarshalPublic(view)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeView(encoded, view.RequestRef); (err == nil) != test.Accept {
				t.Fatalf("denial acceptance = %v, want %v: %v", err == nil, test.Accept, err)
			}
		})
	}
}

func TestMemoryReaderTerminalClosureAndCrossWorkIdentity(t *testing.T) {
	view := memoryReaderView(t)
	view.Facts = view.Facts[:8]
	if !validEvidenceView(view, view.RequestRef) {
		t.Fatal("in-progress Memory call rejected")
	}
	view.State.Claim.Phase = v0.ClaimPhaseSucceeded
	for _, operation := range []string{"Succeeded", "TerminateSucceeded", "CleanupSucceeded"} {
		view.State.Evidence.RuntimeEvents = append(view.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: operation})
		view.Facts = append(view.Facts, facts.Fact{ID: "fact:" + operation, Sequence: uint64(len(view.Facts) + 1), Timestamp: view.Facts[0].Timestamp, Kind: "Runtime", RequestRef: view.RequestRef, ClaimID: view.State.Claim.ID, Operation: operation})
	}
	view.Facts = append(view.Facts, facts.Fact{ID: "fact:run-outcome", Sequence: uint64(len(view.Facts) + 1), Timestamp: view.Facts[0].Timestamp, Kind: "RunOutcome", RequestRef: view.RequestRef, ClaimID: view.State.Claim.ID, Operation: "Succeeded"})
	view.Outcome = &evidence.Outcome{Status: "Succeeded"}
	if validEvidenceView(view, view.RequestRef) {
		t.Fatal("terminal Work accepted an unfinished Memory invocation")
	}
	base, other := memoryReaderView(t), memoryReaderView(t)
	other.RequestRef, other.Request.Metadata.Name, other.State.RequestRef, other.State.Evidence.RequestRef, other.State.Claim.RequestRef = "other", "other", "other", "other", "other"
	other.State.Claim.ID, other.State.Claim.AuthorityRef, other.State.EffectiveAuthority.ID, other.State.Claim.BackendIdentity.WorkerID = "claim:other", "authority:other", "authority:other", "worker:other"
	for i := range other.Facts {
		f := &other.Facts[i]
		f.RequestRef, f.ID, f.Sequence = "other", "other:"+f.ID, f.Sequence+100
		if f.ClaimID != "" {
			f.ClaimID = "claim:other"
		}
		if f.Authority != nil {
			f.Authority.ID = "authority:other"
		}
		if f.BackendIdentity != nil {
			f.BackendIdentity.WorkerID = "worker:other"
		}
	}
	if !validEvidenceView(other, other.RequestRef) {
		t.Fatal("independent public Work rejected before list correlation")
	}
	encoded, err := evidence.MarshalPublic([]evidence.View{base, other})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(encoded) }))
	defer server.Close()
	client := Client{Context: "test-reader-context", Namespace: "test-reader", OpenTunnel: func(context.Context) (string, func(), error) { return server.URL, func() {}, nil }}
	if _, err := client.List(); err == nil {
		t.Fatal("cross-Work Memory invocation reuse accepted")
	}
}
