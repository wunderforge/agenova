// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
)

type fakeKubectl struct {
	calls  [][]string
	inputs [][]byte
	run    func([]string) (commandResult, error)
}

func (f *fakeKubectl) Run(_ context.Context, input []byte, args ...string) (commandResult, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	f.inputs = append(f.inputs, append([]byte(nil), input...))
	if f.run != nil {
		return f.run(args)
	}
	return commandResult{}, nil
}

func TestKubernetesPlanIsReadOnlyAndReportsMissingTarget(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "cluster-info") {
			return commandResult{stdout: "ok"}, nil
		}
		return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
	}}
	adapter := newKubernetesDeployment(runner)
	target, changes, statuses, err := adapter.Plan(context.Background(), deploymentRequest())
	if err != nil || target != "kind-agenova/agenova-system" || len(changes) != 4 || len(statuses) != 4 {
		t.Fatalf("Plan() = %q %#v %#v, %v", target, changes, statuses, err)
	}
	for _, call := range runner.calls {
		if contains(call, "apply") || contains(call, "create") || contains(call, "patch") {
			t.Fatalf("read-only Plan issued mutating call: %#v", call)
		}
	}
}

func TestKubernetesApplyDeniesBeforeMutationWhenRBACMissing(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "can-i") {
			return commandResult{stdout: "no\n"}, nil
		}
		return commandResult{}, nil
	}}
	statuses, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "lacks required RBAC") || len(statuses) != 4 {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	for i, call := range runner.calls {
		if contains(call, "apply") {
			t.Fatalf("call %d mutated target after denied preflight: %#v", i, call)
		}
	}
}

func TestKubernetesApplyUsesOneSecretFreeManifestAndWaitsReady(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		return commandResult{}, nil
	}}
	statuses, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err != nil || len(statuses) != 4 {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	applyIndex, rollout := -1, false
	for index, call := range runner.calls {
		if contains(call, "apply") {
			applyIndex = index
		}
		if contains(call, "rollout") {
			rollout = true
		}
	}
	if applyIndex < 0 || !rollout {
		t.Fatalf("calls = %#v, want apply then rollout", runner.calls)
	}
	manifest := string(runner.inputs[applyIndex])
	for _, required := range []string{"kind: Namespace", "kind: ConfigMap", "kind: Deployment", "kind: Service", "reference-default-deny"} {
		if !strings.Contains(manifest, required) {
			t.Fatalf("manifest missing %q\n%s", required, manifest)
		}
	}
	if strings.Contains(strings.ToLower(manifest), "apikey") || strings.Contains(strings.ToLower(manifest), "password") {
		t.Fatalf("manifest contains credential-shaped field: %s", manifest)
	}
}

func deploymentRequest() platformapply.DeploymentRequest {
	resolved := &platform.ResolvedPlatform{PlatformName: "reference", Revision: "sha256:test", InitialPolicyRef: v1alpha1.PlatformPolicyReference{ID: platformapply.ReferencePolicyID, Version: platformapply.ReferencePolicyVersion}}
	lock := &platform.PlatformLock{PlatformName: resolved.PlatformName, Revision: resolved.Revision, InitialPolicyRef: resolved.InitialPolicyRef}
	return platformapply.DeploymentRequest{Platform: resolved, Lock: lock, Config: map[string]any{"context": "kind-agenova", "namespace": "agenova-system"}}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
