// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strings"

	"github.com/wunderforge/agenova/internal/platformapply"
	"github.com/wunderforge/agenova/internal/policy"
	"gopkg.in/yaml.v3"
)

const (
	controlPlaneName  = "agenova-control-plane"
	platformRecord    = "agenova-platform"
	policyRecord      = "agenova-policy-reference-default-deny-v1"
	controlPlaneImage = "agenova-control-plane:0.1.0"
)

var errKubectlUnavailable = errors.New("kubectl executable is unavailable; install kubectl and add it to PATH")

type commandResult struct {
	stdout string
	stderr string
}

type kubectlRunner interface {
	Run(context.Context, []byte, ...string) (commandResult, error)
}

type processKubectl struct{}

func (processKubectl) Run(ctx context.Context, input []byte, args ...string) (commandResult, error) {
	command := exec.CommandContext(ctx, "kubectl", args...)
	if len(input) > 0 {
		command.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String()}, err
}

func newKubernetesDeployment(runner kubectlRunner) *KubernetesDeployment {
	if runner == nil {
		runner = processKubectl{}
	}
	return &KubernetesDeployment{runner: runner}
}

func (k *KubernetesDeployment) Plan(ctx context.Context, request platformapply.DeploymentRequest) (string, []platformapply.Change, []platformapply.ComponentStatus, error) {
	contextName, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return "", nil, nil, err
	}
	target := platformapply.SafeTarget(contextName, namespace)
	if _, err := k.run(ctx, nil, "--context", contextName, "version", "--request-timeout=5s", "-o", "json"); errors.Is(err, errKubectlUnavailable) {
		return target, nil, nil, err
	} else if err != nil {
		return target, nil, nil, fmt.Errorf("selected Kubernetes context %q is unavailable", contextName)
	}
	namespaceObject, namespaceErr := k.getJSON(ctx, contextName, "", "namespace", namespace)
	if namespaceErr != nil && !isNotFound(namespaceErr) {
		return target, nil, nil, namespaceErr
	}
	if namespaceTerminating(namespaceObject) {
		return target, nil, nil, fmt.Errorf("selected Kubernetes namespace %q is terminating; wait for deletion or choose another namespace", namespace)
	}

	record, recordErr := k.getJSON(ctx, contextName, namespace, "configmap", platformRecord)
	if recordErr != nil && !isNotFound(recordErr) {
		return target, nil, nil, recordErr
	}
	deployment, deploymentErr := k.getJSON(ctx, contextName, namespace, "deployment", controlPlaneName)
	if deploymentErr != nil && !isNotFound(deploymentErr) {
		return target, nil, nil, deploymentErr
	}
	policyObject, policyErr := k.getJSON(ctx, contextName, namespace, "configmap", policyRecord)
	if policyErr != nil && !isNotFound(policyErr) {
		return target, nil, nil, policyErr
	}
	service, serviceErr := k.getJSON(ctx, contextName, namespace, "service", controlPlaneName)
	if serviceErr != nil && !isNotFound(serviceErr) {
		return target, nil, nil, serviceErr
	}
	for _, existing := range []struct {
		kind   string
		name   string
		object map[string]any
	}{
		{"configmap", platformRecord, record},
		{"configmap", policyRecord, policyObject},
		{"deployment", controlPlaneName, deployment},
		{"service", controlPlaneName, service},
	} {
		if err := requireManaged(existing.object, existing.kind, existing.name); err != nil {
			return target, nil, nil, err
		}
	}

	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		return target, nil, nil, err
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		return target, nil, nil, fmt.Errorf("encode reference policy: %w", err)
	}
	namespaceReady := objectName(namespaceObject) == namespace
	revision := objectAnnotation(record, "agenova.io/platform-revision")
	recordReady := revision == request.Platform.Revision && objectData(record, "platform-lock.json") == lockJSON
	policyReady := objectAnnotation(policyObject, "agenova.io/platform-revision") == request.Platform.Revision && objectData(policyObject, "policy.json") == string(policyData)
	ready := deploymentMatches(deployment, deploymentObject(request, namespace), request.Platform.Revision)
	serviceReady := serviceMatches(service, serviceObject(namespace))
	var changes []platformapply.Change
	if !namespaceReady {
		changes = append(changes, platformapply.Change{Component: namespace, Action: "create", Detail: "create the selected Agenova namespace"})
	}
	if !recordReady {
		action := "create"
		if revision != "" {
			action = "update"
		}
		changes = append(changes, platformapply.Change{Component: platformRecord, Action: action, Detail: "reconcile effective Platform revision and adapter lock"})
	}
	if !ready {
		changes = append(changes, platformapply.Change{Component: controlPlaneName, Action: "reconcile", Detail: "run the internal reference control plane at the effective revision"})
	}
	if !policyReady {
		changes = append(changes, platformapply.Change{Component: policyRecord, Action: "reconcile", Detail: "seed the versioned reference default-deny policy"})
	}
	if !serviceReady {
		action := "create"
		if service != nil {
			action = "reconcile"
		}
		changes = append(changes, platformapply.Change{Component: controlPlaneName + "-service", Action: action, Detail: "expose the internal reference status endpoint inside the cluster"})
	}
	statuses := []platformapply.ComponentStatus{
		{Name: namespace, Category: "deployment", State: state(namespaceReady, "available", "unavailable")},
		{Name: platformRecord, Category: "deployment", State: state(recordReady, "configured", "unavailable"), Reference: request.Platform.Revision},
		{Name: policyRecord, Category: "policy", State: state(policyReady, "configured", "unavailable"), Reference: platformapply.ReferencePolicyID + "@" + platformapply.ReferencePolicyVersion},
		{Name: controlPlaneName, Category: "deployment", State: state(ready, "available", "unavailable")},
		{Name: controlPlaneName + "-service", Category: "deployment", State: state(serviceReady, "available", "unavailable")},
	}
	return target, changes, statuses, nil
}

func (k *KubernetesDeployment) Apply(ctx context.Context, request platformapply.DeploymentRequest) ([]platformapply.ComponentStatus, error) {
	contextName, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return nil, err
	}
	if err := k.Preflight(ctx, request); err != nil {
		return failedStatuses(request.Platform.Revision), err
	}
	namespaceExists, err := k.resourceExists(ctx, contextName, "namespaces", namespace, "")
	if err != nil {
		return failedStatuses(request.Platform.Revision), err
	}
	steps, err := referenceSteps(request, namespace, !namespaceExists)
	if err != nil {
		return failedStatuses(request.Platform.Revision), err
	}
	for index, step := range steps {
		replaceSelector := false
		replacePodSpec := false
		if step.name == controlPlaneName+"-service" {
			replaceSelector, err = k.serviceSelectorDrift(ctx, contextName, namespace)
			if err != nil {
				return k.stepFailure(ctx, request, steps, index, err)
			}
		}
		if step.name == controlPlaneName {
			replacePodSpec, err = k.deploymentPodSpecDrift(ctx, contextName, namespace, step.object)
			if err != nil {
				return k.stepFailure(ctx, request, steps, index, err)
			}
		}
		manifest, err := yaml.Marshal(step.object)
		if err != nil {
			return failedStatuses(request.Platform.Revision), fmt.Errorf("encode Kubernetes %s manifest: %w", step.name, err)
		}
		if _, err := k.run(ctx, manifest, "--context", contextName, "apply", "-f", "-"); err != nil {
			return k.stepFailure(ctx, request, steps, index, err)
		}
		if replaceSelector {
			if err := k.replaceServiceSelector(ctx, contextName, namespace); err != nil {
				return k.stepFailure(ctx, request, steps, index, err)
			}
		}
		if replacePodSpec {
			if err := k.replaceDeploymentPodSpec(ctx, contextName, namespace, step.object); err != nil {
				return k.stepFailure(ctx, request, steps, index, err)
			}
		}
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "rollout", "status", "deployment/"+controlPlaneName, "--timeout=60s"); err != nil {
		statuses := k.observedAfterFailure(ctx, request)
		for index := range statuses {
			if statuses[index].Name == controlPlaneName && statuses[index].Category == "deployment" {
				statuses[index].State = "failed"
			}
		}
		return statuses, fmt.Errorf("wait for reference control plane Ready: %w", err)
	}
	_, remaining, statuses, err := k.Plan(ctx, request)
	if err != nil {
		return failedStatuses(request.Platform.Revision), fmt.Errorf("verify reference control plane after rollout: %w", err)
	}
	if len(remaining) != 0 {
		return statuses, fmt.Errorf("reference control plane is not reconciled after rollout; inspect the remaining plan")
	}
	return statuses, nil
}

func (k *KubernetesDeployment) stepFailure(ctx context.Context, request platformapply.DeploymentRequest, steps []manifestStep, index int, cause error) ([]platformapply.ComponentStatus, error) {
	statuses := k.observedAfterFailure(ctx, request)
	markStepStatus(statuses, steps[index], "failed")
	for _, pending := range steps[index+1:] {
		markStepStatus(statuses, pending, "pending")
	}
	return statuses, fmt.Errorf("reconcile Kubernetes %s: %w", steps[index].name, cause)
}

func (k *KubernetesDeployment) serviceSelectorDrift(ctx context.Context, contextName, namespace string) (bool, error) {
	object, err := k.getJSON(ctx, contextName, namespace, "service", controlPlaneName)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	spec, _ := object["spec"].(map[string]any)
	selector, _ := spec["selector"].(map[string]any)
	desiredSpec := serviceObject(namespace)["spec"].(map[string]any)
	return !reflect.DeepEqual(selector, desiredSpec["selector"]), nil
}

func (k *KubernetesDeployment) deploymentPodSpecDrift(ctx context.Context, contextName, namespace string, desired map[string]any) (bool, error) {
	object, err := k.getJSON(ctx, contextName, namespace, "deployment", controlPlaneName)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !managedPodSpecMatches(object, desired), nil
}

func (k *KubernetesDeployment) replaceDeploymentPodSpec(ctx context.Context, contextName, namespace string, desired map[string]any) error {
	spec := desired["spec"].(map[string]any)["template"].(map[string]any)["spec"]
	patch, err := json.Marshal([]map[string]any{{"op": "replace", "path": "/spec/template/spec", "value": spec}})
	if err != nil {
		return fmt.Errorf("encode Deployment pod spec patch: %w", err)
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "patch", "deployment", controlPlaneName, "--type=json", "-p", string(patch)); err != nil {
		return fmt.Errorf("replace managed Deployment pod spec: %w", err)
	}
	return nil
}

func (k *KubernetesDeployment) replaceServiceSelector(ctx context.Context, contextName, namespace string) error {
	desiredSpec := serviceObject(namespace)["spec"].(map[string]any)
	patch, err := json.Marshal([]map[string]any{{"op": "replace", "path": "/spec/selector", "value": desiredSpec["selector"]}})
	if err != nil {
		return fmt.Errorf("encode Service selector patch: %w", err)
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "patch", "service", controlPlaneName, "--type=json", "-p", string(patch)); err != nil {
		return fmt.Errorf("replace managed Service selector: %w", err)
	}
	return nil
}

type manifestStep struct {
	name     string
	category string
	object   map[string]any
}

func markStepStatus(statuses []platformapply.ComponentStatus, step manifestStep, state string) {
	for index := range statuses {
		if statuses[index].Name == step.name && statuses[index].Category == step.category {
			statuses[index].State = state
			return
		}
	}
}

func (k *KubernetesDeployment) observedAfterFailure(ctx context.Context, request platformapply.DeploymentRequest) []platformapply.ComponentStatus {
	_, _, statuses, err := k.Plan(ctx, request)
	if err != nil {
		return failedStatuses(request.Platform.Revision)
	}
	return statuses
}

func (k *KubernetesDeployment) Preflight(ctx context.Context, request platformapply.DeploymentRequest) error {
	contextName, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return err
	}
	return k.preflight(ctx, contextName, namespace)
}

func (k *KubernetesDeployment) preflight(ctx context.Context, contextName, namespace string) error {
	// rollout status watches the Deployment after apply; require that verb before
	// any target mutation, not after the resources have already been created.
	if err := k.requireRBAC(ctx, contextName, "watch", "deployments.apps", namespace); err != nil {
		return err
	}
	targets := []struct {
		resource  string
		name      string
		namespace string
	}{
		{resource: "namespaces", name: namespace},
		{resource: "configmaps", name: platformRecord, namespace: namespace},
		{resource: "configmaps", name: policyRecord, namespace: namespace},
		{resource: "deployments.apps", name: controlPlaneName, namespace: namespace},
		{resource: "services", name: controlPlaneName, namespace: namespace},
	}
	for _, target := range targets {
		if err := k.requireRBAC(ctx, contextName, "get", target.resource, target.namespace); err != nil {
			return err
		}
		exists, err := k.resourceExists(ctx, contextName, target.resource, target.name, target.namespace)
		if err != nil {
			return err
		}
		if exists && target.resource != "namespaces" {
			object, err := k.getJSON(ctx, contextName, target.namespace, target.resource, target.name)
			if err != nil {
				return err
			}
			if err := requireManaged(object, target.resource, target.name); err != nil {
				return err
			}
		}
		if exists && target.resource == "namespaces" {
			object, err := k.getJSON(ctx, contextName, "", target.resource, target.name)
			if err != nil {
				return err
			}
			if namespaceTerminating(object) {
				return fmt.Errorf("selected Kubernetes namespace %q is terminating; wait for deletion or choose another namespace", target.name)
			}
		}
		// The selected namespace may predate Agenova. Once it exists, do not
		// mutate its metadata just to install namespaced Agenova resources.
		if exists && target.resource == "namespaces" {
			continue
		}
		action := "create"
		if exists {
			action = "patch"
		}
		if err := k.requireRBAC(ctx, contextName, action, target.resource, target.namespace); err != nil {
			return err
		}
	}
	return nil
}

func (k *KubernetesDeployment) requireRBAC(ctx context.Context, contextName, action, resource, namespace string) error {
	args := []string{"--context", contextName, "auth", "can-i", action, resource}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	result, err := k.run(ctx, nil, args...)
	if err != nil || strings.TrimSpace(result.stdout) != "yes" {
		detail := action + " " + resource
		if namespace != "" {
			detail += " --namespace " + namespace
		}
		return fmt.Errorf("current Kubernetes identity lacks required RBAC: %s", detail)
	}
	return nil
}

func (k *KubernetesDeployment) resourceExists(ctx context.Context, contextName, resource, name, namespace string) (bool, error) {
	args := []string{"--context", contextName}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "get", resource, name, "-o", "name")
	_, err := k.run(ctx, nil, args...)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Kubernetes %s/%s before apply: %w", resource, name, err)
	}
	return true, nil
}

func (k *KubernetesDeployment) getJSON(ctx context.Context, contextName, namespace, kind, name string) (map[string]any, error) {
	args := []string{"--context", contextName}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "get", kind, name, "-o", "json")
	result, err := k.run(ctx, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("read Kubernetes %s/%s: %w", kind, name, err)
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &object); err != nil {
		return nil, fmt.Errorf("decode Kubernetes %s/%s status", kind, name)
	}
	return object, nil
}

func (k *KubernetesDeployment) run(ctx context.Context, input []byte, args ...string) (commandResult, error) {
	if k == nil || k.runner == nil {
		return commandResult{}, fmt.Errorf("kubectl runner is not configured")
	}
	result, err := k.runner.Run(ctx, input, args...)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return result, errKubectlUnavailable
		}
		return result, errors.New(safeKubectlError(result.stderr))
	}
	return result, nil
}

func deploymentCoordinates(config map[string]any) (string, string, error) {
	contextName, _ := config["context"].(string)
	namespace, _ := config["namespace"].(string)
	contextName, namespace = strings.TrimSpace(contextName), strings.TrimSpace(namespace)
	if contextName == "" || namespace == "" {
		return "", "", fmt.Errorf("deployment adapter requires context and namespace")
	}
	return contextName, namespace, nil
}

func referenceSteps(request platformapply.DeploymentRequest, namespace string, createNamespace bool) ([]manifestStep, error) {
	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		return nil, err
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		return nil, fmt.Errorf("encode reference policy: %w", err)
	}
	steps := []manifestStep{
		{name: platformRecord, category: "deployment", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": platformRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}}},
		{name: policyRecord, category: "policy", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": policyRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyData)}}},
		{name: controlPlaneName, category: "deployment", object: deploymentObject(request, namespace)},
		{name: controlPlaneName + "-service", category: "deployment", object: serviceObject(namespace)},
	}
	if createNamespace {
		steps = append([]manifestStep{{name: namespace, category: "deployment", object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": namespace, "labels": managedLabels()}}}}, steps...)
	}
	return steps, nil
}

func deploymentObject(request platformapply.DeploymentRequest, namespace string) map[string]any {
	labels := map[string]any{"app.kubernetes.io/name": controlPlaneName, "app.kubernetes.io/managed-by": "agenova"}
	container := map[string]any{
		"name":            "control-plane",
		"image":           controlPlaneImage,
		"imagePullPolicy": "IfNotPresent",
		"ports":           []any{map[string]any{"name": "http", "containerPort": 8080}},
		"env": []any{
			map[string]any{"name": "AGENOVA_PLATFORM_NAME", "value": request.Platform.PlatformName},
			map[string]any{"name": "AGENOVA_PLATFORM_REVISION", "value": request.Platform.Revision},
			map[string]any{"name": "AGENOVA_POLICY_REF", "value": platformapply.ReferencePolicyID + "@" + platformapply.ReferencePolicyVersion},
		},
		"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/readyz", "port": "http"}, "initialDelaySeconds": 1, "periodSeconds": 2},
	}
	template := map[string]any{
		"metadata": map[string]any{"labels": labels, "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}},
		"spec":     map[string]any{"containers": []any{container}},
	}
	return map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": controlPlaneName, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}},
		"spec":     map[string]any{"replicas": 1, "selector": map[string]any{"matchLabels": labels}, "template": template},
	}
}

func serviceObject(namespace string) map[string]any {
	selector := map[string]any{"app.kubernetes.io/name": controlPlaneName, "app.kubernetes.io/managed-by": "agenova"}
	return map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": controlPlaneName, "namespace": namespace, "labels": managedLabels()}, "spec": map[string]any{"type": "ClusterIP", "selector": selector, "ports": []any{map[string]any{"name": "http", "port": 8080, "targetPort": "http"}}}}
}

func managedLabels() map[string]any {
	return map[string]any{"app.kubernetes.io/managed-by": "agenova", "app.kubernetes.io/part-of": "agenova"}
}

func referencePolicyJSON() ([]byte, error) {
	bundle := policy.ReferenceBundle()
	rules := make([]map[string]string, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		rules = append(rules, map[string]string{"team": rule.Team, "action": rule.Action, "project": rule.Project, "templateRef": rule.TemplateRef})
	}
	return json.Marshal(map[string]any{"id": bundle.ID, "version": bundle.Version, "rules": rules})
}

func objectAnnotation(object map[string]any, key string) string {
	metadata, _ := object["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	value, _ := annotations[key].(string)
	return value
}

func objectName(object map[string]any) string {
	metadata, _ := object["metadata"].(map[string]any)
	value, _ := metadata["name"].(string)
	return value
}

func namespaceTerminating(object map[string]any) bool {
	metadata, _ := object["metadata"].(map[string]any)
	if timestamp, _ := metadata["deletionTimestamp"].(string); timestamp != "" {
		return true
	}
	status, _ := object["status"].(map[string]any)
	return status["phase"] == "Terminating"
}

func objectData(object map[string]any, key string) string {
	data, _ := object["data"].(map[string]any)
	value, _ := data[key].(string)
	return value
}

func requireManaged(object map[string]any, kind, name string) error {
	if object == nil {
		return nil
	}
	metadata, _ := object["metadata"].(map[string]any)
	labels, _ := metadata["labels"].(map[string]any)
	if labels["app.kubernetes.io/managed-by"] != "agenova" {
		return fmt.Errorf("Kubernetes %s/%s already exists but is not managed by Agenova", kind, name)
	}
	return nil
}

func deploymentMatches(object, desired map[string]any, revision string) bool {
	if objectAnnotation(object, "agenova.io/platform-revision") != revision {
		return false
	}
	data, err := json.Marshal(desired["spec"])
	if err != nil {
		return false
	}
	var expectedSpec any
	if json.Unmarshal(data, &expectedSpec) != nil || !expectedFieldsMatch(object["spec"], expectedSpec) || !managedPodSpecMatches(object, desired) {
		return false
	}
	metadata, _ := object["metadata"].(map[string]any)
	status, _ := object["status"].(map[string]any)
	generation, _ := metadata["generation"].(float64)
	observed, _ := status["observedGeneration"].(float64)
	replicas, _ := status["replicas"].(float64)
	updated, _ := status["updatedReplicas"].(float64)
	available, _ := status["availableReplicas"].(float64)
	return generation > 0 && observed >= generation && replicas == 1 && updated == 1 && available == 1
}

func managedPodSpecMatches(object, desired map[string]any) bool {
	actualSpec, _ := object["spec"].(map[string]any)
	actualTemplate, _ := actualSpec["template"].(map[string]any)
	actualPod, _ := actualTemplate["spec"].(map[string]any)
	wantedSpec, _ := desired["spec"].(map[string]any)
	wantedTemplate, _ := wantedSpec["template"].(map[string]any)
	wantedPod, _ := wantedTemplate["spec"].(map[string]any)
	if actualPod == nil || wantedPod == nil {
		return false
	}
	podDefaults := map[string]any{"dnsPolicy": "ClusterFirst", "restartPolicy": "Always", "schedulerName": "default-scheduler", "securityContext": map[string]any{}, "terminationGracePeriodSeconds": float64(30)}
	if !noUnexpectedFields(actualPod, wantedPod, podDefaults) {
		return false
	}
	actualContainers, _ := actualPod["containers"].([]any)
	wantedContainers, _ := wantedPod["containers"].([]any)
	if len(actualContainers) != len(wantedContainers) {
		return false
	}
	containerDefaults := map[string]any{"resources": map[string]any{}, "terminationMessagePath": "/dev/termination-log", "terminationMessagePolicy": "File"}
	for index, wanted := range wantedContainers {
		actual, _ := actualContainers[index].(map[string]any)
		want, _ := wanted.(map[string]any)
		if !noUnexpectedFields(actual, want, containerDefaults) {
			return false
		}
	}
	return true
}

func noUnexpectedFields(actual, desired, defaults map[string]any) bool {
	if actual == nil || desired == nil {
		return false
	}
	for key, value := range actual {
		if _, owned := desired[key]; owned {
			continue
		}
		defaultValue, known := defaults[key]
		if !known || !reflect.DeepEqual(value, defaultValue) {
			return false
		}
	}
	return true
}

func expectedFieldsMatch(actual, expected any) bool {
	switch wanted := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range wanted {
			if !expectedFieldsMatch(got[key], value) {
				return false
			}
		}
		return true
	case []any:
		got, ok := actual.([]any)
		if !ok || len(got) != len(wanted) {
			return false
		}
		for index := range wanted {
			if !expectedFieldsMatch(got[index], wanted[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

func serviceMatches(object, desired map[string]any) bool {
	if objectName(object) != controlPlaneName {
		return false
	}
	spec, _ := object["spec"].(map[string]any)
	expected, _ := desired["spec"].(map[string]any)
	selector, _ := spec["selector"].(map[string]any)
	expectedSelector, _ := expected["selector"].(map[string]any)
	if !reflect.DeepEqual(selector, expectedSelector) {
		return false
	}
	data, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	var normalized any
	return json.Unmarshal(data, &normalized) == nil && expectedFieldsMatch(spec, normalized)
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func safeKubectlError(stderr string) string {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "notfound"), strings.Contains(lower, "not found"):
		return "Kubernetes resource not found"
	case strings.Contains(lower, "forbidden"), strings.Contains(lower, "unauthorized"):
		return "Kubernetes access denied for the current identity"
	case strings.Contains(lower, "connection refused"), strings.Contains(lower, "unable to connect"), strings.Contains(lower, "i/o timeout"):
		return "Kubernetes API is unreachable for the selected context"
	default:
		return "kubectl command failed; check the selected context and cluster access"
	}
}

func state(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func failedStatuses(revision string) []platformapply.ComponentStatus {
	statuses := []platformapply.ComponentStatus{{Name: platformRecord, Category: "deployment", State: "failed", Reference: revision}, {Name: controlPlaneName, Category: "deployment", State: "failed"}, {Name: controlPlaneName + "-service", Category: "deployment", State: "failed"}, {Name: policyRecord, Category: "policy", State: "failed", Reference: platformapply.ReferencePolicyID + "@" + platformapply.ReferencePolicyVersion}}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses
}
