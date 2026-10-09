// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/memory"
	"github.com/wunderforge/agenova/internal/runtime"
)

// The real lifecycle owner supplies the lock. This wrapper only reports when
// the callback is inside that boundary, to force the old and new interleavings
// without sleeps, scheduler assumptions or a production hook.
type admissionReader struct {
	*app.RunService
	observing atomic.Bool
}

func (r *admissionReader) ObserveState(id string, observe func(*v0.IssuedState)) {
	r.RunService.ObserveState(id, func(state *v0.IssuedState) {
		r.observing.Store(true)
		defer r.observing.Store(false)
		observe(state)
	})
}

type admissionSink struct {
	*facts.Journal
	kind string
	hook func()
}

func (s *admissionSink) Append(f facts.Fact) (facts.Fact, error) {
	if f.Kind == s.kind && (f.Kind != "MemoryDecision" || f.Result == v0.DecisionResultAllow) {
		s.hook()
	}
	return s.Journal.Append(f)
}

type admissionRuntime struct{ claim string }

type admissionBackend struct {
	readerBackend
	beforeCall func()
}

func (b *admissionBackend) Write(ctx context.Context, n memory.Namespace, in memory.WriteInput) (memory.Record, error) {
	if b.beforeCall != nil {
		b.beforeCall()
	}
	return b.readerBackend.Write(ctx, n, in)
}
func (b *admissionBackend) Search(ctx context.Context, n memory.Namespace, in memory.SearchInput) ([]memory.Record, error) {
	if b.beforeCall != nil {
		b.beforeCall()
	}
	return b.readerBackend.Search(ctx, n, in)
}

func (r *admissionRuntime) Allocate(in runtime.AllocateRequest) (runtime.Allocation, error) {
	r.claim = in.ClaimID
	return runtime.Allocation{ClaimID: in.ClaimID, Identity: v0.SandboxClaimBackendIdentity{Backend: "test-backend", WorkerID: "worker:demo"}}, nil
}
func (r *admissionRuntime) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{ClaimID: r.claim, Identity: id, Ready: true}, nil
}
func (*admissionRuntime) Start(v0.SandboxClaimBackendIdentity) error     { return nil }
func (*admissionRuntime) Terminate(v0.SandboxClaimBackendIdentity) error { return nil }
func (*admissionRuntime) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}

func TestMemoryAdmissionCorrelatesWithTerminalTransitions(t *testing.T) {
	var traces []evidence.View
	for _, interrupt := range []string{"cancel", "expire"} {
		for _, boundary := range []string{"MemoryDecision", "ProviderAttempt", "terminal-first", "backend-in-flight"} {
			for _, operation := range []string{v0.MemoryRead, v0.MemoryWrite} {
				name := interrupt + "/" + boundary + "/" + operation
				t.Run(name, func(t *testing.T) {
					view := memoryReaderView(t)
					view.State.Claim.Phase, view.State.Claim.BackendIdentity = v0.ClaimPhasePending, nil
					view.State.Evidence.RuntimeEvents = []v0.EvidenceRuntimeEvent{}
					journal := facts.NewJournal()
					if err := journal.RegisterRequest(view.RequestRef, view.State.Principal); err != nil {
						t.Fatal(err)
					}
					if err := journal.BindClaim(view.RequestRef, *view.State.Claim); err != nil {
						t.Fatal(err)
					}
					for _, f := range view.Facts[:3] {
						if _, err := journal.Append(f); err != nil {
							t.Fatal(err)
						}
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					expiry := make(chan time.Time, 1)
					terminal := make(chan struct{})
					terminalEvent := "Cancelled"
					if interrupt == "expire" {
						terminalEvent = app.EventExpired
					}
					owner, err := app.NewRunService(&admissionRuntime{}, app.RunServiceOptions{
						After: func(time.Duration) <-chan time.Time { return expiry },
						OnEvent: func(state *v0.IssuedState, event string) error {
							_, err := journal.Append(facts.Fact{Kind: "Runtime", RequestRef: state.RequestRef, ClaimID: state.Claim.ID, BackendIdentity: state.Claim.BackendIdentity, Operation: event, ReasonCode: "runtime-" + event, PolicyRef: &state.PolicyRef})
							if event == terminalEvent {
								close(terminal)
							}
							return err
						},
					})
					if err != nil {
						t.Fatal(err)
					}
					reader := &admissionReader{RunService: owner}
					backend := &admissionBackend{}
					var result memory.Result
					final, runErr := owner.RunContext(ctx, view.State, app.ResolvedLaunch{ProfileRef: "standard-isolated", TemplateRef: "engineer"}, func(workCtx context.Context) error {
						interruptRun := func() {
							if interrupt == "cancel" {
								cancel()
							} else {
								expiry <- time.Now()
							}
							<-workCtx.Done() // stopWork cancels before acquiring the lifecycle write lock.
							if !reader.observing.Load() {
								<-terminal // old admission permits terminal publication to win.
							}
						}
						sink := &admissionSink{Journal: journal, kind: boundary, hook: interruptRun}
						if boundary == "backend-in-flight" {
							// Terminal publication must proceed while IO is in flight.
							backend.beforeCall = interruptRun
						}
						service, err := memory.New(reader, sink, backend, []memory.Namespace{{Team: "team-a", Project: "payments", Scope: "team-docs"}})
						if err != nil {
							return err
						}
						state := owner.State(view.State.Claim.ID)
						session, err := service.Bind(workCtx, state.Claim.ID, *state.Claim.BackendIdentity)
						if err != nil {
							return err
						}
						if boundary == "terminal-first" {
							interruptRun()
							<-terminal
						}
						request := memory.Request{ClaimID: state.Claim.ID, Operation: operation, Scope: "team-docs"}
						if operation == v0.MemoryRead {
							request.Query = "private-admission-query"
						} else {
							request.Body = "private-admission-body"
						}
						result, err = session.Invoke(context.Background(), request)
						return err
					})
					if !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, app.ErrRunDeadline) {
						t.Fatalf("unexpected run result: %v", runErr)
					}
					wantCalls := 0
					if boundary == "backend-in-flight" {
						wantCalls = 1
					}
					if final == nil || backend.calls != wantCalls {
						t.Fatalf("backend calls=%d, want %d", backend.calls, wantCalls)
					}
					wantStatus := memory.Cancelled
					if boundary == "terminal-first" {
						wantStatus = memory.Denied
					} else if boundary == "backend-in-flight" && operation == v0.MemoryWrite {
						wantStatus = memory.Written // preserve a known commit after revocation.
					}
					if result.Status != wantStatus || len(result.Entries) != 0 {
						t.Fatalf("unexpected Memory result: %s", result.Status)
					}
					outcomeStatus := "Failed"
					if interrupt == "expire" {
						outcomeStatus = "Expired"
					}
					view.State, view.Outcome = final, &evidence.Outcome{Status: outcomeStatus, Failure: "Run interrupted."}
					if _, err := journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: view.RequestRef, ClaimID: final.Claim.ID, Operation: outcomeStatus, ReasonCode: "runtime-interrupted", Reason: view.Outcome.Failure}); err != nil {
						t.Fatal(err)
					}
					view.Facts = journal.ForRequest(view.RequestRef)
					view = evidence.ProjectPublic(view)
					terminalSeen := false
					for _, f := range view.Facts {
						if f.Kind == "Runtime" && f.Operation == terminalEvent {
							terminalSeen = true
						}
						if terminalSeen && (f.Kind == "MemoryDecision" && f.Result == v0.DecisionResultAllow || f.Kind == "ProviderAttempt") {
							t.Error("new Memory admission published after terminal authority")
						}
					}
					if !terminalSeen || !validEvidenceView(view, view.RequestRef) {
						t.Error("real lifecycle/Memory evidence is rejected by connected reader")
					}
					traces = append(traces, view)
				})
			}
		}
	}
	// Explicit evidence capture supplies the same public producer traces to the
	// Portal reader regression. Normal and repeated tests do not write artifacts.
	if path := os.Getenv("AGENOVA_MEMORY_ADMISSION_TRACES"); path != "" && !t.Failed() {
		data, err := evidence.MarshalPublic(traces)
		if err != nil || os.WriteFile(path, data, 0600) != nil {
			t.Fatal("could not capture public admission traces")
		}
	}
}
