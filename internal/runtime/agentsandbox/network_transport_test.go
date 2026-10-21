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

// These tests run the production kubectlRunner transport/argv/context plumbing.
// Only the command process is simulated. A blocked read waits on its ACTUAL
// command context, never on a post-response hook or a manually released gate.
// The fake backend remains configuration/lifecycle evidence, not packet proof.
func TestNetworkActivationRemoteCommandsActuallyAbortOnCallerCancellation(t *testing.T) {
	for _, boundary := range []string{"claim-read", "pod-read", "pod-patch", "rule-exists", "rule-create-after-commit", "policy-readback-after-create", "final-claim-after-create", "final-pod-after-create", "policy-readback-after-create-expiry"} {
		t.Run(boundary, func(t *testing.T) {
			a, k, f, alloc := networkFixture(t, []string{"public-web"})
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			earlierExpiry := strings.HasSuffix(boundary, "-expiry")
			if earlierExpiry {
				k.claims[alloc.Identity.WorkerID].Spec.Lifecycle.ShutdownTime = time.Now().Add(200 * time.Millisecond).Format(time.RFC3339Nano)
			}
			r := newKubectlRunner("test", "agenova-test")
			type milestone struct {
				created, rulePresent bool
			}
			entered := make(chan milestone, 1)
			created, blocked, cleaned := false, false, false
			r.command = func(ctx context.Context, input []byte, args ...string) ([]byte, error) {
				// Strip the production explicit context/namespace prefix.
				if len(args) < 5 || args[0] != "--context" || args[1] != "test" || args[2] != "--namespace" || args[3] != "agenova-test" {
					t.Fatalf("unexpected command boundary %v", args)
				}
				argv := args[4:]
				verb := argv[0]
				if verb == "create" {
					if err := k.createBytes(input); err != nil {
						return nil, err
					}
					created = true
				}
				isGet := verb == "get" && len(argv) >= 3
				resource, name := "", ""
				if isGet {
					resource, name = argv[1], argv[2]
				}
				nameOnly := len(argv) > 1 && argv[len(argv)-1] == "name"
				match := (boundary == "claim-read" && isGet && resource == "sandboxclaims" && !created && !nameOnly) ||
					(boundary == "pod-read" && isGet && resource == "pods" && !created && !nameOnly) ||
					(boundary == "pod-patch" && verb == "patch") ||
					(boundary == "rule-exists" && isGet && resource == "networkpolicies" && nameOnly && !created) ||
					(boundary == "rule-create-after-commit" && verb == "create") ||
					(strings.HasPrefix(boundary, "policy-readback-after-create") && isGet && resource == "networkpolicies" && !nameOnly && created) ||
					(boundary == "final-claim-after-create" && isGet && resource == "sandboxclaims" && !nameOnly && created) ||
					(boundary == "final-pod-after-create" && isGet && resource == "pods" && !nameOnly && created)
				if match && !blocked {
					blocked = true
					// Copy actual-write evidence on the transport goroutine BEFORE
					// the deadline can unblock it and run trusted cleanup. The test
					// must not inspect mutable fake state while expiry deletes it.
					entered <- milestone{created: created, rulePresent: k.policy != nil}
					<-ctx.Done() // the only release; Background/fresh-timeout fails test
					return nil, ctx.Err()
				}
				switch verb {
				case "get":
					if nameOnly {
						present, err := k.exists(resource, name)
						if err != nil {
							return nil, err
						}
						if !present {
							return nil, nil
						}
						return []byte(resource + "/" + name), nil
					}
					var obj any
					if err := k.get(resource, name, &obj); err != nil {
						return nil, err
					}
					return json.Marshal(obj)
				case "create":
					return []byte(`{}`), nil
				case "patch":
					var patch []map[string]string
					if input != nil || len(argv) != 6 || argv[3] != "--type=json" || argv[4] != "--patch" || json.Unmarshal([]byte(argv[5]), &patch) != nil || len(patch) != 3 {
						return nil, errors.New("invalid guarded patch")
					}
					return nil, k.patchPodIncarnation(argv[2], patch[0]["value"], patch[1]["value"], patch[2]["value"])
				case "delete":
					if ctx.Err() != nil {
						return nil, errors.New("trusted cleanup inherited canceled activation")
					}
					cleaned = true
					if len(argv) != 5 || argv[1] != "--raw" {
						return nil, errors.New("unbounded cleanup command")
					}
					segments := strings.Split(argv[2], "/")
					if len(segments) < 3 {
						return nil, errors.New("invalid resource path")
					}
					var opts struct {
						Preconditions struct{ UID, ResourceVersion string }
					}
					if json.Unmarshal(input, &opts) != nil {
						return nil, errors.New("invalid preconditions")
					}
					return nil, k.deletePinned(segments[len(segments)-2], segments[len(segments)-1], opts.Preconditions.UID, opts.Preconditions.ResourceVersion)
				default:
					return nil, errors.New("unexpected test command")
				}
			}
			a.kube = r
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				proof runtime.NetworkEvidence
				err   error
			}
			done := make(chan result, 1)
			go func() { proof, err := a.ActivateNetwork(ctx, alloc.Identity); done <- result{proof, err} }()
			var observed milestone
			select {
			case observed = <-entered:
			case <-time.After(time.Second):
				t.Fatal("activation never reached intended blocked transport")
			}
			postGrant := strings.Contains(boundary, "after-")
			if postGrant && (!observed.created || !observed.rulePresent) {
				t.Fatal("post-grant regression did not start with a real simulated rule")
			}
			wantErr := error(context.Canceled)
			if earlierExpiry {
				wantErr = context.DeadlineExceeded
			} else {
				cancel()
			}
			select {
			case result := <-done:
				if !errors.Is(result.err, wantErr) || result.proof.State != "" {
					t.Fatalf("canceled transport returned Active %+v %v", result.proof, result.err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation could not interrupt actual command; live grant blocked behind background read")
			}
			if earlierExpiry && ctx.Err() != nil {
				t.Fatal("test relied on caller cancellation rather than earlier backend expiry")
			}
			if !cleaned || k.policy != nil || k.podPresent || len(k.claims) != 0 || f.stops != 0 {
				t.Fatalf("actual canceled transport skipped independent trusted cleanup: cleaned=%v policy=%v pod=%v claims=%d stops=%d", cleaned, k.policy, k.podPresent, len(k.claims), f.stops)
			}
		})
	}
}

func TestNetworkActivationReadTransportDoesNotExposeRawFailureBody(t *testing.T) {
	r := newKubectlRunner("test", "agenova-test")
	r.command = func(context.Context, []byte, ...string) ([]byte, error) {
		return []byte("SENSITIVE_TRANSPORT_BODY"), errors.New("bounded command failure")
	}
	var obj map[string]any
	if err := r.getContext(context.Background(), "networkpolicies", "owned", &obj); err == nil || strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatal("raw activation read failure escaped")
	}
	if _, err := r.existsContext(context.Background(), "networkpolicies", "owned"); err == nil || strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatal("raw activation existence failure escaped")
	}
}
