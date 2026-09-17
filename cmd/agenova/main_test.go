// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"path/filepath"
	"strings"
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

func TestCustomAPIConnectPortAdvertisesMatchingViteTarget(t *testing.T) {
	defaultMessage := apiConnectInstruction(8088)
	if strings.Contains(defaultMessage, "AGENOVA_API_URL") {
		t.Fatalf("default port should not require extra Vite configuration: %q", defaultMessage)
	}
	custom := apiConnectInstruction(18081)
	if !strings.Contains(custom, "$env:AGENOVA_API_URL = 'http://127.0.0.1:18081'") ||
		!strings.Contains(custom, "AGENOVA_API_URL=http://127.0.0.1:18081 npm --prefix ui run dev") {
		t.Fatalf("custom port did not advertise the matching Vite target: %q", custom)
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
