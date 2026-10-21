// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/runtime"
)

func TestMemoryNetworkCancellationDoesNotGrantAndStillRevokes(t *testing.T) {
	r := newConfiguredRuntime(t)
	alloc, err := r.Allocate(runtime.AllocateRequest{ClaimID: "canceled-network", TemplateRef: testTemplate, Network: runtime.NetworkAuthority{Profiles: []string{"public-web"}, ExpiresAt: time.Now().Add(time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evidence, err := r.ActivateNetwork(ctx, alloc.Identity)
	if !errors.Is(err, context.Canceled) || evidence.State != "" {
		t.Fatalf("canceled reference activation %+v %v", evidence, err)
	}
	a, err := r.allocationFor(alloc.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if a.networkActive || !a.terminated {
		t.Fatal("reference cancellation left active worker/authority")
	}
	evidence, err = r.RevokeNetwork(alloc.Identity)
	if err != nil || evidence.State != "Revoked" || evidence.EvidenceLevel != "Simulated" {
		t.Fatalf("independent reference cleanup %+v %v", evidence, err)
	}
}
