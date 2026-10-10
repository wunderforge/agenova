// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platformapply

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type readinessDeployment struct {
	fakeDeployment
	readinessRuns        int
	readinessErr         error
	readinessCtx         context.Context
	readinessPlan        []Change
	preflightAtReadiness int
}

func (f *readinessDeployment) CheckReadiness(ctx context.Context, request DeploymentRequest) error {
	f.readinessRuns++
	f.readinessCtx = ctx
	f.readinessPlan = request.TargetChanges
	f.preflightAtReadiness = f.preflightRuns
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.readinessErr
}

func activateReadinessAdapters(t *testing.T, service Service) {
	t.Helper()
	for _, requirement := range testPlatform().Spec.Adapters {
		if _, err := service.Adapters.Install(requirement.ID + "@" + requirement.Version); err != nil {
			t.Fatal(err)
		}
	}
}

func activeReadinessAdapters(t *testing.T, service Service) int {
	t.Helper()
	lock, err := service.Adapters.List()
	if err != nil {
		t.Fatal(err)
	}
	return len(lock.Adapters)
}

func TestServiceApplyReadinessChecksBeforeAnyChangeAndOnUnchangedPlan(t *testing.T) {
	unavailable := errors.New("external prerequisite unavailable")
	for _, scenario := range []struct {
		name                        string
		targetReady, adaptersActive bool
	}{
		{"target mutation", false, false},
		{"activation only", true, false},
		{"unchanged", true, true},
	} {
		for _, readinessErr := range []error{nil, unavailable} {
			name := "available"
			if readinessErr != nil {
				name = "unavailable"
			}
			t.Run(scenario.name+"/"+name, func(t *testing.T) {
				deployment := &readinessDeployment{fakeDeployment: fakeDeployment{applied: scenario.targetReady}, readinessErr: readinessErr}
				service := newTestService(t, deployment)
				resolved, lock, err := service.Validate(testPlatform())
				if err != nil {
					t.Fatal(err)
				}
				if scenario.adaptersActive {
					activateReadinessAdapters(t, service)
				}
				before := activeReadinessAdapters(t, service)
				ctx := context.WithValue(context.Background(), struct{}{}, "caller-owned")
				result, err := service.Apply(ctx, resolved, lock)
				if deployment.readinessRuns != 1 || deployment.readinessCtx != ctx || deployment.readinessPlan == nil {
					t.Fatal("Apply did not check the confirmed target plan with caller context")
				}
				wantPreflight := 0
				wantPlan := []Change{}
				if !scenario.targetReady {
					wantPreflight = 1
					wantPlan = []Change{{Component: "control-plane", Action: "create", Detail: "test"}}
				}
				if deployment.preflightAtReadiness != wantPreflight || !reflect.DeepEqual(deployment.readinessPlan, wantPlan) {
					t.Fatal("readiness ran before required preflight or without exact target changes")
				}
				if readinessErr != nil {
					if !errors.Is(err, unavailable) || result.Applied || result.Ready || deployment.applyRuns != 0 || activeReadinessAdapters(t, service) != before {
						t.Fatalf("failed readiness changed state: error=%v applied=%v ready=%v targetApply=%d", err, result.Applied, result.Ready, deployment.applyRuns)
					}
					return
				}
				wantApply := 0
				if !scenario.targetReady {
					wantApply = 1
				}
				if err != nil || !result.Ready || result.Applied != (!scenario.targetReady || !scenario.adaptersActive) || deployment.applyRuns != wantApply || activeReadinessAdapters(t, service) != 3 {
					t.Fatalf("successful readiness failed reconciliation: error=%v applied=%v ready=%v targetApply=%d", err, result.Applied, result.Ready, deployment.applyRuns)
				}
			})
		}
	}
}

func TestServiceReadinessDoesNotRunDuringValidatePlanOrStatus(t *testing.T) {
	deployment := &readinessDeployment{fakeDeployment: fakeDeployment{applied: true}, readinessErr: errors.New("unavailable")}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Plan(context.Background(), resolved, lock); err != nil {
		t.Fatal(err)
	}
	service.State, err = NewFileState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Remember(resolved, lock); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deployment.readinessRuns != 0 || deployment.applyRuns != 0 || activeReadinessAdapters(t, service) != 0 {
		t.Fatal("metadata-only operation checked prerequisites or changed target/activation")
	}
}

func TestServiceTargetPreflightDenialPreventsReadinessAndActivation(t *testing.T) {
	deployment := &readinessDeployment{fakeDeployment: fakeDeployment{denied: errors.New("target authorization denied")}}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), resolved, lock)
	if err == nil || !strings.Contains(err.Error(), "target authorization denied") || result.Applied || deployment.readinessRuns != 0 || deployment.applyRuns != 0 || activeReadinessAdapters(t, service) != 0 {
		t.Fatalf("authorization denial reached readiness or changes: %v", err)
	}
}

func TestServiceApplyPlannedDriftPreventsReadinessAndActivation(t *testing.T) {
	deployment := &readinessDeployment{}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.Plan(context.Background(), resolved, lock)
	if err != nil {
		t.Fatal(err)
	}
	deployment.applied = true
	result, err := service.ApplyPlanned(context.Background(), resolved, lock, confirmed)
	if err == nil || !strings.Contains(err.Error(), "plan changed") || result.Applied || deployment.readinessRuns != 0 || deployment.applyRuns != 0 || activeReadinessAdapters(t, service) != 0 {
		t.Fatalf("unconfirmed target drift reached readiness or changes: %v", err)
	}
}

func TestServiceApplyPlannedUnchangedChecksReadiness(t *testing.T) {
	deployment := &readinessDeployment{fakeDeployment: fakeDeployment{applied: true}}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	activateReadinessAdapters(t, service)
	confirmed, err := service.Plan(context.Background(), resolved, lock)
	if err != nil || confirmed.Changed() {
		t.Fatalf("setup failed: %v", err)
	}
	result, err := service.ApplyPlanned(context.Background(), resolved, lock, confirmed)
	if err != nil || !result.Ready || result.Applied || deployment.readinessRuns != 1 || deployment.applyRuns != 0 {
		t.Fatalf("unchanged confirmed apply did not check readiness: %v", err)
	}
	unavailable := errors.New("external prerequisite unavailable")
	deployment.readinessErr = unavailable
	result, err = service.ApplyPlanned(context.Background(), resolved, lock, confirmed)
	if !errors.Is(err, unavailable) || result.Ready || result.Applied || deployment.readinessRuns != 2 || deployment.applyRuns != 0 {
		t.Fatalf("unchanged confirmed apply ignored failed readiness: %v", err)
	}
}

func TestServiceReadinessPreservesCallerTerminationBeforeActivation(t *testing.T) {
	deployment := &readinessDeployment{fakeDeployment: fakeDeployment{applied: true}}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := service.Apply(ctx, resolved, lock)
	if !errors.Is(err, context.Canceled) || result.Ready || result.Applied || deployment.readinessRuns != 1 || deployment.readinessCtx != ctx || deployment.applyRuns != 0 || activeReadinessAdapters(t, service) != 0 {
		t.Fatalf("caller termination ignored before activation: %v", err)
	}
}
