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

func TestKubernetesPlanRejectsPolicyOutsideReferenceCatalog(t *testing.T) {
	request := deploymentRequest()
	request.Platform.InitialPolicyRef.Version = "2"
	runner := &fakeKubectl{}
	_, _, _, err := newKubernetesDeployment(runner).Plan(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "reference install provides") || len(runner.calls) != 0 {
		t.Fatalf("Plan() = %v, calls %#v; want policy rejection before target access", err, runner.calls)
	}
}

func TestKubernetesApplyMutatesOnlyChangedPolicyRecord(t *testing.T) {
	request := deploymentRequest()
	policyApplied := false
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		switch {
		case contains(args, "version"):
			return commandResult{stdout: `{}`}, nil
		case contains(args, "can-i") && contains(args, "watch"):
			return commandResult{stdout: "no\n"}, nil
		case contains(args, "can-i"):
			return commandResult{stdout: "yes\n"}, nil
		case contains(args, "apply"):
			policyApplied = true
			return commandResult{}, nil
		case contains(args, "rollout"):
			return commandResult{}, nil
		case contains(args, "get") && contains(args, "json"):
			result, ok := readyResourceResult(args, request)
			if !ok {
				return commandResult{}, errors.New("unexpected resource")
			}
			if contains(args, policyRecord) && !policyApplied {
				var object map[string]any
				_ = json.Unmarshal([]byte(result.stdout), &object)
				object["data"].(map[string]any)["policy.json"] = "stale"
				data, _ := json.Marshal(object)
				result.stdout = string(data)
			}
			return result, nil
		case contains(args, "get"):
			return commandResult{stdout: "found"}, nil
		}
		return commandResult{}, nil
	}}
	adapter := newKubernetesDeployment(runner)
	_, changes, _, err := adapter.Plan(context.Background(), request)
	if err != nil || len(changes) != 1 || changes[0].Component != policyRecord {
		t.Fatalf("Plan() changes = %#v, %v", changes, err)
	}
	request.TargetChanges = changes
	statuses, attempted, err := adapter.Apply(context.Background(), request)
	if err != nil || !attempted || len(statuses) != 5 {
		t.Fatalf("Apply() = %#v, %t, %v", statuses, attempted, err)
	}
	applyCount := 0
	for index, call := range runner.calls {
		if contains(call, "rollout") || (contains(call, "can-i") && contains(call, "watch")) {
			t.Fatalf("policy-only plan required Deployment rollout authority: %#v", call)
		}
		if contains(call, "apply") {
			applyCount++
			if !strings.Contains(string(runner.inputs[index]), "name: "+policyRecord) {
				t.Fatalf("unexpected resource mutation: %s", runner.inputs[index])
			}
		}
		if contains(call, "can-i") && (contains(call, "create") || contains(call, "patch")) && !contains(call, "configmaps") {
			t.Fatalf("unplanned write RBAC requested: %#v", call)
		}
	}
	if applyCount != 1 {
		t.Fatalf("applied %d resources, want only policy record", applyCount)
	}
}

func TestKubernetesApplyRejectsStaleConfirmedTargetPlan(t *testing.T) {
	request := deploymentRequest()
	request.TargetChanges = []platformapply.Change{{Component: policyRecord, Action: "reconcile"}}
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		if result, ok := readyResourceResult(args, request); ok {
			return result, nil
		}
		return commandResult{}, nil
	}}
	_, attempted, err := newKubernetesDeployment(runner).Apply(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "target changed before apply") || attempted {
		t.Fatalf("Apply() = attempted %t, error %v; want fresh confirmation", attempted, err)
	}
	for _, call := range runner.calls {
		if contains(call, "apply") || contains(call, "patch") {
			t.Fatalf("stale plan mutated target: %#v", call)
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
	statuses, attempted, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "lacks required RBAC") || len(statuses) != 4 {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	if attempted {
		t.Fatal("denied preflight reported a target mutation")
	}
	for i, call := range runner.calls {
		if contains(call, "apply") {
			t.Fatalf("call %d mutated target after denied preflight: %#v", i, call)
		}
	}
}

func TestKubernetesApplyUsesSecretFreeResourceStepsAndWaitsReady(t *testing.T) {
	request := deploymentRequest()
	rolledOut := false
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "rollout") {
			rolledOut = true
			return commandResult{}, nil
		}
		if rolledOut {
			if result, ok := readyResourceResult(args, request); ok {
				return result, nil
			}
		}
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		if contains(args, "get") {
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
		return commandResult{}, nil
	}}
	statuses, _, err := newKubernetesDeployment(runner).Apply(context.Background(), request)
	if err != nil || len(statuses) != 5 {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	applyCount, rollout := 0, false
	var manifest strings.Builder
	for index, call := range runner.calls {
		if contains(call, "apply") {
			applyCount++
			manifest.Write(runner.inputs[index])
		}
		if contains(call, "rollout") {
			rollout = true
		}
	}
	if applyCount != 5 || !rollout {
		t.Fatalf("calls = %#v, want five resource applies then rollout", runner.calls)
	}
	manifestText := manifest.String()
	for _, required := range []string{"kind: Namespace", "kind: ConfigMap", "kind: Deployment", "kind: Service", "reference-default-deny"} {
		if !strings.Contains(manifestText, required) {
			t.Fatalf("manifest missing %q\n%s", required, manifestText)
		}
	}
	if strings.Contains(strings.ToLower(manifestText), "apikey") || strings.Contains(strings.ToLower(manifestText), "password") {
		t.Fatalf("manifest contains credential-shaped field: %s", manifestText)
	}
	if !strings.Contains(manifestText, "team-a") || !strings.Contains(manifestText, "claim.create") {
		t.Fatalf("initial policy omitted the actual reference allow rule: %s", manifestText)
	}
}

func TestKubernetesApplyRechecksTargetAfterRollout(t *testing.T) {
	request := deploymentRequest()
	rolledOut := false
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		switch {
		case contains(args, "can-i"):
			return commandResult{stdout: "yes\n"}, nil
		case contains(args, "rollout"):
			rolledOut = true
			return commandResult{}, nil
		case contains(args, "version"):
			return commandResult{stdout: `{}`}, nil
		case rolledOut:
			if result, ok := readyResourceResult(args, request); ok {
				if contains(args, "service") {
					service := serviceObject("agenova-system")
					service["spec"].(map[string]any)["selector"].(map[string]any)["unexpected"] = "raced"
					data, _ := json.Marshal(service)
					return commandResult{stdout: string(data)}, nil
				}
				return result, nil
			}
		case contains(args, "get"):
			return commandResult{stderr: "NotFound"}, errors.New("exit 1")
		}
		return commandResult{}, nil
	}}
	statuses, _, err := newKubernetesDeployment(runner).Apply(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "not reconciled after rollout") {
		t.Fatalf("Apply() = %#v, %v, want final drift detection", statuses, err)
	}
	for _, status := range statuses {
		if status.Name == controlPlaneName+"-service" && status.State != "unavailable" {
			t.Fatalf("raced Service status = %q", status.State)
		}
	}
}

func TestKubernetesApplyDoesNotRelabelExistingNamespace(t *testing.T) {
	request := deploymentRequest()
	rolledOut := false
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "rollout") {
			rolledOut = true
			return commandResult{}, nil
		}
		if rolledOut {
			if result, ok := readyResourceResult(args, request); ok {
				return result, nil
			}
		}
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		if contains(args, "get") && contains(args, "namespaces") {
			if contains(args, "json") {
				return commandResult{stdout: `{"metadata":{"name":"agenova-system"},"status":{"phase":"Active"}}`}, nil
			}
			return commandResult{stdout: "namespace/agenova-system\n"}, nil
		}
		if contains(args, "get") {
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
		return commandResult{}, nil
	}}
	if _, _, err := newKubernetesDeployment(runner).Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	applyCount := 0
	for index, call := range runner.calls {
		if contains(call, "can-i") && contains(call, "patch") && contains(call, "namespaces") {
			t.Fatalf("existing namespace patch permission was requested: %#v", call)
		}
		if contains(call, "apply") {
			applyCount++
			if strings.Contains(string(runner.inputs[index]), "kind: Namespace") {
				t.Fatal("existing namespace was silently relabeled")
			}
		}
	}
	if applyCount != 4 {
		t.Fatalf("got %d resource applies, want four namespaced resources", applyCount)
	}
}

func TestKubernetesRejectsTerminatingNamespace(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		if contains(args, "get") && (contains(args, "namespace") || contains(args, "namespaces")) {
			if contains(args, "json") {
				return commandResult{stdout: `{"metadata":{"name":"agenova-system","deletionTimestamp":"2026-09-16T00:00:00Z"},"status":{"phase":"Terminating"}}`}, nil
			}
			return commandResult{stdout: "namespace/agenova-system\n"}, nil
		}
		return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
	}}
	adapter := newKubernetesDeployment(runner)
	_, _, _, err := adapter.Plan(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "is terminating") {
		t.Fatalf("Plan() error = %v", err)
	}
	if err := adapter.Preflight(context.Background(), deploymentRequest()); err == nil || !strings.Contains(err.Error(), "is terminating") {
		t.Fatalf("Preflight() error = %v", err)
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
			return `{"metadata":{"name":"agenova-control-plane","labels":{"app.kubernetes.io/managed-by":"agenova"}},"spec":{"type":"ClusterIP","selector":{"app.kubernetes.io/name":"agenova-control-plane","app.kubernetes.io/managed-by":"agenova"},"ports":[{"name":"http","port":8080,"targetPort":"http"}]}}`
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
	environment[2].(map[string]any)["value"] = "reference-default-deny@1"
	pod := actual["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	pod["dnsPolicy"] = "ClusterFirst"
	pod["restartPolicy"] = "Always"
	container["resources"] = map[string]any{}
	actual["spec"].(map[string]any)["progressDeadlineSeconds"] = float64(600)
	actual["spec"].(map[string]any)["revisionHistoryLimit"] = float64(10)
	if !deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("Kubernetes server defaults should not create drift")
	}
	actual["spec"].(map[string]any)["paused"] = true
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("unexpected Deployment-level pause must be reconciled")
	}
	delete(actual["spec"].(map[string]any), "paused")
	pod["hostNetwork"] = true
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("unexpected hostNetwork must be reconciled")
	}
	delete(pod, "hostNetwork")
	container["command"] = []any{"sleep", "infinity"}
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("unexpected container command must be reconciled")
	}
	delete(container, "command")
	probe := container["readinessProbe"].(map[string]any)
	probe["timeoutSeconds"] = float64(30)
	if deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("unexpected nested probe timeout must be reconciled")
	}
	probe["timeoutSeconds"] = float64(1)
	if !deploymentMatches(actual, desired, request.Platform.Revision) {
		t.Fatal("normal Kubernetes probe default must be accepted")
	}
}

func TestKubernetesPlanLabelsExistingServiceDriftAsReconcile(t *testing.T) {
	service := serviceObject("agenova-system")
	service["spec"].(map[string]any)["selector"].(map[string]any)["unexpected"] = "extra"
	data, _ := json.Marshal(service)
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		if contains(args, "service") {
			return commandResult{stdout: string(data)}, nil
		}
		return commandResult{stderr: "NotFound"}, errors.New("exit 1")
	}}
	_, changes, _, err := newKubernetesDeployment(runner).Plan(context.Background(), deploymentRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Component == controlPlaneName+"-service" {
			if change.Action != "reconcile" {
				t.Fatalf("existing Service drift action = %q", change.Action)
			}
			return
		}
	}
	t.Fatal("drifted existing Service missing from plan")
}

func TestKubernetesPlanExistingPlatformRecordWithoutRevisionIsUpdate(t *testing.T) {
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: `{}`}, nil
		}
		if contains(args, "get") && contains(args, platformRecord) {
			return commandResult{stdout: `{"metadata":{"name":"agenova-platform","labels":{"app.kubernetes.io/managed-by":"agenova"}},"data":{}}`}, nil
		}
		return commandResult{stderr: "NotFound"}, errors.New("exit 1")
	}}
	_, changes, _, err := newKubernetesDeployment(runner).Plan(context.Background(), deploymentRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Component == platformRecord {
			if change.Action != "update" {
				t.Fatalf("existing Platform record action = %q", change.Action)
			}
			return
		}
	}
	t.Fatal("drifted existing Platform record missing from plan")
}

func TestServiceMatchesFullManagedSelector(t *testing.T) {
	desired := serviceObject("agenova-system")
	data, err := json.Marshal(desired)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	if !serviceMatches(actual, desired) {
		t.Fatal("matching Service should be ready")
	}
	selector := actual["spec"].(map[string]any)["selector"].(map[string]any)
	selector["app.kubernetes.io/managed-by"] = "someone-else"
	if serviceMatches(actual, desired) {
		t.Fatal("changed managed selector must be reconciled")
	}
	selector["app.kubernetes.io/managed-by"] = "agenova"
	selector["unexpected"] = "extra"
	if serviceMatches(actual, desired) {
		t.Fatal("extra selector must be reconciled")
	}
}

func TestKubernetesApplyReplacesExtraServiceSelector(t *testing.T) {
	request := deploymentRequest()
	rolledOut := false
	deployment := deploymentObject(request, "agenova-system")
	deployment["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["hostNetwork"] = true
	deploymentData, _ := json.Marshal(deployment)
	service := serviceObject("agenova-system")
	service["spec"].(map[string]any)["selector"].(map[string]any)["unexpected"] = "extra"
	serviceData, err := json.Marshal(service)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "rollout") {
			rolledOut = true
			return commandResult{}, nil
		}
		if rolledOut {
			if result, ok := readyResourceResult(args, request); ok {
				return result, nil
			}
		}
		if contains(args, "can-i") {
			return commandResult{stdout: "yes\n"}, nil
		}
		if contains(args, "get") && contains(args, "services") {
			if contains(args, "json") {
				return commandResult{stdout: string(serviceData)}, nil
			}
			return commandResult{stdout: "service/agenova-control-plane\n"}, nil
		}
		if contains(args, "get") && contains(args, "service") {
			return commandResult{stdout: string(serviceData)}, nil
		}
		if contains(args, "get") && (contains(args, "deployment") || contains(args, "deployments.apps")) {
			return commandResult{stdout: string(deploymentData)}, nil
		}
		if contains(args, "get") {
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
		return commandResult{}, nil
	}}
	if _, _, err := newKubernetesDeployment(runner).Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	patchedSelector, patchedPod := false, false
	for _, args := range runner.calls {
		if !contains(args, "patch") || !contains(args, "-p") {
			continue
		}
		payload := args[len(args)-1]
		if strings.Contains(payload, `"path":"/spec/selector"`) {
			patchedSelector = true
			if strings.Contains(payload, "unexpected") {
				t.Fatalf("selector patch retains extra field: %q", payload)
			}
		}
		if strings.Contains(payload, `"path":"/spec"`) {
			patchedPod = true
			if strings.Contains(payload, "hostNetwork") {
				t.Fatalf("Deployment spec patch retains extra field: %q", payload)
			}
		}
	}
	if !patchedSelector || !patchedPod {
		t.Fatalf("expected selector and Deployment spec replacement; selector=%v deployment=%v", patchedSelector, patchedPod)
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
	_, attempted, err := newKubernetesDeployment(runner).Apply(context.Background(), deploymentRequest())
	if err == nil || !strings.Contains(err.Error(), "watch deployments.apps") {
		t.Fatalf("Apply() error = %v, want missing rollout watch", err)
	}
	if attempted {
		t.Fatal("denied rollout watch reported a target mutation")
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
	policyData, err := referencePolicyJSON()
	if err != nil {
		t.Fatal(err)
	}
	partial := false
	applyCount := 0
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		switch {
		case contains(args, "can-i"):
			return commandResult{stdout: "yes\n"}, nil
		case contains(args, "version"):
			return commandResult{stdout: `{}`}, nil
		case contains(args, "apply"):
			applyCount++
			if applyCount == 4 {
				partial = true
				return commandResult{stderr: "deployment rejected"}, errors.New("exit 1")
			}
			return commandResult{}, nil
		case contains(args, "get") && partial && contains(args, "namespace"):
			return commandResult{stdout: `{"metadata":{"name":"agenova-system"}}`}, nil
		case contains(args, "get") && partial && contains(args, platformRecord):
			data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}})
			return commandResult{stdout: string(data)}, nil
		case contains(args, "get") && partial && contains(args, policyRecord):
			data, _ := json.Marshal(map[string]any{"metadata": map[string]any{"labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyData)}})
			return commandResult{stdout: string(data)}, nil
		default:
			return commandResult{stderr: "Error from server (NotFound): resource not found"}, errors.New("exit 1")
		}
	}}
	statuses, attempted, err := newKubernetesDeployment(runner).Apply(context.Background(), request)
	if err == nil || !partial {
		t.Fatalf("Apply() = %#v, %v", statuses, err)
	}
	if !attempted {
		t.Fatal("partial target mutation was not reported")
	}
	states := map[string]string{}
	for _, status := range statuses {
		states[status.Name] = status.State
	}
	if states["agenova-system"] != "available" || states[platformRecord] != "configured" || states[policyRecord] != "configured" || states[controlPlaneName] != "failed" || states[controlPlaneName+"-service"] != "pending" {
		t.Fatalf("partial state = %#v", statuses)
	}
}

func readyResourceResult(args []string, request platformapply.DeploymentRequest) (commandResult, bool) {
	if !contains(args, "get") || !contains(args, "json") {
		return commandResult{}, false
	}
	var object map[string]any
	switch {
	case contains(args, "namespace"):
		object = map[string]any{"metadata": map[string]any{"name": "agenova-system"}}
	case contains(args, platformRecord):
		lockJSON, _ := platformapply.EncodeLock(request.Lock)
		object = map[string]any{"metadata": map[string]any{"name": platformRecord, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}}
	case contains(args, policyRecord):
		policyJSON, _ := referencePolicyJSON()
		object = map[string]any{"metadata": map[string]any{"name": policyRecord, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyJSON)}}
	case contains(args, "deployment"):
		object = deploymentObject(request, "agenova-system")
		object["metadata"].(map[string]any)["generation"] = 1
		object["status"] = map[string]any{"observedGeneration": 1, "replicas": 1, "updatedReplicas": 1, "availableReplicas": 1}
	case contains(args, "service"):
		object = serviceObject("agenova-system")
	default:
		return commandResult{}, false
	}
	data, _ := json.Marshal(object)
	return commandResult{stdout: string(data)}, true
}

func deploymentRequest() platformapply.DeploymentRequest {
	resolved := &platform.ResolvedPlatform{PlatformName: "reference", Revision: "sha256:test", InitialPolicyRef: v1alpha1.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"}}
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
