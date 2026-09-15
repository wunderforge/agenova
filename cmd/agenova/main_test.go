// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestSubmitClaimRequestPreservesTerminalReportOnBackendFailure(t *testing.T) {
	t.Setenv("AGENOVA_LOCAL_PRINCIPAL", "team-a")
	path := filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
	report, err := submitClaimRequest(path, allocationFailureBackend{})
	if err == nil {
		t.Fatal("backend failure was not returned")
	}
	if report.RequestRef != "fix-payment-timeout" || report.Decision != "Allow" || report.ClaimID == "" || report.Phase != "Failed" || report.Allocated {
		t.Fatalf("terminal report = %+v", report)
	}
}

type allocationFailureBackend struct{}

func (allocationFailureBackend) Allocate(runtime.AllocateRequest) (runtime.Allocation, error) {
	return runtime.Allocation{}, errors.New("backend unavailable")
}

func (allocationFailureBackend) Observe(v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{}, errors.New("unexpected Observe")
}

func (allocationFailureBackend) Start(v1alpha1.SandboxClaimBackendIdentity) error {
	return errors.New("unexpected Start")
}

func (allocationFailureBackend) Terminate(v1alpha1.SandboxClaimBackendIdentity) error {
	return errors.New("unexpected Terminate")
}

func (allocationFailureBackend) Cleanup(v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return runtime.CleanupResult{}, errors.New("unexpected Cleanup")
}
