// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/runtime"
)

// Literal oracle for AWS network-policy-agent v1.4.0 GetPodIdentifier. This is
// cache-key compatibility evidence, not a packet enforcement simulation.
func awsV140PodIdentifier(name, namespace string) string {
	if index := strings.LastIndex(name, "-"); index >= 0 {
		name = name[:index]
	}
	return strings.ReplaceAll(name+"@"+namespace, ".", "_")
}

func TestColdNamesKeepDistinctAWSGroupingAcrossClaimsAndAttempts(t *testing.T) {
	claims := []string{"run-1", "run-2", "run--1", "RUN-1", "run:1", "研究-任务", strings.Repeat("prefix-", 200), strings.Repeat("prefix-", 200) + "other"}
	seen := map[string]bool{}
	for _, claim := range claims {
		for _, attempt := range []string{"first-attempt", "second-attempt"} {
			name := dedicatedClaimName(claim, attempt)
			key := awsV140PodIdentifier(name, "same-namespace")
			if !isDedicatedClaimName(name) || len(name) > 63 || name != dedicatedClaimName(claim, attempt) || seen[key] {
				t.Fatalf("unsafe or colliding CNI key: %q %q", name, key)
			}
			seen[key] = true
		}
	}
	// Establish the old failure mode; a unique suffix alone is insufficient.
	if awsV140PodIdentifier("same-warm-pool-abc", "n") != awsV140PodIdentifier("same-warm-pool-def", "n") {
		t.Fatal("oracle no longer models the supported agent version")
	}
	for _, invalid := range []string{"agenova-nw-w", "agenova-nw-" + strings.Repeat("A", 40) + "-w", "agenova-nw-" + strings.Repeat("g", 40) + "-w", dedicatedClaimName("c", "a") + "-extra"} {
		if isDedicatedClaimName(invalid) {
			t.Fatalf("accepted malformed dedicated name %q", invalid)
		}
	}
}

func TestGovernedOfflineAndPublicAllocationsIgnoreExistingWarmPool(t *testing.T) {
	for _, profiles := range [][]string{nil, {"public-web"}} {
		a, k, _, alloc := networkFixture(t, profiles)
		entry := a.allocations[alloc.ClaimID]
		claim := k.claims[entry.upstreamClaimName]
		if claim.Spec.Warmpool != "none" || alloc.Identity.WorkerID != entry.upstreamClaimName || !isDedicatedClaimName(alloc.Identity.WorkerID) || claim.Spec.SandboxTemplateRef.Name != "agenova-tmpl-tmpl-v1" {
			t.Fatal("governed allocation reused a warm worker", claim, alloc)
		}
		if claim.Metadata.Annotations["agenova.io/allocation-attempt"] != entry.attemptID || dedicatedClaimName(alloc.ClaimID, entry.attemptID) != alloc.Identity.WorkerID || alloc.Identity.AllocationID != "uid-claim-one/uid-pod-one" {
			t.Fatal("cold binding lost attempt or incarnation")
		}
		if claim.Spec.Lifecycle.ShutdownTime != entry.network.ExpiresAt.UTC().Format(time.RFC3339Nano) || len(a.pools) != 1 {
			t.Fatal("cold allocation changed expiry or mutated old warm resources")
		}
	}
}

func TestColdUnexpectedWarmBindingNeverReservedOrDeleted(t *testing.T) {
	k := &networkKube{fakeKube: newFakeKube(), forceWarmBinding: true}
	k.bindOnApply, k.readyOnApply = "foreign-warm-worker", true
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	request := runtime.AllocateRequest{ClaimID: "cold", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: time.Now().Add(time.Minute)}}
	for i := 0; i < 2; i++ {
		if allocation, err := a.Allocate(request); !errors.Is(err, errIdentityMismatch) || allocation.Identity.WorkerID != "" {
			t.Fatal("unexpected warm worker accepted", allocation, err)
		}
		if k.delets != 0 || len(a.byWorker) != 0 || !k.sandboxes["foreign-warm-worker"] || len(k.claims) != 1 || a.allocations["cold"].state != allocRecoveryPending {
			t.Fatal("unsafe compensation/reservation of unexpected worker")
		}
	}
}

func TestControlledTemplateConfigureCreatesNoWarmResourceAndLegacyStillAllocates(t *testing.T) {
	k := &isolatedTemplateKube{objects: map[string][]byte{}}
	a := newControlledAdapter(newSpikeAdapter(k, "controlled"), &fakeWorkerControl{})
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	ref, err := a.ConfigureAgentTemplate(imageTemplate("arbitrary/image:v1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(k.objects) != 1 || len(a.pools) != 0 || !a.coldRefs[ref] {
		t.Fatal("controlled setup created standing warm resources")
	}
	var template upstreamSandboxTemplate
	if err := json.Unmarshal(k.objects[kindSandboxTemplate+"/"+a.poolRefs[ref]], &template); err != nil || template.Spec.NetworkPolicy == nil || template.Spec.PodTemplate.Spec.Containers[0].Image != "arbitrary/image:v1" {
		t.Fatal("lost immutable image or startup deny", err)
	}
	// The old zero-expiry probe contract still works after Configure; it does
	// not get new network evidence or rename historical identities.
	legacy := newFakeKube()
	legacy.bindOnApply = resourceName("claim", "legacy")
	a.kube = legacy
	alloc, err := a.Allocate(runtime.AllocateRequest{ClaimID: "legacy", TemplateRef: ref})
	if err != nil || alloc.Identity.WorkerID != resourceName("claim", "legacy") || alloc.Identity.AllocationID != "" {
		t.Fatal("legacy configured allocation regressed", alloc, err)
	}
}

func TestColdRecoveryRequiresExactAttemptNameAndOriginalUIDs(t *testing.T) {
	for _, scenario := range []string{"valid", "attempt-mismatch", "claim-mismatch", "warm-policy", "missing-incarnation", "missing-claim"} {
		t.Run(scenario, func(t *testing.T) {
			_, k, _, allocation := networkFixture(t, nil)
			id := allocation.Identity
			claim := k.claims[id.WorkerID]
			switch scenario {
			case "attempt-mismatch":
				claim.Metadata.Annotations["agenova.io/allocation-attempt"] = "another-attempt"
			case "claim-mismatch":
				claim.Metadata.Annotations["agenova.io/claim-id"] = "another-work"
			case "warm-policy":
				claim.Spec.Warmpool = "some-pool"
			case "missing-incarnation":
				id.AllocationID = ""
			case "missing-claim":
				delete(k.claims, id.WorkerID)
				delete(k.sandboxes, id.WorkerID)
				k.podPresent = false
			}
			a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
			a.kube = k
			err := a.RestoreForCleanup(allocation.ClaimID, id)
			valid := scenario == "valid" || scenario == "missing-claim"
			if valid && err != nil || !valid && !errors.Is(err, errIdentityMismatch) {
				t.Fatal("wrong cold recovery result", err)
			}
			if k.delets != 0 || !valid && len(a.allocations) != 0 {
				t.Fatal("recovery adopted/deleted mismatched resources")
			}
			if scenario == "valid" && a.allocations[allocation.ClaimID].attemptID != claim.Metadata.Annotations["agenova.io/allocation-attempt"] {
				t.Fatal("cleanup did not retain verified attempt binding")
			}
		})
	}
}

func TestHistoricalGovernedWarmIdentityStillRestoresForCleanup(t *testing.T) {
	_, k, _, allocation := networkFixture(t, nil)
	oldName := resourceName("claim", allocation.ClaimID)
	claim := k.claims[allocation.Identity.WorkerID]
	delete(k.claims, allocation.Identity.WorkerID)
	claim.Metadata.Name = oldName
	claim.Status.Sandbox.Name = "historical-pool-random"
	claim.Spec.Warmpool = "historical-pool"
	k.claims[oldName] = claim
	allocation.Identity.WorkerID = claim.Status.Sandbox.Name
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); err != nil {
		t.Fatal("historical original UID binding no longer recovers", err)
	}
	if a.allocations[allocation.ClaimID].upstreamClaimName != oldName || k.delets != 0 {
		t.Fatal("historical recovery renamed or deleted resources")
	}
}

func TestColdBindExpiryNeverPublishesAllocationAndKeepsUnknownRecovery(t *testing.T) {
	k := &networkKube{fakeKube: newFakeKube()}
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	// Read the created cold claim but simulate a slow controller which has not
	// assigned any Sandbox before expiry. No original worker can be proven.
	k.forceWarmBinding = true
	expiry := time.Now().Add(time.Second)
	var once sync.Once
	k.afterGet = func(resource string) {
		if resource == "sandboxclaims" {
			once.Do(func() {
				// The remote read is already in flight for an actual created
				// claim; do not depend on scheduling a 5ms preallocation window.
				<-time.After(time.Until(expiry) + time.Millisecond)
			})
		}
	}
	alloc, err := a.Allocate(runtime.AllocateRequest{ClaimID: "cold-timeout", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: expiry}})
	entry := a.allocations["cold-timeout"]
	if err == nil || alloc.Identity.WorkerID != "" || k.networkCreates != 0 || k.delets != 0 || len(a.byWorker) != 0 || entry == nil || entry.state != allocRecoveryPending {
		t.Fatal("cold expiry published authority or claimed unknown cleanup", alloc, err)
	}
}

func TestColdPodPinExpiryDoesNotConfirmLingeringProcessCleanup(t *testing.T) {
	k := &networkKube{fakeKube: newFakeKube(), retainPod: true}
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	if err := a.ConfigureNetworkIsolation("strict"); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Second)
	var once sync.Once
	k.afterGet = func(resource string) {
		if resource == "pods" {
			once.Do(func() { <-time.After(time.Until(expiry) + time.Millisecond) })
		}
	}
	alloc, err := a.Allocate(runtime.AllocateRequest{ClaimID: "cold-pin-timeout", TemplateRef: "tmpl-v1", Network: runtime.NetworkAuthority{ExpiresAt: expiry}})
	entry := a.allocations["cold-pin-timeout"]
	if err == nil || alloc.Identity.WorkerID != "" || entry == nil || entry.state != allocRecoveryPending || !k.podPresent || k.networkCreates != 0 || len(k.claims) != 0 || len(k.sandboxes) != 0 {
		t.Fatal("expired unpinned process claimed released", alloc, err)
	}
	if len(a.byWorker) != 1 {
		t.Fatal("unconfirmed process lost its recovery reservation")
	}
	k.podPresent = false
	if err := a.recoverAttempt(entry); err != nil || len(a.allocations) != 0 || len(a.byWorker) != 0 {
		t.Fatal("confirmed later absence could not finish recovery", err)
	}
}
