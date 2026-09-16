// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
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
		if contains(args, "version") {
			return commandResult{stdout: "ok"}, nil
		}
		return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
	}}
	adapter := newKubernetesDeployment(runner)
	target, changes, statuses, err := adapter.Plan(context.Background(), deploymentRequest())
	if err != nil || target != "kind-agenova/agenova-system" || len(changes) != 5 || len(statuses) != 5 {
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
		if contains(args, "get") {
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
		return commandResult{}, nil
	}}
	statuses, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err != nil || len(statuses) != 5 {
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
	if !strings.Contains(manifest, "team-a") || !strings.Contains(manifest, "claim.create") {
		t.Fatalf("initial policy omitted the actual reference allow rule: %s", manifest)
	}
}

func TestKubernetesPlanDetectsRevisionPreservingDrift(t *testing.T) {
	request := deploymentRequest()
	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		t.Fatal(err)
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		t.Fatal(err)
	}
	object := func(args []string) string {
		switch {
		case contains(args, "namespace"):
			return `{"metadata":{"name":"agenova-system"}}`
		case contains(args, "configmap") && contains(args, platformRecord):
			data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}})
			return string(data)
		case contains(args, "configmap") && contains(args, policyRecord):
			data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyData)}})
			return string(data)
		case contains(args, "deployment"):
			return `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"agenova"},"annotations":{"agenova.io/platform-revision":"sha256:test"}},"spec":{"replicas":1,"template":{"metadata":{"annotations":{"agenova.io/platform-revision":"sha256:test"}},"spec":{"containers":[{"image":"tampered:latest"}]}}},"status":{"availableReplicas":1}}`
		case contains(args, "service"):
			return `{"metadata":{"name":"agenova-control-plane","labels":{"app.kubernetes.io/managed-by":"agenova"}},"spec":{"type":"ClusterIP","selector":{"app.kubernetes.io/name":"agenova-control-plane"},"ports":[{"port":8080,"targetPort":"http"}]}}`
		default:
			return `{}`
		}
	}
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		return commandResult{stdout: object(args)}, nil
	}}
	_, changes, _, err := newKubernetesDeployment(runner).Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Component != controlPlaneName {
		t.Fatalf("drift plan = %#v, want only deployment reconcile", changes)
	}
}

func TestKubernetesPlanRejectsUnmanagedNameCollision(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		if contains(args, platformRecord) {
			return commandResult{stdout: `{"metadata":{"name":"agenova-platform"}}`}, nil
		}
		return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
	}}
	_, _, _, err := newKubernetesDeployment(runner).Plan(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "not managed by Agenova") {
		t.Fatalf("Plan() error = %v, want unmanaged resource collision", err)
	}
}

func TestKubernetesPreflightRejectsUnmanagedNameCollision(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		if contains(args, "get") && contains(args, platformRecord) {
			if contains(args, "json") {
				return commandResult{stdout: `{"metadata":{"name":"agenova-platform"}}`}, nil
			}
			return commandResult{stdout: "configmap/agenova-platform\n"}, nil
		}
		return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
	}}
	err := newKubernetesDeployment(runner).Preflight(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "not managed by Agenova") {
		t.Fatalf("Preflight() error = %v, want unmanaged resource collision", err)
	}
	for _, call := range runner.calls {
		if contains(call, "apply") {
			t.Fatalf("preflight mutated target: %#v", call)
		}
	}
}

func TestDeploymentMatchesCurrentRolloutAndOwnedSpec(t *testing.T) {
	request := deploymentRequest()
	desired := deploymentObject(request, "agenova-system")
	data, err := json.Marshal(desired)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	metadata := actual["metadata"].(map[string]any)
	metadata["generation"] = float64(2)
	actual["status"] = map[string]any{"observedGeneration": float64(1), "replicas": float64(2), "updatedReplicas": float64(1), "availableReplicas": float64(1)}
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("old available ReplicaSet must not make a new rollout Ready")
	}
	actual["status"] = map[string]any{"observedGeneration": float64(2), "replicas": float64(1), "updatedReplicas": float64(1), "availableReplicas": float64(1)}
	if !deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("matching current rollout should be Ready")
	}
	container := actual["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)
	environment := container["env"].([]any)
	environment[2].(map[string]any)["value"] = "unexpected-policy@2"
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("same-revision managed environment drift must be reconciled")
	}
}

func TestKubernetesErrorsAreActionableWithoutForwardingStderr(t *testing.T) {
	missing := &fakeKubectl{run: func([]string) (commandResult, error) {
		return commandResult{}, exec.ErrNotFound
	}}
	_, _, _, err := newKubernetesDeployment(missing).Plan(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "install kubectl") {
		t.Fatalf("missing kubectl error = %v", err)
	}
	const marker = "sensitive-provider-output"
	failed := &fakeKubectl{run: func([]string) (commandResult, error) {
		return commandResult{stderr: marker}, errors.New("exit 1")
	}}
	_, _, _, err = newKubernetesDeployment(failed).Plan(context.Background(), deploymentRequest())
	if err == nil || strings.Contains(err.Error(), marker) {
		t.Fatalf("untrusted subprocess stderr leaked: %v", err)
	}
}

func TestKubernetesPreflightRequiresRolloutWatchBeforeMutation(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "can-i") && contains(args, "watch") {
			return commandResult{stdout: "no\n"}, nil
		}
		return commandResult{stdout: "yes\n"}, nil
	}}
	_, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "watch deployments.apps") {
		t.Fatalf("Apply() error = %v, want missing rollout watch", err)
	}
	for _, call := range runner.calls {
		if contains(call, "apply") {
			t.Fatalf("denied rollout watch mutated target: %#v", call)
		}
	}
}

func TestKubernetesApplyReportsObservedPartialState(t *testing.T) {
	request := deploymentRequest()
	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		t.Fatal(err)
	}
	partial := false
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		switch {
		case contains(args, "can-i"):
			return commandResult{stdout: "yes\n"}, nil
		case contains(args, "version"):
			return commandResult{stdout: `{}`}, nil
		case contains(args, "apply"):
			partial = true
			return commandResult{stderr: "deployment rejected"}, errors.New("exit 1")
		case contains(args, "get") && partial && contains(args, "namespace"):
			return commandResult{stdout: `{"metadata":{"name":"agenova-system"}}`}, nil
		case contains(args, "get") && partial && contains(args, platformRecord):
			data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}})
			return commandResult{stdout: string(data)}, nil
		default:
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
	}}
	statuses, err := newKubernetesDeployment(runner).Apply(context.Background(), request)
	if err == nil || !partial {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	states := map[string]string{}
	for _, status := range statuses {
		states[status.Name] = status.State
	}
	if states["agenova-system"] != "available" || states[platformRecord] != "configured" || states["reconciliation"] != "failed" || states[controlPlaneName] != "unavailable" {
		t.Fatalf("partial state = %#v", statuses)
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
