// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
	if _, err := k.run(ctx, nil, "--context", contextName, "version", "--request-timeout=5s", "-o", "json"); err != nil {
		return target, nil, nil, fmt.Errorf("selected Kubernetes context %q is unavailable", contextName)
	}
	namespaceObject, namespaceErr := k.getJSON(ctx, contextName, "", "namespace", namespace)
	if namespaceErr != nil && !isNotFound(namespaceErr) {
		return target, nil, nil, namespaceErr
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
	ready := deploymentMatches(deployment, request.Platform.Revision)
	serviceReady := serviceMatches(service)
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
		changes = append(changes, platformapply.Change{Component: controlPlaneName + "-service", Action: "create", Detail: "expose the internal reference status endpoint inside the cluster"})
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
	manifest, err := referenceManifest(request, namespace)
	if err != nil {
		return failedStatuses(request.Platform.Revision), err
	}
	if _, err := k.run(ctx, manifest, "--context", contextName, "apply", "-f", "-"); err != nil {
		return failedStatuses(request.Platform.Revision), fmt.Errorf("reconcile Kubernetes resources: %w", err)
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "rollout", "status", "deployment/"+controlPlaneName, "--timeout=60s"); err != nil {
		return failedStatuses(request.Platform.Revision), fmt.Errorf("wait for reference control plane Ready: %w", err)
	}
	return []platformapply.ComponentStatus{
		{Name: namespace, Category: "deployment", State: "available"},
		{Name: platformRecord, Category: "deployment", State: "configured", Reference: request.Platform.Revision},
		{Name: policyRecord, Category: "policy", State: "configured", Reference: platformapply.ReferencePolicyID + "@" + platformapply.ReferencePolicyVersion},
		{Name: controlPlaneName, Category: "deployment", State: "available"},
		{Name: controlPlaneName + "-service", Category: "deployment", State: "available"},
	}, nil
}

func (k *KubernetesDeployment) Preflight(ctx context.Context, request platformapply.DeploymentRequest) error {
	contextName, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return err
	}
	return k.preflight(ctx, contextName, namespace)
}

func (k *KubernetesDeployment) preflight(ctx context.Context, contextName, namespace string) error {
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
		return nil, fmt.Errorf("read Kubernetes %s/%s: %s", kind, name, safeKubectlError(result.stderr))
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
		message := safeKubectlError(result.stderr)
		if message == "" {
			message = "kubectl command failed"
		}
		return result, fmt.Errorf("%s", message)
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

func referenceManifest(request platformapply.DeploymentRequest, namespace string) ([]byte, error) {
	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		return nil, err
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		return nil, fmt.Errorf("encode reference policy: %w", err)
	}
	objects := []map[string]any{
		{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": namespace, "labels": managedLabels()}},
		{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": platformRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON}},
		{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": policyRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyData)}},
		deploymentObject(request, namespace),
		serviceObject(namespace),
	}
	var output bytes.Buffer
	for i, object := range objects {
		if i > 0 {
			output.WriteString("---\n")
		}
		data, err := yaml.Marshal(object)
		if err != nil {
			return nil, fmt.Errorf("encode Kubernetes reference manifest: %w", err)
		}
		output.Write(data)
	}
	return output.Bytes(), nil
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

func deploymentMatches(object map[string]any, revision string) bool {
	if objectAnnotation(object, "agenova.io/platform-revision") != revision || availableReplicas(object) < 1 {
		return false
	}
	spec, _ := object["spec"].(map[string]any)
	replicas, _ := spec["replicas"].(float64)
	template, _ := spec["template"].(map[string]any)
	templateMeta, _ := template["metadata"].(map[string]any)
	annotations, _ := templateMeta["annotations"].(map[string]any)
	podSpec, _ := template["spec"].(map[string]any)
	containers, _ := podSpec["containers"].([]any)
	if replicas != 1 || annotations["agenova.io/platform-revision"] != revision || len(containers) != 1 {
		return false
	}
	container, _ := containers[0].(map[string]any)
	return container["image"] == controlPlaneImage
}

func serviceMatches(object map[string]any) bool {
	if objectName(object) != controlPlaneName {
		return false
	}
	spec, _ := object["spec"].(map[string]any)
	selector, _ := spec["selector"].(map[string]any)
	ports, _ := spec["ports"].([]any)
	if spec["type"] != "ClusterIP" || selector["app.kubernetes.io/name"] != controlPlaneName || len(ports) != 1 {
		return false
	}
	port, _ := ports[0].(map[string]any)
	return port["port"] == float64(8080) && port["targetPort"] == "http"
}

func availableReplicas(object map[string]any) int {
	status, _ := object["status"].(map[string]any)
	value, _ := status["availableReplicas"].(float64)
	return int(value)
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func safeKubectlError(stderr string) string {
	line := strings.TrimSpace(stderr)
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	if len(line) > 240 {
		line = line[:240]
	}
	return line
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
