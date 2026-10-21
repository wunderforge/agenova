// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/runtime"
)

// Exercise actual production kubectlRunner argument/context propagation. The
// substituted command waits on ctx.Done(), not a post-response fake hook.
func TestColdAllocationRemoteCommandsUseOneAbsoluteDeadline(t *testing.T) {
	for _, boundary := range []string{"bind-read", "postwait-claim-read", "pod-pin-read", "late-success-read", "create-uncommitted", "create-committed", "pod-pin-retained-process"} {
		for _, limit := range []string{"bind-budget", "work-expiry"} {
			t.Run(boundary+"/"+limit, func(t *testing.T) {
				k := &networkKube{fakeKube: newFakeKube(), retainPod: boundary == "pod-pin-retained-process"}
				control := &fakeWorkerControl{}
				a := newControlledAdapter(newTestAdapter(k.fakeKube), control)
				a.bindTimeout = time.Minute
				expiry := time.Now().Add(time.Minute)
				if limit == "bind-budget" {
					a.bindTimeout = 200 * time.Millisecond
				} else {
					expiry = time.Now().Add(200 * time.Millisecond)
				}
				if err := a.ConfigureNetworkIsolation("strict"); err != nil {
					t.Fatal(err)
				}
				r := newKubectlRunner("binding-test", "agenova-test")
				blocked, cleanupIndependent := false, false
				claimReads, deletes := 0, 0
				var originalDeadline time.Time
				r.command = func(ctx context.Context, input []byte, args ...string) ([]byte, error) {
					if len(args) < 5 || args[0] != "--context" || args[1] != "binding-test" || args[2] != "--namespace" || args[3] != "agenova-test" {
						return nil, errors.New("unexpected binding command boundary")
					}
					argv := args[4:]
					verb := argv[0]
					if blocked {
						deadline, ok := ctx.Deadline()
						if ctx.Err() != nil || !ok || !deadline.After(originalDeadline) {
							return nil, errors.New("compensation inherited expired binding context")
						}
						cleanupIndependent = true
					} else {
						deadline, ok := ctx.Deadline()
						if !ok || deadline.After(expiry) {
							return nil, errors.New("binding command lost Work deadline")
						}
						if originalDeadline.IsZero() {
							originalDeadline = deadline
						} else if !deadline.Equal(originalDeadline) {
							return nil, errors.New("binding phase reset absolute deadline")
						}
					}
					resource, name := "", ""
					if verb == "get" && len(argv) >= 3 {
						resource, name = argv[1], argv[2]
					}
					nameOnly := argv[len(argv)-1] == "name"
					if !blocked && resource == "sandboxclaims" && !nameOnly {
						claimReads++
					}
					if verb == "create" && boundary != "create-uncommitted" {
						if err := k.createBytes(input); err != nil {
							return nil, err
						}
					}
					match := !blocked && ((strings.HasPrefix(boundary, "create-") && verb == "create") ||
						((boundary == "bind-read" || boundary == "late-success-read") && claimReads == 1 && resource == "sandboxclaims" && !nameOnly) ||
						(boundary == "postwait-claim-read" && claimReads == 2 && resource == "sandboxclaims" && !nameOnly) ||
						(strings.HasPrefix(boundary, "pod-pin") && resource == "pods" && !nameOnly))
					if match {
						blocked = true
						<-ctx.Done() // Only the real propagated deadline can release.
						if boundary != "late-success-read" {
							return nil, ctx.Err()
						}
						// Even a transport that returns a late successful body must
						// not publish an expired allocation.
					}
					switch verb {
					case "create":
						return []byte(`{}`), nil
					case "get":
						if nameOnly {
							present, err := k.exists(resource, name)
							if err != nil || !present {
								return nil, err
							}
							return []byte(resource + "/" + name), nil
						}
						var obj any
						if err := k.get(resource, name, &obj); err != nil {
							return nil, err
						}
						return json.Marshal(obj)
					case "delete":
						if len(argv) != 5 || argv[1] != "--raw" {
							return nil, errors.New("unguarded compensation delete")
						}
						segments := strings.Split(argv[2], "/")
						var opts struct {
							Preconditions struct{ UID, ResourceVersion string }
						}
						if json.Unmarshal(input, &opts) != nil {
							return nil, errors.New("missing original delete preconditions")
						}
						deletes++
						return nil, k.deletePinned(segments[len(segments)-2], segments[len(segments)-1], opts.Preconditions.UID, opts.Preconditions.ResourceVersion)
					default:
						return nil, errors.New("unexpected side effect during allocation")
					}
				}
				a.kube = r
				type result struct {
					allocation runtime.Allocation
					err        error
				}
				done := make(chan result, 1)
				go func() {
					allocation, err := a.Allocate(runtime.AllocateRequest{ClaimID: "cold-deadline", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: expiry}})
					done <- result{allocation, err}
				}()
				select {
				case got := <-done:
					if got.err == nil || got.allocation.Identity.WorkerID != "" || !blocked || !cleanupIndependent || control.starts != 0 || k.networkCreates != 0 {
						t.Fatal("expired transport published allocation or skipped independent reconciliation", got, blocked, cleanupIndependent)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("actual binding command ignored earliest deadline")
				}
				entry := a.allocations["cold-deadline"]
				pending := boundary == "create-uncommitted" || boundary == "pod-pin-retained-process"
				if pending && (entry == nil || entry.state != allocRecoveryPending) || !pending && entry != nil {
					t.Fatal("ambiguous cleanup was discarded or confirmed cleanup retained", entry)
				}
				if boundary == "create-uncommitted" && deletes != 0 {
					t.Fatal("unknown uncommitted attempt deleted resources")
				}
				if boundary == "pod-pin-retained-process" && !k.podPresent {
					t.Fatal("test failed to retain an unconfirmed original process")
				}
			})
		}
	}
}

func TestColdAllocationExpiredContextMakesNoRemoteCall(t *testing.T) {
	r := newKubectlRunner("binding-test", "agenova-test")
	called := false
	r.command = func(context.Context, []byte, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	a := newSpikeAdapter(r, "agenova-test")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	var obj any
	if err := a.allocationRead(ctx, "pods", "worker", &obj); !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatal("expired read reached transport", err)
	}
	if err := r.createBytesContext(ctx, []byte(`{}`)); !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatal("expired create reached transport", err)
	}
}
