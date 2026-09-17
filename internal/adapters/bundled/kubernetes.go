// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strings"

	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"gopkg.in/yaml.v3"
)

const (
	controlPlaneName   = "agenova-control-plane"
	platformRecord     = "agenova-platform"
	activePolicyRecord = "agenova-active-policy"
	controlPlaneRole   = "agenova-control-plane-runtime"
	controlPlaneImage  = "agenova-control-plane:0.1.0"
)

// The reference install writes the same immutable record name as the
// operator registration store; otherwise ActivePolicy cannot read the seed.
var policyRecord = policyRegistrationRecord(policy.ReferenceBundle().ID, policy.ReferenceBundle().Version)

func policyRegistrationRecord(id, version string) string {
	sum := sha256.Sum256([]byte(id + "@" + version))
	return "agenova-policy-" + hex.EncodeToString(sum[:12])
}

func referenceWorkerImage(resolved *platform.ResolvedPlatform) string {
	if resolved == nil {
		return ""
	}
	for _, instance := range resolved.Instances {
		if instance.Category == platform.CapabilityRuntime {
			image, _ := instance.Config["compatible-worker-image"].(string)
			return image
		}
	}
	return ""
}

func validateReferenceRuntime(request platformapply.DeploymentRequest) error {
	if request.Platform == nil {
		return fmt.Errorf("resolved Platform is required")
	}
	_, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return err
	}
	count := 0
	for _, instance := range request.Platform.Instances {
		if instance.Category != platform.CapabilityRuntime {
			continue
		}
		count++
		connection, _ := instance.Config["connection"].(map[string]any)
		workerNamespace, _ := connection["namespace"].(string)
		if workerNamespace != namespace {
			return fmt.Errorf("reference runtime namespace must match the installed Control Plane namespace")
		}
		if instance.Config["compatible-worker-image"] != referenceControlledWorkerImage {
			return fmt.Errorf("reference runtime requires the bundled controlled worker image")
		}
	}
	if count != 1 {
		return fmt.Errorf("reference Control Plane requires exactly one runtime backend")
	}
	backends := map[string]string{}
	for _, instance := range request.Platform.Instances {
		if instance.Category == platform.CapabilityModel {
			endpoint, _ := instance.Config["endpoint"].(string)
			backends[instance.Name] = endpoint
		}
	}
	models := map[string]string{}
	endpoint := ""
	for _, profile := range request.Platform.Profiles {
		if profile.Capability != platform.CapabilityModel {
			continue
		}
		backend := backends[profile.BackendRef]
		if backend == "" || (endpoint != "" && endpoint != backend) {
			return fmt.Errorf("reference Control Plane requires model profiles on one configured endpoint")
		}
		endpoint = backend
		model, _ := profile.Config["model"].(string)
		models[profile.Name] = model
	}
	_, err = modelprovider.New(modelprovider.Config{Endpoint: endpoint, AllowDockerHostHTTP: strings.HasPrefix(endpoint, "http://host.docker.internal:"), Models: models})
	if err != nil {
		return fmt.Errorf("reference Control Plane model composition is unsupported: %w", err)
	}
	return nil
}

// ValidateComposition is target-specific but read-only: generic Platform
// validation calls it before planning or applying cluster resources.
func (*KubernetesDeployment) ValidateComposition(request platformapply.DeploymentRequest) error {
	return validateReferenceRuntime(request)
}

// A later policy registration may legitimately activate a different version.
// The initial reference is only used when no valid active registry pointer
// exists; platform reconciliation must not silently undo policy apply.
func (k *KubernetesDeployment) activePolicyReady(ctx context.Context, contextName, namespace string, pointer map[string]any, initial registration.PolicyReference) (bool, string, error) {
	if pointer == nil {
		return false, initial.ID + "@" + initial.Version, nil
	}
	var ref registration.PolicyReference
	if err := json.Unmarshal([]byte(objectData(pointer, "reference.json")), &ref); err != nil || ref.ID == "" || ref.Version == "" {
		return false, initial.ID + "@" + initial.Version, nil
	}
	identity := ref.ID + "@" + ref.Version
	// The initial record is checked independently as policyRecord/policyReady.
	// Reconcile a drifted record without rewriting an already-correct pointer.
	if ref.ID == initial.ID && ref.Version == initial.Version {
		return true, identity, nil
	}
	recordName := policyRegistrationRecord(ref.ID, ref.Version)
	record, err := k.getJSON(ctx, contextName, namespace, "configmap", recordName)
	if isNotFound(err) {
		return false, identity, nil
	}
	if err != nil {
		return false, identity, err
	}
	if err := requireManaged(record, "configmap", recordName); err != nil {
		return false, identity, err
	}
	var bundle policy.PolicyBundle
	if err := json.Unmarshal([]byte(objectData(record, "policy.json")), &bundle); err != nil || policy.ValidateBundle(bundle) != nil || bundle.ID != ref.ID || bundle.Version != ref.Version {
		return false, identity, nil
	}
	return true, identity, nil
}

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
	if err := k.ValidateComposition(request); err != nil {
		return target, nil, nil, err
	}
	if err := (ReferencePolicyCatalog{}).Require(request.Platform.InitialPolicyRef); err != nil {
		return target, nil, nil, fmt.Errorf("initial policy: %w", err)
	}
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
	activePolicyObject, activePolicyErr := k.getJSON(ctx, contextName, namespace, "configmap", activePolicyRecord)
	if activePolicyErr != nil && !isNotFound(activePolicyErr) {
		return target, nil, nil, activePolicyErr
	}
	service, serviceErr := k.getJSON(ctx, contextName, namespace, "service", controlPlaneName)
	if serviceErr != nil && !isNotFound(serviceErr) {
		return target, nil, nil, serviceErr
	}
	serviceAccount, serviceAccountErr := k.getJSON(ctx, contextName, namespace, "serviceaccount", controlPlaneName)
	if serviceAccountErr != nil && !isNotFound(serviceAccountErr) {
		return target, nil, nil, serviceAccountErr
	}
	role, roleErr := k.getJSON(ctx, contextName, namespace, "role", controlPlaneRole)
	if roleErr != nil && !isNotFound(roleErr) {
		return target, nil, nil, roleErr
	}
	roleBinding, roleBindingErr := k.getJSON(ctx, contextName, namespace, "rolebinding", controlPlaneRole)
	if roleBindingErr != nil && !isNotFound(roleBindingErr) {
		return target, nil, nil, roleBindingErr
	}
	for _, existing := range []struct {
		kind   string
		name   string
		object map[string]any
	}{
		{"configmap", platformRecord, record},
		{"configmap", policyRecord, policyObject},
		{"configmap", activePolicyRecord, activePolicyObject},
		{"deployment", controlPlaneName, deployment},
		{"service", controlPlaneName, service},
		{"serviceaccount", controlPlaneName, serviceAccount},
		{"role", controlPlaneRole, role},
		{"rolebinding", controlPlaneRole, roleBinding},
	} {
		if err := requireManaged(existing.object, existing.kind, existing.name); err != nil {
			return target, nil, nil, err
		}
	}

	lockJSON, err := platformapply.EncodeLock(request.Lock)
	if err != nil {
		return target, nil, nil, err
	}
	effectiveJSON, err := json.Marshal(request.Platform)
	if err != nil {
		return target, nil, nil, fmt.Errorf("encode effective Platform: %w", err)
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		return target, nil, nil, fmt.Errorf("encode reference policy: %w", err)
	}
	namespaceReady := objectName(namespaceObject) == namespace
	revision := objectAnnotation(record, "agenova.io/platform-revision")
	recordReady := revision == request.Platform.Revision && objectData(record, "platform-lock.json") == lockJSON && objectData(record, "effective-platform.json") == string(effectiveJSON)
	policyReady := objectAnnotation(policyObject, "agenova.io/platform-revision") == request.Platform.Revision && objectData(policyObject, "policy.json") == string(policyData)
	activePolicyReady, activePolicyRef, err := k.activePolicyReady(ctx, contextName, namespace, activePolicyObject, registration.PolicyReference{ID: request.Platform.InitialPolicyRef.ID, Version: request.Platform.InitialPolicyRef.Version})
	if err != nil {
		return target, nil, nil, err
	}
	ready := deploymentMatches(deployment, deploymentObject(request, namespace), request.Platform.Revision)
	serviceReady := serviceMatches(service, serviceObject(namespace))
	saReady := managedObjectMatches(serviceAccount, serviceAccountObject(namespace))
	roleReady := managedObjectMatches(role, roleObject(namespace))
	bindingReady := managedObjectMatches(roleBinding, roleBindingObject(namespace))
	var changes []platformapply.Change
	if !namespaceReady {
		changes = append(changes, platformapply.Change{Component: namespace, Action: "create", Detail: "create the selected Agenova namespace"})
	}
	if !recordReady {
		action := "create"
		if record != nil {
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
	if !activePolicyReady {
		changes = append(changes, platformapply.Change{Component: activePolicyRecord, Action: "reconcile", Detail: "activate the initial policy in the operator registry"})
	}
	if !serviceReady {
		action := "create"
		if service != nil {
			action = "reconcile"
		}
		changes = append(changes, platformapply.Change{Component: controlPlaneName + "-service", Action: action, Detail: "expose the internal reference status endpoint inside the cluster"})
	}
	for _, component := range []struct {
		name  string
		ready bool
	}{{controlPlaneName + "-account", saReady}, {controlPlaneRole, roleReady}, {controlPlaneRole + "-binding", bindingReady}} {
		if !component.ready {
			changes = append(changes, platformapply.Change{Component: component.name, Action: "reconcile", Detail: "install reference control-plane namespace permissions"})
		}
	}
	statuses := []platformapply.ComponentStatus{
		{Name: namespace, Category: "deployment", State: state(namespaceReady, "available", "unavailable")},
		{Name: platformRecord, Category: "deployment", State: state(recordReady, "configured", "unavailable"), Reference: request.Platform.Revision},
		{Name: policyRecord, Category: "policy", State: state(policyReady, "configured", "unavailable"), Reference: request.Platform.InitialPolicyRef.ID + "@" + request.Platform.InitialPolicyRef.Version},
		{Name: activePolicyRecord, Category: "policy", State: state(activePolicyReady, "configured", "unavailable"), Reference: activePolicyRef},
		{Name: controlPlaneName, Category: "deployment", State: state(ready, "available", "unavailable")},
		{Name: controlPlaneName + "-service", Category: "deployment", State: state(serviceReady, "available", "unavailable")},
		{Name: controlPlaneName + "-account", Category: "deployment", State: state(saReady, "configured", "unavailable")},
		{Name: controlPlaneRole, Category: "deployment", State: state(roleReady, "configured", "unavailable")},
		{Name: controlPlaneRole + "-binding", Category: "deployment", State: state(bindingReady, "configured", "unavailable")},
	}
	return target, changes, statuses, nil
}

func (k *KubernetesDeployment) Apply(ctx context.Context, request platformapply.DeploymentRequest) ([]platformapply.ComponentStatus, bool, error) {
	contextName, namespace, err := deploymentCoordinates(request.Config)
	if err != nil {
		return nil, false, err
	}
	// A direct adapter call has no confirmed plan. Preserve fail-closed RBAC
	// preflight before inspecting and planning the target.
	if request.TargetChanges == nil {
		if err := k.Preflight(ctx, request); err != nil {
			return failedStatuses(request.Platform.Revision), false, err
		}
	}
	_, currentChanges, currentStatuses, err := k.Plan(ctx, request)
	if err != nil {
		return nil, false, err
	}
	if request.TargetChanges != nil && !reflect.DeepEqual(currentChanges, request.TargetChanges) {
		return nil, false, fmt.Errorf("Kubernetes target changed before apply; review the new plan and retry")
	}
	request.TargetChanges = currentChanges
	if len(currentChanges) == 0 {
		return currentStatuses, false, nil
	}
	if err := k.Preflight(ctx, request); err != nil {
		return failedStatuses(request.Platform.Revision), false, err
	}
	namespaceExists, err := k.resourceExists(ctx, contextName, "namespaces", namespace, "")
	if err != nil {
		return failedStatuses(request.Platform.Revision), false, err
	}
	steps, err := referenceSteps(request, namespace, !namespaceExists)
	if err != nil {
		return failedStatuses(request.Platform.Revision), false, err
	}
	mutationAttempted := false
	for index, step := range steps {
		if !plannedComponent(request.TargetChanges, step.name) {
			continue
		}
		replaceSelector := false
		replaceDeploymentSpec := false
		if step.name == controlPlaneName+"-service" {
			replaceSelector, err = k.serviceSelectorDrift(ctx, contextName, namespace)
			if err != nil {
				return k.stepFailure(ctx, request, steps, index, mutationAttempted, err)
			}
		}
		if step.name == controlPlaneName {
			replaceDeploymentSpec, err = k.deploymentSpecDrift(ctx, contextName, namespace, step.object)
			if err != nil {
				return k.stepFailure(ctx, request, steps, index, mutationAttempted, err)
			}
		}
		manifest, err := yaml.Marshal(step.object)
		if err != nil {
			return failedStatuses(request.Platform.Revision), mutationAttempted, fmt.Errorf("encode Kubernetes %s manifest: %w", step.name, err)
		}
		// A mutating command can fail after the API server accepted a write.
		// Conservatively mark it attempted before invoking kubectl.
		mutationAttempted = true
		if _, err := k.run(ctx, manifest, "--context", contextName, "apply", "-f", "-"); err != nil {
			return k.stepFailure(ctx, request, steps, index, mutationAttempted, err)
		}
		if replaceSelector {
			if err := k.replaceServiceSelector(ctx, contextName, namespace); err != nil {
				return k.stepFailure(ctx, request, steps, index, mutationAttempted, err)
			}
		}
		if replaceDeploymentSpec {
			if err := k.replaceDeploymentSpec(ctx, contextName, namespace, step.object); err != nil {
				return k.stepFailure(ctx, request, steps, index, mutationAttempted, err)
			}
		}
	}
	if plannedComponent(request.TargetChanges, controlPlaneName) {
		if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "rollout", "status", "deployment/"+controlPlaneName, "--timeout=60s"); err != nil {
			statuses := k.observedAfterFailure(ctx, request)
			for index := range statuses {
				if statuses[index].Name == controlPlaneName && statuses[index].Category == "deployment" {
					statuses[index].State = "failed"
				}
			}
			return statuses, mutationAttempted, fmt.Errorf("wait for reference control plane Ready: %w", err)
		}
	}
	_, remaining, statuses, err := k.Plan(ctx, request)
	if err != nil {
		return failedStatuses(request.Platform.Revision), mutationAttempted, fmt.Errorf("verify reference control plane after rollout: %w", err)
	}
	if len(remaining) != 0 {
		return statuses, mutationAttempted, fmt.Errorf("reference control plane is not reconciled after rollout; inspect the remaining plan")
	}
	return statuses, mutationAttempted, nil
}

func (k *KubernetesDeployment) stepFailure(ctx context.Context, request platformapply.DeploymentRequest, steps []manifestStep, index int, mutationAttempted bool, cause error) ([]platformapply.ComponentStatus, bool, error) {
	statuses := k.observedAfterFailure(ctx, request)
	markStepStatus(statuses, steps[index], "failed")
	for _, pending := range steps[index+1:] {
		if plannedComponent(request.TargetChanges, pending.name) {
			markStepStatus(statuses, pending, "pending")
		}
	}
	return statuses, mutationAttempted, fmt.Errorf("reconcile Kubernetes %s: %w", steps[index].name, cause)
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

func (k *KubernetesDeployment) deploymentSpecDrift(ctx context.Context, contextName, namespace string, desired map[string]any) (bool, error) {
	object, err := k.getJSON(ctx, contextName, namespace, "deployment", controlPlaneName)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !managedDeploymentSpecMatches(object, desired), nil
}

func (k *KubernetesDeployment) replaceDeploymentSpec(ctx context.Context, contextName, namespace string, desired map[string]any) error {
	patch, err := json.Marshal([]map[string]any{{"op": "replace", "path": "/spec", "value": desired["spec"]}})
	if err != nil {
		return fmt.Errorf("encode Deployment spec patch: %w", err)
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "patch", "deployment", controlPlaneName, "--type=json", "-p", string(patch)); err != nil {
		return fmt.Errorf("replace managed Deployment spec: %w", err)
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
	if request.TargetChanges == nil {
		// Standalone callers have not yet confirmed a target plan; require
		// authority for every managed reference resource.
		request.TargetChanges = []platformapply.Change{
			{Component: namespace}, {Component: platformRecord},
			{Component: policyRecord}, {Component: activePolicyRecord}, {Component: controlPlaneName},
			{Component: controlPlaneName + "-service"},
			{Component: controlPlaneName + "-account"}, {Component: controlPlaneRole}, {Component: controlPlaneRole + "-binding"},
		}
	}
	return k.preflight(ctx, contextName, namespace, request.TargetChanges)
}

func plannedComponent(changes []platformapply.Change, component string) bool {
	for _, change := range changes {
		if change.Component == component {
			return true
		}
	}
	return false
}

func (k *KubernetesDeployment) preflight(ctx context.Context, contextName, namespace string, changes []platformapply.Change) error {
	// A changed Deployment needs a rollout watch after mutation. ConfigMap or
	// Service-only plans are verified by read-only re-planning instead.
	if plannedComponent(changes, controlPlaneName) {
		if err := k.requireRBAC(ctx, contextName, "watch", "deployments.apps", namespace); err != nil {
			return err
		}
	}
	targets := []struct {
		resource  string
		name      string
		namespace string
	}{
		{resource: "namespaces", name: namespace},
		{resource: "configmaps", name: platformRecord, namespace: namespace},
		{resource: "configmaps", name: policyRecord, namespace: namespace},
		{resource: "configmaps", name: activePolicyRecord, namespace: namespace},
		{resource: "deployments.apps", name: controlPlaneName, namespace: namespace},
		{resource: "services", name: controlPlaneName, namespace: namespace},
		{resource: "serviceaccounts", name: controlPlaneName, namespace: namespace},
		{resource: "roles.rbac.authorization.k8s.io", name: controlPlaneRole, namespace: namespace},
		{resource: "rolebindings.rbac.authorization.k8s.io", name: controlPlaneRole, namespace: namespace},
	}
	for _, target := range targets {
		component := target.name
		if target.resource == "services" {
			component += "-service"
		}
		if target.resource == "serviceaccounts" {
			component += "-account"
		}
		if target.resource == "rolebindings.rbac.authorization.k8s.io" {
			component += "-binding"
		}
		if !plannedComponent(changes, component) {
			continue
		}
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
	effectiveJSON, err := json.Marshal(request.Platform)
	if err != nil {
		return nil, fmt.Errorf("encode effective Platform: %w", err)
	}
	policyData, err := referencePolicyJSON()
	if err != nil {
		return nil, fmt.Errorf("encode reference policy: %w", err)
	}
	activeRef, err := json.Marshal(registration.PolicyReference{ID: request.Platform.InitialPolicyRef.ID, Version: request.Platform.InitialPolicyRef.Version})
	if err != nil {
		return nil, fmt.Errorf("encode active reference policy: %w", err)
	}
	steps := []manifestStep{
		{name: platformRecord, category: "deployment", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": platformRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"platform-lock.json": lockJSON, "effective-platform.json": string(effectiveJSON)}}},
		{name: policyRecord, category: "policy", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": policyRecord, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}, "data": map[string]any{"policy.json": string(policyData)}}},
		{name: activePolicyRecord, category: "policy", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": activePolicyRecord, "namespace": namespace, "labels": managedLabels()}, "data": map[string]any{"reference.json": string(activeRef)}}},
		{name: controlPlaneName + "-account", category: "deployment", object: serviceAccountObject(namespace)},
		{name: controlPlaneRole, category: "deployment", object: roleObject(namespace)},
		{name: controlPlaneRole + "-binding", category: "deployment", object: roleBindingObject(namespace)},
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
			map[string]any{"name": "AGENOVA_POLICY_REF", "value": request.Platform.InitialPolicyRef.ID + "@" + request.Platform.InitialPolicyRef.Version},
			map[string]any{"name": "AGENOVA_ALLOWED_WORKER_IMAGE", "value": referenceWorkerImage(request.Platform)},
		},
		"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/readyz", "port": "http"}, "initialDelaySeconds": 1, "periodSeconds": 2},
	}
	template := map[string]any{
		"metadata": map[string]any{"labels": labels, "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}},
		"spec":     map[string]any{"serviceAccount": controlPlaneName, "serviceAccountName": controlPlaneName, "automountServiceAccountToken": false, "containers": []any{container}},
	}
	container["volumeMounts"] = []any{map[string]any{"name": "platform", "mountPath": "/etc/agenova", "readOnly": true}, map[string]any{"name": "service-token", "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount", "readOnly": true}}
	template["spec"].(map[string]any)["volumes"] = []any{
		map[string]any{"name": "platform", "configMap": map[string]any{"name": platformRecord}},
		map[string]any{"name": "service-token", "projected": map[string]any{"sources": []any{map[string]any{"serviceAccountToken": map[string]any{"path": "token", "expirationSeconds": 3600}}, map[string]any{"configMap": map[string]any{"name": "kube-root-ca.crt", "items": []any{map[string]any{"key": "ca.crt", "path": "ca.crt"}}}}}}},
	}
	return map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": controlPlaneName, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}},
		"spec":     map[string]any{"replicas": 1, "selector": map[string]any{"matchLabels": labels}, "template": template},
	}
}

func serviceAccountObject(namespace string) map[string]any {
	return map[string]any{"apiVersion": "v1", "kind": "ServiceAccount", "metadata": map[string]any{"name": controlPlaneName, "namespace": namespace, "labels": managedLabels()}}
}

func roleObject(namespace string) map[string]any {
	// Installed setup enumerates managed template records in this namespace.
	// Kubernetes RBAC cannot scope list by label.
	rules := []any{map[string]any{"apiGroups": []any{""}, "resources": []any{"configmaps"}, "verbs": []any{"get", "list"}}}
	rules = append(rules, agentsandbox.ReferenceNamespaceRules()...)
	return map[string]any{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "Role", "metadata": map[string]any{"name": controlPlaneRole, "namespace": namespace, "labels": managedLabels()}, "rules": rules}
}

func roleBindingObject(namespace string) map[string]any {
	return map[string]any{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "RoleBinding", "metadata": map[string]any{"name": controlPlaneRole, "namespace": namespace, "labels": managedLabels()}, "roleRef": map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": controlPlaneRole}, "subjects": []any{map[string]any{"kind": "ServiceAccount", "name": controlPlaneName, "namespace": namespace}}}
}

func managedObjectMatches(actual, desired map[string]any) bool {
	if actual == nil || objectName(actual) != objectName(desired) {
		return false
	}
	metadata, _ := actual["metadata"].(map[string]any)
	labels, _ := metadata["labels"].(map[string]any)
	if labels["app.kubernetes.io/managed-by"] != "agenova" {
		return false
	}
	for _, key := range []string{"rules", "roleRef", "subjects"} {
		if value, ok := desired[key]; ok && !expectedFieldsMatch(actual[key], value) {
			return false
		}
	}
	return true
}

func serviceObject(namespace string) map[string]any {
	selector := map[string]any{"app.kubernetes.io/name": controlPlaneName, "app.kubernetes.io/managed-by": "agenova"}
	return map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": controlPlaneName, "namespace": namespace, "labels": managedLabels()}, "spec": map[string]any{"type": "ClusterIP", "selector": selector, "ports": []any{map[string]any{"name": "http", "port": 8080, "targetPort": "http"}}}}
}

func managedLabels() map[string]any {
	return map[string]any{"app.kubernetes.io/managed-by": "agenova", "app.kubernetes.io/part-of": "agenova"}
}

func referencePolicyJSON() ([]byte, error) {
	return json.Marshal(policy.ReferenceBundle())
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
	if !managedDeploymentSpecMatches(object, desired) {
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

func managedDeploymentSpecMatches(object, desired map[string]any) bool {
	data, err := json.Marshal(desired["spec"])
	if err != nil {
		return false
	}
	var normalized any
	return json.Unmarshal(data, &normalized) == nil && managedFieldsMatch(object["spec"], normalized, "deploymentSpec")
}

// The pod spec is Agenova-owned. Only known API-server defaults may appear
// outside the manifest, including inside nested container/probe maps.
func managedFieldsMatch(actual, desired any, path string) bool {
	switch wanted := desired.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range wanted {
			if !managedFieldsMatch(got[key], value, path+"/"+key) {
				return false
			}
		}
		defaults := managedKubernetesDefaults(path)
		for key, value := range got {
			if _, owned := wanted[key]; owned {
				continue
			}
			if defaultValue, known := defaults[key]; !known || !reflect.DeepEqual(value, defaultValue) {
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
			if !managedFieldsMatch(got[index], wanted[index], path+"[]") {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, desired)
	}
}

func managedKubernetesDefaults(path string) map[string]any {
	if strings.HasPrefix(path, "deploymentSpec/template/spec") {
		path = "pod" + strings.TrimPrefix(path, "deploymentSpec/template/spec")
	}
	switch path {
	case "deploymentSpec":
		return map[string]any{"progressDeadlineSeconds": float64(600), "revisionHistoryLimit": float64(10), "strategy": map[string]any{"type": "RollingUpdate", "rollingUpdate": map[string]any{"maxSurge": "25%", "maxUnavailable": "25%"}}}
	case "pod":
		return map[string]any{"dnsPolicy": "ClusterFirst", "restartPolicy": "Always", "schedulerName": "default-scheduler", "securityContext": map[string]any{}, "terminationGracePeriodSeconds": float64(30)}
	case "pod/containers[]":
		return map[string]any{"resources": map[string]any{}, "terminationMessagePath": "/dev/termination-log", "terminationMessagePolicy": "File"}
	case "pod/containers[]/ports[]":
		return map[string]any{"protocol": "TCP"}
	case "pod/containers[]/readinessProbe":
		return map[string]any{"failureThreshold": float64(3), "successThreshold": float64(1), "timeoutSeconds": float64(1)}
	case "pod/containers[]/readinessProbe/httpGet":
		return map[string]any{"scheme": "HTTP"}
	case "pod/volumes[]/configMap", "pod/volumes[]/projected":
		return map[string]any{"defaultMode": float64(420)}
	default:
		return nil
	}
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
	statuses := []platformapply.ComponentStatus{{Name: platformRecord, Category: "deployment", State: "failed", Reference: revision}, {Name: controlPlaneName, Category: "deployment", State: "failed"}, {Name: controlPlaneName + "-service", Category: "deployment", State: "failed"}, {Name: policyRecord, Category: "policy", State: "failed"}, {Name: activePolicyRecord, Category: "policy", State: "failed"}}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses
}
