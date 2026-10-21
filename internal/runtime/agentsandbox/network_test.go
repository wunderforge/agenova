// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// networkKube is controller/API simulation only. It cannot prove packet drops.
type networkKube struct {
	*fakeKube
	policy             map[string]any
	podPresent         bool
	retainPod          bool
	wrongPodBinding    bool
	policyErr          error
	ruleDeleteErr      error
	claimUID           string
	podUID             string
	podNetworkLabel    string
	labelledPodUID     string
	patchErr           error
	replaceBeforePatch bool
	replaceAfterPatch  bool
	patches            int
	afterGet           func(string)
	afterPatch         func()
	afterExists        func(string)
	afterPolicyCreate  func()
	createResponseErr  error
	networkCreates     int
	createDeadline     time.Time
	createContext      context.Context
}

func (k *networkKube) applyBytes(data []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj["kind"] == "NetworkPolicy" {
		if k.policyErr != nil {
			return k.policyErr
		}
		k.policy = obj
		return nil
	}
	if err := k.fakeKube.applyBytes(data); err != nil {
		return err
	}
	var claim upstreamSandboxClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return err
	}
	if k.claimUID == "" {
		k.claimUID = "uid-claim-one"
	}
	k.claims[claim.Metadata.Name].Metadata.UID = k.claimUID
	k.claims[claim.Metadata.Name].Metadata.ResourceVersion = "1"
	k.claims[claim.Metadata.Name].Metadata.Annotations = claim.Metadata.Annotations
	k.claims[claim.Metadata.Name].Metadata.Labels = claim.Metadata.Labels
	k.claims[claim.Metadata.Name].Spec = claim.Spec
	k.podPresent = true
	return nil
}

func (k *networkKube) get(resource, name string, dst any) error {
	defer func() {
		if k.afterGet != nil {
			k.afterGet(resource)
		}
	}()
	if resource == "networkpolicies" {
		data, _ := json.Marshal(k.policy)
		return json.Unmarshal(data, dst)
	}
	if resource != "pods" {
		return k.fakeKube.get(resource, name, dst)
	}
	uid := k.claimUID
	if k.wrongPodBinding {
		uid = "uid-another-claim"
	}
	if k.podUID == "" {
		k.podUID = "uid-pod-one"
	}
	labels := map[string]string{claimUIDLabel: uid}
	if k.labelledPodUID == k.podUID && k.podNetworkLabel != "" {
		labels[networkIncarnationLabel] = k.podNetworkLabel
	}
	data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"name": name, "uid": k.podUID, "labels": labels}})
	return json.Unmarshal(data, dst)
}

func (k *networkKube) delete(resource, name string) error {
	if resource == "networkpolicies" {
		if k.deleteErr != nil {
			return k.deleteErr
		}
		k.policy = nil
		return nil
	}
	if err := k.fakeKube.delete(resource, name); err != nil {
		return err
	}
	if resource == "sandboxclaims" && !k.retainPod {
		k.podPresent = false
	}
	return nil
}

func (k *networkKube) exists(resource, name string) (bool, error) {
	defer func() {
		if k.afterExists != nil {
			k.afterExists(resource)
		}
	}()
	if resource == "networkpolicies" {
		return k.policy != nil, k.existsErr
	}
	if resource == "pods" {
		return k.podPresent, k.existsErr
	}
	return k.fakeKube.exists(resource, name)
}

func (k *networkKube) createNetworkPolicyBefore(ctx context.Context, data []byte, expiry time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k.networkCreates++
	k.createContext = ctx
	k.createDeadline = expiry
	err := k.createBytes(data)
	if k.afterPolicyCreate != nil {
		k.afterPolicyCreate()
	}
	return errors.Join(err, k.createResponseErr)
}

func TestNetworkActivationCancellationAtEveryRemoteBoundaryCompensates(t *testing.T) {
	for _, boundary := range []string{"before-activation", "claim-read", "pod-read", "pod-patch", "rule-exists", "rule-create", "rule-readback", "final-binding", "offline-binding"} {
		t.Run(boundary, func(t *testing.T) {
			profiles := []string{"public-web"}
			if boundary == "offline-binding" {
				profiles = nil
			}
			a, k, f, alloc := networkFixture(t, profiles)
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			claimReads := 0
			k.afterGet = func(resource string) {
				if resource == "sandboxclaims" {
					claimReads++
				}
				if (boundary == "claim-read" && resource == "sandboxclaims") || (boundary == "pod-read" && resource == "pods") || (boundary == "rule-readback" && resource == "networkpolicies") || (boundary == "final-binding" && resource == "sandboxclaims" && claimReads == 2) || (boundary == "offline-binding" && resource == "pods") {
					cancel()
				}
			}
			if boundary == "pod-patch" {
				k.afterPatch = cancel
			}
			if boundary == "rule-exists" {
				k.afterExists = func(resource string) {
					if resource == "networkpolicies" {
						cancel()
					}
				}
			}
			if boundary == "rule-create" {
				k.afterPolicyCreate = cancel
			}
			if boundary == "before-activation" {
				cancel()
			}
			evidence, err := a.ActivateNetwork(ctx, alloc.Identity)
			if !errors.Is(err, context.Canceled) || evidence.State != "" {
				t.Fatalf("canceled activation returned proof: %+v %v", evidence, err)
			}
			if k.policy != nil || k.podPresent || len(k.claims) != 0 || f.stops != 0 {
				t.Fatalf("cancellation cleanup inherited canceled context or trusted image: policy=%v pod=%v claims=%d stops=%d", k.policy, k.podPresent, len(k.claims), f.stops)
			}
			created := boundary == "rule-create" || boundary == "rule-readback" || boundary == "final-binding"
			if (!created && k.networkCreates != 0) || (created && k.networkCreates != 1) {
				t.Fatalf("created grant after cancellation boundary: %d", k.networkCreates)
			}
			if created && k.createContext != ctx {
				t.Fatal("caller context not forwarded to grant write")
			}
		})
	}
}

func TestNetworkCanceledActivationPreservesCleanupAndForeignIdentityErrors(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanupErr := errors.New("rule deletion unavailable")
	k.afterPolicyCreate = cancel
	k.ruleDeleteErr = cleanupErr
	evidence, err := a.ActivateNetwork(ctx, alloc.Identity)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cleanupErr) || evidence.State != "" || k.podPresent {
		t.Fatalf("cancellation/cleanup error hidden %+v %v", evidence, err)
	}
	// Compensation is still UID bound if a claim name is replaced while read is slow.
	a, k, _, alloc = networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	k.afterGet = func(resource string) {
		if resource == "sandboxclaims" {
			k.claims[resourceName("claim", "run-1")].Metadata.UID = "foreign-claim"
			cancel()
		}
	}
	_, err = a.ActivateNetwork(ctx, alloc.Identity)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, errIdentityMismatch) || k.delets != 0 || k.policy != nil {
		t.Fatalf("canceled activation touched replacement: %v deletes=%d", err, k.delets)
	}
}

func TestNetworkPolicyCreateTransportInheritsCancellationAndEarlierCallerDeadline(t *testing.T) {
	r := newKubectlRunner("test", "work")
	expiry := time.Now().Add(time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	r.command = func(context.Context, []byte, ...string) ([]byte, error) { called = true; return nil, nil }
	if err := r.createNetworkPolicyBefore(ctx, nil, expiry); !errors.Is(err, context.Canceled) || called {
		t.Fatalf("pre-canceled write started: %v called=%v", err, called)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	r.command = func(child context.Context, _ []byte, _ ...string) ([]byte, error) {
		cancel()
		<-child.Done()
		return nil, child.Err()
	}
	if err := r.createNetworkPolicyBefore(ctx, nil, expiry); !errors.Is(err, context.Canceled) {
		t.Fatal("active write ignored cancellation", err)
	}
	deadline := time.Now().Add(15 * time.Millisecond)
	ctx, cancel = context.WithDeadline(context.Background(), deadline)
	defer cancel()
	r.command = func(child context.Context, _ []byte, _ ...string) ([]byte, error) {
		actual, ok := child.Deadline()
		if !ok || actual.After(deadline) {
			t.Fatal("write outlives earlier caller deadline")
		}
		<-child.Done()
		return nil, child.Err()
	}
	if err := r.createNetworkPolicyBefore(ctx, nil, expiry); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func lifetimeGuard(t *testing.T, a *ControlledAdapter, ctx context.Context, id v1alpha1.SandboxClaimBackendIdentity) *networkLifetime {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	g := a.networkLifetimes[id.WorkerID][ctx.Done()]
	if g == nil {
		t.Fatal("successful activation did not retain caller lifetime")
	}
	return g
}
func awaitLifetime(t *testing.T, g *networkLifetime) {
	t.Helper()
	select {
	case <-g.complete:
	case <-time.After(time.Second):
		t.Fatal("activation lifetime did not complete trusted safety action")
	}
}

func TestNetworkActivationLifetimeRevokesWithoutApplicationProgress(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evidence, err := a.ActivateNetwork(ctx, alloc.Identity)
	if err != nil || evidence.State != "Active" {
		t.Fatalf("activate %+v %v", evidence, err)
	}
	g := lifetimeGuard(t, a, ctx, alloc.Identity)
	// Simulate an application journal never returning after receiving Active:
	// no app teardown, explicit revoke or additional adapter call is made.
	cancel()
	awaitLifetime(t, g)
	if g.err != nil || k.policy != nil || k.podPresent || len(k.claims) != 0 {
		t.Fatalf("grant survived blocked application: err=%v policy=%v pod=%v", g.err, k.policy, k.podPresent)
	}
	a.mu.Lock()
	remaining := len(a.networkLifetimes)
	a.mu.Unlock()
	if remaining != 0 {
		t.Fatal("completed lifetime retained callback references")
	}
}

func TestNetworkActivationLifetimeDeadlineRevokesWithoutApplicationProgress(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := a.ActivateNetwork(ctx, alloc.Identity); err != nil {
		t.Fatal(err)
	}
	g := lifetimeGuard(t, a, ctx, alloc.Identity)
	awaitLifetime(t, g)
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || g.err != nil || k.policy != nil || k.podPresent {
		t.Fatalf("deadline did not revoke trusted grant: %v %v", ctx.Err(), g.err)
	}
}

func TestNetworkExplicitRevokeDeregistersAllActivationLifetimes(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	for _, ctx := range []context.Context{ctx1, ctx1, ctx2} {
		if _, err := a.ActivateNetwork(ctx, alloc.Identity); err != nil {
			t.Fatal(err)
		}
	}
	g1, g2 := lifetimeGuard(t, a, ctx1, alloc.Identity), lifetimeGuard(t, a, ctx2, alloc.Identity)
	a.mu.Lock()
	count := len(a.networkLifetimes[alloc.Identity.WorkerID])
	a.mu.Unlock()
	if count != 2 {
		t.Fatal("repeated activation duplicated original lifetime")
	}
	if _, err := a.RevokeNetwork(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	awaitLifetime(t, g1)
	awaitLifetime(t, g2)
	cancel1()
	cancel2()
	a.mu.Lock()
	count = len(a.networkLifetimes)
	a.mu.Unlock()
	if count != 0 || k.policy != nil || k.podPresent {
		t.Fatal("explicit release retained live callbacks or grant")
	}
}

func TestNetworkLifetimeAndExplicitRevokeAreIdempotentTogether(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := a.ActivateNetwork(ctx, alloc.Identity); err != nil {
		t.Fatal(err)
	}
	g := lifetimeGuard(t, a, ctx, alloc.Identity)
	done := make(chan error, 1)
	go func() { _, err := a.RevokeNetwork(alloc.Identity); done <- err }()
	cancel()
	awaitLifetime(t, g)
	if err := <-done; err != nil || g.err != nil || k.policy != nil || k.podPresent {
		t.Fatalf("concurrent cleanup failed %v %v", err, g.err)
	}
}

func TestNetworkLifetimeFailureRetainsDiagnosisAndRejectsReactivation(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := a.ActivateNetwork(ctx, alloc.Identity); err != nil {
		t.Fatal(err)
	}
	g := lifetimeGuard(t, a, ctx, alloc.Identity)
	k.retainPod = true
	cancel()
	awaitLifetime(t, g)
	if !errors.Is(g.err, errCleanupUnconfirmed) || k.policy != nil || !k.podPresent {
		t.Fatalf("unconfirmed release hidden %v", g.err)
	}
	if evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil || evidence.State != "" {
		t.Fatalf("new context revived canceled allocation %+v %v", evidence, err)
	}
	k.podPresent = false
	if _, err := a.RevokeNetwork(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	remaining := len(a.networkLifetimes)
	a.mu.Unlock()
	if remaining != 0 {
		t.Fatal("successful retry retained lifetime failure")
	}
}

func TestNetworkLifetimeCannotDeleteSameNameForeignReplacement(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := a.ActivateNetwork(ctx, alloc.Identity); err != nil {
		t.Fatal(err)
	}
	g := lifetimeGuard(t, a, ctx, alloc.Identity)
	k.claims[resourceName("claim", "run-1")].Metadata.UID = "foreign-claim"
	cancel()
	awaitLifetime(t, g)
	if !errors.Is(g.err, errIdentityMismatch) || k.delets != 0 || k.claims[resourceName("claim", "run-1")].Metadata.UID != "foreign-claim" {
		t.Fatalf("lifetime cleanup touched foreign incarnation %v deletes=%d", g.err, k.delets)
	}
}

func (k *networkKube) createBytes(data []byte) error {
	var header struct {
		Kind     string
		Metadata upstreamMeta
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.Kind == kindSandboxClaim {
		if _, exists := k.claims[header.Metadata.Name]; exists {
			return errors.New("AlreadyExists")
		}
		return k.applyBytes(data)
	}
	if k.policy != nil {
		return errors.New("AlreadyExists")
	}
	if err := k.applyBytes(data); err != nil {
		return err
	}
	m := k.policy["metadata"].(map[string]any)
	m["uid"], m["resourceVersion"] = "uid-network-one", "1"
	return nil
}

func (k *networkKube) deletePinned(resource, name, uid, version string) error {
	if resource == "networkpolicies" && k.ruleDeleteErr != nil {
		return k.ruleDeleteErr
	}
	if uid == "" || version == "" {
		return errIdentityMismatch
	}
	if resource == "sandboxclaims" && k.claims[name].Metadata.UID != uid {
		return errIdentityMismatch
	}
	return k.delete(resource, name)
}

func (k *networkKube) patchPodIncarnation(name, uid, claimUID, value string) error {
	defer func() {
		if k.afterPatch != nil {
			k.afterPatch()
		}
	}()
	k.patches++
	if k.patchErr != nil {
		return k.patchErr
	}
	if k.replaceBeforePatch {
		k.podUID = "replacement-pod"
	}
	if uid != k.podUID || claimUID != k.claimUID || name != k.bindOnApply {
		return errIdentityMismatch
	}
	k.podNetworkLabel, k.labelledPodUID = value, uid
	if k.replaceAfterPatch {
		k.podUID = "replacement-pod"
	}
	return nil
}

func networkFixture(t *testing.T, profiles []string) (*ControlledAdapter, *networkKube, *fakeWorkerControl, runtime.Allocation) {
	t.Helper()
	k := &networkKube{fakeKube: newFakeKube()}
	k.bindOnApply, k.readyOnApply = "sbx-1", true
	f := &fakeWorkerControl{claim: "run-1", worker: "sbx-1", state: "idle"}
	a := newControlledAdapter(newTestAdapter(k.fakeKube), f)
	a.kube = k
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	alloc, err := a.Allocate(runtime.AllocateRequest{ClaimID: f.claim, TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{Profiles: profiles, ExpiresAt: time.Now().Add(time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	return a, k, f, alloc
}

func TestNetworkUnsupportedFailsBeforeAllocation(t *testing.T) {
	k := newFakeKube()
	a := newTestAdapter(k)
	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "blocked", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: time.Now().Add(time.Minute)}})
	if !errors.Is(err, runtime.ErrUnsupported) || k.applies != 0 || len(a.allocations) != 0 {
		t.Fatalf("unsupported allocated: %v %+v", err, k)
	}
}

func TestNetworkActivationExpiryAtEveryRemoteBoundaryCompensates(t *testing.T) {
	for _, boundary := range []string{"claim-read", "pod-read", "pod-patch", "rule-exists", "rule-create", "rule-readback", "final-binding", "offline-binding"} {
		t.Run(boundary, func(t *testing.T) {
			profiles := []string{"public-web"}
			if boundary == "offline-binding" {
				profiles = nil
			}
			a, k, f, alloc := networkFixture(t, profiles)
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			expiry := a.allocations["run-1"].network.ExpiresAt
			a.networkNow = func() time.Time { return now }
			advance := func() { now = expiry }
			claimReads := 0
			k.afterGet = func(resource string) {
				if resource == "sandboxclaims" {
					claimReads++
				}
				if (boundary == "claim-read" && resource == "sandboxclaims") || (boundary == "pod-read" && resource == "pods") || (boundary == "rule-readback" && resource == "networkpolicies") || (boundary == "final-binding" && resource == "sandboxclaims" && claimReads == 2) || (boundary == "offline-binding" && resource == "pods") {
					advance()
				}
			}
			if boundary == "pod-patch" {
				k.afterPatch = advance
			}
			if boundary == "rule-exists" {
				k.afterExists = func(resource string) {
					if resource == "networkpolicies" {
						advance()
					}
				}
			}
			if boundary == "rule-create" {
				k.afterPolicyCreate = advance
			}
			evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity)
			if err == nil || evidence.State != "" {
				t.Fatalf("expired activation returned authority: %+v %v", evidence, err)
			}
			if k.policy != nil || k.podPresent || len(k.claims) != 0 || f.stops != 0 {
				t.Fatalf("expiry cleanup not trusted/complete: policy=%v pod=%v claims=%d stops=%d", k.policy, k.podPresent, len(k.claims), f.stops)
			}
			created := boundary == "rule-create" || boundary == "rule-readback" || boundary == "final-binding"
			if (!created && k.networkCreates != 0) || (created && k.networkCreates != 1) {
				t.Fatalf("creation crossed stale boundary: %d", k.networkCreates)
			}
			if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil {
				t.Fatal("expired allocation reactivated")
			}
		})
	}
}

func TestNetworkActivationAmbiguousCreateRevokesOwnedRuleAndDestroysWorker(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprint("expired-", expired), func(t *testing.T) {
			a, k, _, alloc := networkFixture(t, []string{"public-web"})
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			transportErr := errors.New("ambiguous create response")
			k.createResponseErr = transportErr
			if expired {
				now := time.Now()
				a.networkNow = func() time.Time { return now }
				k.afterPolicyCreate = func() { now = a.allocations["run-1"].network.ExpiresAt }
			}
			evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity)
			if !errors.Is(err, transportErr) || evidence.State != "" || k.policy != nil || k.podPresent || len(k.claims) != 0 {
				t.Fatalf("ambiguous grant escaped compensation: %+v %v pod=%v policy=%v", evidence, err, k.podPresent, k.policy)
			}
		})
	}
}

func TestNetworkActivationExpiryCleanupFailureIsNotHidden(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a.networkNow = func() time.Time { return now }
	k.afterPolicyCreate = func() { now = a.allocations["run-1"].network.ExpiresAt }
	cleanupErr := errors.New("rule cleanup unavailable")
	k.ruleDeleteErr = cleanupErr
	evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity)
	if !errors.Is(err, cleanupErr) || evidence.State != "" || k.podPresent {
		t.Fatalf("cleanup failure hidden or live worker retained: %+v %v", evidence, err)
	}
	if k.policy == nil {
		t.Fatal("test did not retain failed-delete rule")
	}
}

func TestNetworkActivationWriteUsesEarlierUpstreamExpiry(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	earlier := time.Now().Add(10 * time.Second)
	k.claims[resourceName("claim", "run-1")].Spec.Lifecycle.ShutdownTime = earlier.Format(time.RFC3339Nano)
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if !k.createDeadline.Equal(earlier) {
		t.Fatalf("write deadline %v exceeds earlier upstream deadline %v", k.createDeadline, earlier)
	}
}

func TestNetworkPolicyCreateTransportBoundedByAuthorityDeadline(t *testing.T) {
	r := newKubectlRunner("test", "work")
	expiry := time.Now().Add(20 * time.Millisecond)
	called := false
	r.command = func(ctx context.Context, _ []byte, args ...string) ([]byte, error) {
		called = true
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(expiry) {
			t.Fatal("write context outlives authority")
		}
		if !strings.Contains(strings.Join(args, " "), "create -f -") {
			t.Fatal(args)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := r.createNetworkPolicyBefore(context.Background(), []byte(`{}`), expiry); !errors.Is(err, context.DeadlineExceeded) || !called {
		t.Fatalf("unbounded grant write: %v called=%v", err, called)
	}
	called = false
	if err := r.createNetworkPolicyBefore(context.Background(), nil, time.Now().Add(-time.Second)); !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatalf("expired write was started: %v called=%v", err, called)
	}
}

func TestNetworkWarmTemplateHasCustomDenyNotUpstreamPublicDefault(t *testing.T) {
	k := &templateCaptureKube{}
	a := newSpikeAdapter(k, "test")
	if err := a.AddTemplate(v1alpha1.AgentSandboxTemplate{Metadata: v1alpha1.ObjectMeta{Name: "test"}, Spec: v1alpha1.AgentSandboxTemplateSpec{Image: "test:one"}}); err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(k.manifest, &obj); err != nil {
		t.Fatal(err)
	}
	spec := obj["spec"].(map[string]any)
	if spec["networkPolicy"] == nil || spec["networkPolicyManagement"] != "Managed" || len(spec["networkPolicy"].(map[string]any)) != 0 {
		t.Fatalf("missing explicit empty custom policy: %s", k.manifest)
	}
	if strings.Contains(string(k.manifest), networkIncarnationLabel) {
		t.Fatal("network incarnation propagated through warm template")
	}
}

func TestNetworkActivationIsWorkBoundAndExpiresIndependently(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil || k.policy != nil {
		t.Fatal("idle worker gained access")
	}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity)
	if err != nil || evidence.State != "Active" || evidence.EvidenceLevel != "Configured" {
		t.Fatalf("activate: %+v %v", evidence, err)
	}
	data, _ := json.Marshal(k.policy)
	text := string(data)
	for _, want := range []string{`"agents.x-k8s.io/claim-uid":"uid-claim-one"`, `"port":443`, `"k8s-app":"kube-dns"`, `"kubernetes.io/metadata.name":"kube-system"`, `169.254.0.0/16`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	claim := k.claims[resourceName("claim", "run-1")]
	deadline, err := time.Parse(time.RFC3339Nano, claim.Spec.Lifecycle.ShutdownTime)
	if err != nil || !deadline.Equal(evidence.ExpiresAt) {
		t.Fatalf("expiry not upstream: %v %v", deadline, err)
	}
	// Defensive evidence cannot mutate authority.
	evidence.Profiles[0] = "other"
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkOfflineNeverCreatesAllowPolicy(t *testing.T) {
	a, k, _, alloc := networkFixture(t, nil)
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	evidence, err := a.ActivateNetwork(context.Background(), alloc.Identity)
	if err != nil || len(evidence.Profiles) != 0 || k.policy != nil || k.patches != 0 {
		t.Fatalf("offline grant: %+v %v", evidence, err)
	}
}

func simulatedPodMatchesNetworkRule(k *networkKube) bool {
	if k.policy == nil {
		return false
	}
	selector := k.policy["spec"].(map[string]any)["podSelector"].(map[string]any)["matchLabels"].(map[string]any)
	return selector[claimUIDLabel] == k.claimUID && selector[networkIncarnationLabel] == k.podNetworkLabel && k.labelledPodUID == k.podUID && k.podNetworkLabel != ""
}

func TestNetworkConcretePodIncarnationNeverTransfersToSameClaimReplacement(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if !simulatedPodMatchesNetworkRule(k) {
		t.Fatal("original Pod did not match concrete rule")
	}
	k.podUID = "replacement-pod" // same claim UID and Pod name, different process incarnation
	if simulatedPodMatchesNetworkRule(k) {
		t.Fatal("replacement inherited original Pod network grant")
	}
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil {
		t.Fatal("replacement was relabelled as original")
	}
	claim, _ := json.Marshal(k.claims[resourceName("claim", "run-1")])
	if strings.Contains(string(claim), networkIncarnationLabel) {
		t.Fatal("incarnation escaped into claim/template inheritance")
	}
}

func TestNetworkGuardedPodPatchRejectsReplacementAndFailedPatchBeforeGrant(t *testing.T) {
	for _, scenario := range []string{"replacement-before-patch", "replacement-after-patch", "patch-failure"} {
		t.Run(scenario, func(t *testing.T) {
			a, k, _, alloc := networkFixture(t, []string{"public-web"})
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "replacement-before-patch":
				k.replaceBeforePatch = true
			case "replacement-after-patch":
				k.replaceAfterPatch = true
			case "patch-failure":
				k.patchErr = errors.New("patch unavailable")
			}
			if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil {
				t.Fatal("unsafe activation succeeded")
			}
			if simulatedPodMatchesNetworkRule(k) {
				t.Fatal("replacement/failed patch received usable grant")
			}
			if scenario != "replacement-after-patch" && k.policy != nil {
				t.Fatal("grant created despite failed guarded patch")
			}
		})
	}
}

func TestNetworkConcurrentActivationPinsSingleConcretePod(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if k.patches != 12 || !simulatedPodMatchesNetworkRule(k) || k.podNetworkLabel != networkIncarnation(k.claimUID, k.podUID) {
		t.Fatal("concurrent grants lost original incarnation")
	}
}

func TestNetworkPodPatchUsesAtomicUIDAndClaimTests(t *testing.T) {
	r := newKubectlRunner("test", "work")
	called := false
	r.command = func(_ context.Context, stdin []byte, args ...string) ([]byte, error) {
		called = true
		var patch []map[string]string
		if err := json.Unmarshal(stdin, &patch); err != nil {
			t.Fatal(err)
		}
		if len(patch) != 3 || patch[0]["op"] != "test" || patch[0]["path"] != "/metadata/uid" || patch[0]["value"] != "original-pod" || patch[1]["op"] != "test" || patch[1]["path"] != "/metadata/labels/agents.x-k8s.io~1claim-uid" || patch[1]["value"] != "original-claim" || patch[2]["op"] != "add" || patch[2]["path"] != "/metadata/labels/agenova.io~1network-incarnation" || patch[2]["value"] != networkIncarnation("original-claim", "original-pod") {
			t.Fatalf("unsafe patch: %s", stdin)
		}
		if !strings.Contains(strings.Join(args, " "), "patch pods worker --type=json --patch-file=-") {
			t.Fatalf("wrong patch command %v", args)
		}
		return nil, nil
	}
	if err := r.patchPodIncarnation("worker", "original-pod", "original-claim", networkIncarnation("original-claim", "original-pod")); err != nil || !called {
		t.Fatalf("patch %v called=%v", err, called)
	}
	if err := r.patchPodIncarnation("worker", "original-pod", "original-claim", "forged"); err == nil {
		t.Fatal("arbitrary label value accepted")
	}
}

func TestNetworkForgedBindingAndFailedGrantFailClosed(t *testing.T) {
	for _, name := range []string{"forged", "apply-failure", "expiry-missing"} {
		t.Run(name, func(t *testing.T) {
			a, k, _, alloc := networkFixture(t, []string{"public-web"})
			if err := a.Start(alloc.Identity); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "forged":
				k.wrongPodBinding = true
			case "apply-failure":
				k.policyErr = errors.New("no grant")
			case "expiry-missing":
				k.claims[resourceName("claim", "run-1")].Spec.Lifecycle.ShutdownTime = ""
			}
			if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil || k.policy != nil {
				t.Fatal("invalid authority granted")
			}
		})
	}
}

func TestNetworkRevokeRequiresActualPodAbsenceNotImageAcknowledgement(t *testing.T) {
	a, k, f, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
		t.Fatal(err)
	}
	k.retainPod = true
	if _, err := a.RevokeNetwork(alloc.Identity); !errors.Is(err, errCleanupUnconfirmed) {
		t.Fatalf("orphan Pod treated revoked: %v", err)
	}
	if k.policy != nil {
		t.Fatal("grant was retained during failed termination")
	}
	if f.stops != 0 {
		t.Fatal("network release trusted image stop")
	}
	k.podPresent = false
	evidence, err := a.RevokeNetwork(alloc.Identity)
	if err != nil || evidence.State != "Revoked" {
		t.Fatalf("revoke: %+v %v", evidence, err)
	}
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if result, err := a.Cleanup(alloc.Identity); err != nil || !result.Released {
		t.Fatalf("cleanup %+v %v", result, err)
	}
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil {
		t.Fatal("terminated authority reactivated")
	}
}

func TestNetworkIncarnationRejectsCoherentSameNameReplacement(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	// Same names, coherent current UID labels, but NOT this allocation's UID.
	k.claimUID, k.podUID = "replacement-claim", "replacement-pod"
	k.claims[resourceName("claim", "run-1")].Metadata.UID = k.claimUID
	if err := a.Start(alloc.Identity); err == nil {
		t.Fatal("replacement started as original")
	}
	if _, err := a.RevokeNetwork(alloc.Identity); err == nil || k.delets != 0 {
		t.Fatalf("replacement deleted: %v deletes=%d", err, k.delets)
	}
	restored := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	restored.kube = k
	if err := restored.RestoreForCleanup("run-1", alloc.Identity); err == nil || k.delets != 0 {
		t.Fatalf("recovery adopted replacement: %v", err)
	}
}

func TestNetworkRecoveryRequiresDurableOriginalIncarnation(t *testing.T) {
	_, k, _, alloc := networkFixture(t, []string{"public-web"})
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	legacy := alloc.Identity
	legacy.AllocationID = ""
	if err := a.RestoreForCleanup("run-1", legacy); err == nil {
		t.Fatal("new network allocation adopted as legacy")
	}
	if err := a.RestoreForCleanup("run-1", alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RevokeNetwork(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if k.podPresent {
		t.Fatal("recovered worker not destroyed")
	}
}

func TestNetworkForeignRuleCannotBeOverwrittenOrDeleted(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	foreign := a.publicWebPolicy(a.allocations["run-1"], "foreign-uid")
	foreign["metadata"].(map[string]any)["uid"] = "foreign-policy"
	foreign["metadata"].(map[string]any)["resourceVersion"] = "1"
	k.policy = foreign
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err == nil {
		t.Fatal("foreign rule overwritten")
	}
	if _, err := a.RevokeNetwork(alloc.Identity); err == nil {
		t.Fatal("foreign ownership hidden")
	}
	if k.policy == nil {
		t.Fatal("foreign rule deleted")
	}
	if k.podPresent {
		t.Fatal("rule ownership failure skipped own worker destruction")
	}
}

func TestNetworkRuleDeleteFailureStillDestroysWorker(t *testing.T) {
	a, k, _, alloc := networkFixture(t, []string{"public-web"})
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ActivateNetwork(context.Background(), alloc.Identity); err != nil {
		t.Fatal(err)
	}
	k.ruleDeleteErr = errors.New("rule deletion unavailable")
	if _, err := a.RevokeNetwork(alloc.Identity); err == nil || k.podPresent {
		t.Fatalf("failed rule retained live worker: %v pod=%v", err, k.podPresent)
	}
	k.ruleDeleteErr = nil
	if _, err := a.RevokeNetwork(alloc.Identity); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkForeignClaimIsNeverAppliedOrCompensated(t *testing.T) {
	k := &networkKube{fakeKube: newFakeKube()}
	name := resourceName("claim", "run-1")
	k.claims[name] = &upstreamSandboxClaim{Metadata: upstreamMeta{Name: name, UID: "foreign-claim"}, Status: upstreamSandboxClaimStatus{Sandbox: &upstreamSandboxRef{Name: "foreign-worker"}}}
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: time.Now().Add(time.Minute)}}); err == nil {
		t.Fatal("foreign claim adopted")
	}
	if k.delets != 0 || k.claims[name].Metadata.UID != "foreign-claim" {
		t.Fatal("foreign claim modified or compensated")
	}
}

func TestNetworkIsolationGenerationNeverReusesLegacyWarmPool(t *testing.T) {
	template := imageTemplate("example/worker:v1")
	data, _ := json.Marshal(template)
	digest := sha256.Sum256(data)
	legacyRef := "at-" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:20]))
	k := &isolatedTemplateKube{objects: map[string][]byte{}}
	a := newSpikeAdapter(k, "test")
	ref, err := a.ConfigureAgentTemplate(template)
	if err != nil || ref == legacyRef {
		t.Fatalf("legacy warm identity reused: %s %v", ref, err)
	}
}

func TestNetworkPinnedDeleteSendsUIDAndResourceVersionPreconditions(t *testing.T) {
	r := newKubectlRunner("test", "work")
	r.command = func(_ context.Context, stdin []byte, args ...string) ([]byte, error) {
		var options struct {
			Preconditions map[string]string `json:"preconditions"`
		}
		if err := json.Unmarshal(stdin, &options); err != nil {
			t.Fatal(err)
		}
		if options.Preconditions["uid"] != "original-uid" || options.Preconditions["resourceVersion"] != "42" || !strings.Contains(strings.Join(args, " "), "/apis/networking.k8s.io/v1/namespaces/work/networkpolicies/owned") {
			t.Fatalf("unsafe delete: %s %v", stdin, args)
		}
		return nil, nil
	}
	if err := r.deletePinned("networkpolicies", "owned", "original-uid", "42"); err != nil {
		t.Fatal(err)
	}
}
