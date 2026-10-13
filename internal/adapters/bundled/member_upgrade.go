// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"

	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
)

func memberBindingValue(key, value string) bool {
	switch key {
	case "deployment-name":
		return len(value) <= 63 && regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(value)
	case "deployment-uid":
		return regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`).MatchString(value)
	case "pod-template-sha256":
		return regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(value)
	}
	return false
}

// The binding deliberately excludes only the image being upgraded. Auth, Secret
// references, workload identity, probes and sidecars must remain byte-equivalent
// after JSON canonicalization. No Secret values are read or written here.
func memberTemplateBinding(object map[string]any) (string, int, string, error) {
	spec, _ := object["spec"].(map[string]any)
	strategy, _ := spec["strategy"].(map[string]any)
	if spec["replicas"] != float64(1) || strategy["type"] != "Recreate" {
		return "", 0, "", fmt.Errorf("member upgrade requires one replica and Recreate strategy")
	}
	template, _ := spec["template"].(map[string]any)
	raw, _ := json.Marshal(template)
	var copyTemplate map[string]any
	if json.Unmarshal(raw, &copyTemplate) != nil || copyTemplate == nil {
		return "", 0, "", fmt.Errorf("missing member Pod template")
	}
	pod, _ := copyTemplate["spec"].(map[string]any)
	containers, _ := pod["containers"].([]any)
	index := -1
	image := ""
	for i, item := range containers {
		c, _ := item.(map[string]any)
		if c["name"] != "control-plane" {
			continue
		}
		if index != -1 {
			return "", 0, "", fmt.Errorf("ambiguous control-plane container")
		}
		index = i
		image, _ = c["image"].(string)
		env, _ := c["env"].([]any)
		member, config := false, false
		for _, entry := range env {
			e, _ := entry.(map[string]any)
			if e["name"] == "AGENOVA_AUTH_MODE" && e["value"] == "member" {
				member = true
			}
			if e["name"] == "AGENOVA_MEMBER_CONFIG" && e["value"] != "" && e["value"] != nil {
				config = true
			}
		}
		if !member || !config || image == "" {
			return "", 0, "", fmt.Errorf("target is not an explicitly configured member control plane")
		}
		delete(c, "image")
	}
	if index < 0 {
		return "", 0, "", fmt.Errorf("control-plane container missing")
	}
	raw, _ = json.Marshal(copyTemplate)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), index, image, nil
}

func runtimeComposition(p platform.ResolvedPlatform) string {
	p.Revision = ""
	instances := make([]platform.ResolvedInstance, 0, len(p.Instances))
	for _, i := range p.Instances {
		if i.Category != platform.CapabilityDeployment {
			instances = append(instances, i)
		}
	}
	p.Instances = instances
	raw, _ := json.Marshal(p)
	return string(raw)
}

func (k *KubernetesDeployment) inspectMemberUpgrade(ctx context.Context, r platformapply.DeploymentRequest) (map[string]any, int, string, error) {
	contextName, namespace, err := deploymentCoordinates(r.Config)
	if err != nil {
		return nil, 0, "", err
	}
	if err = k.ValidateComposition(r); err != nil {
		return nil, 0, "", err
	}
	name, _ := r.Config["deployment-name"].(string)
	object, err := k.getJSON(ctx, contextName, namespace, "deployment", name)
	if err != nil {
		return nil, 0, "", err
	}
	meta, _ := object["metadata"].(map[string]any)
	if meta["uid"] != r.Config["deployment-uid"] || meta["resourceVersion"] == nil || meta["deletionTimestamp"] != nil {
		return nil, 0, "", fmt.Errorf("member Deployment identity changed or is terminating")
	}
	binding, index, image, err := memberTemplateBinding(object)
	if err != nil {
		return nil, 0, "", err
	}
	if binding != r.Config["pod-template-sha256"] {
		return nil, 0, "", fmt.Errorf("member Pod configuration drifted; review and renew the template binding")
	}
	record, err := k.getJSON(ctx, contextName, namespace, "configmap", platformRecord)
	if err != nil {
		return nil, 0, "", err
	}
	var installed platform.ResolvedPlatform
	if json.Unmarshal([]byte(objectData(record, "effective-platform.json")), &installed) != nil || runtimeComposition(installed) != runtimeComposition(*r.Platform) {
		return nil, 0, "", fmt.Errorf("member image upgrade cannot change installed runtime, models, policy or Platform identity")
	}
	return object, index, image, nil
}

func memberUpgradePlan(r platformapply.DeploymentRequest, object map[string]any, image string) ([]platformapply.Change, []platformapply.ComponentStatus) {
	name, _ := r.Config["deployment-name"].(string)
	desired, _ := r.Config["control-plane-image"].(string)
	changes := []platformapply.Change{}
	status, _ := object["status"].(map[string]any)
	meta, _ := object["metadata"].(map[string]any)
	ready := image == desired && status["observedGeneration"] == meta["generation"] && status["replicas"] == float64(1) && status["updatedReplicas"] == float64(1) && status["availableReplicas"] == float64(1)
	if image != desired {
		changes = append(changes, platformapply.Change{Component: name, Action: "update", Detail: "control-plane image: " + image + " -> " + desired})
	} else if !ready {
		changes = append(changes, platformapply.Change{Component: name, Action: "wait", Detail: "wait for member Deployment readiness"})
	}
	return changes, []platformapply.ComponentStatus{{Name: name, Category: "deployment", State: state(ready, "available", "not-ready"), Reference: desired}}
}

func (k *KubernetesDeployment) planMemberUpgrade(ctx context.Context, r platformapply.DeploymentRequest) (string, []platformapply.Change, []platformapply.ComponentStatus, error) {
	c, n, err := deploymentCoordinates(r.Config)
	if err != nil {
		return "", nil, nil, err
	}
	object, _, image, err := k.inspectMemberUpgrade(ctx, r)
	if err != nil {
		return platformapply.SafeTarget(c, n), nil, nil, err
	}
	changes, status := memberUpgradePlan(r, object, image)
	return platformapply.SafeTarget(c, n), changes, status, nil
}

func (k *KubernetesDeployment) preflightMemberUpgrade(ctx context.Context, r platformapply.DeploymentRequest) error {
	c, n, err := deploymentCoordinates(r.Config)
	if err != nil {
		return err
	}
	for _, verb := range []string{"get", "patch", "watch"} {
		if err = k.requireRBAC(ctx, c, verb, "deployments.apps", n); err != nil {
			return err
		}
	}
	return k.requireRBAC(ctx, c, "get", "configmaps", n)
}

func (k *KubernetesDeployment) applyMemberUpgrade(ctx context.Context, r platformapply.DeploymentRequest) ([]platformapply.ComponentStatus, bool, error) {
	if err := k.preflightMemberUpgrade(ctx, r); err != nil {
		return nil, false, err
	}
	object, index, image, err := k.inspectMemberUpgrade(ctx, r)
	if err != nil {
		return nil, false, err
	}
	changes, status := memberUpgradePlan(r, object, image)
	if r.TargetChanges != nil && !reflect.DeepEqual(changes, r.TargetChanges) {
		return status, false, fmt.Errorf("member Deployment changed since plan; review and retry")
	}
	c, n, _ := deploymentCoordinates(r.Config)
	name := r.Config["deployment-name"].(string)
	attempted := false
	if image != r.Config["control-plane-image"] {
		meta := object["metadata"].(map[string]any)
		path := "/spec/template/spec/containers/" + strconv.Itoa(index) + "/image"
		patch, _ := json.Marshal([]map[string]any{
			{"op": "test", "path": "/metadata/uid", "value": meta["uid"]},
			{"op": "test", "path": "/metadata/resourceVersion", "value": meta["resourceVersion"]},
			{"op": "test", "path": path, "value": image},
			{"op": "replace", "path": path, "value": r.Config["control-plane-image"]},
		})
		attempted = true
		if _, err = k.run(ctx, nil, "--context", c, "--namespace", n, "patch", "deployment", name, "--type=json", "-p", string(patch)); err != nil {
			return status, attempted, fmt.Errorf("member image patch failed; re-plan before retry: %w", err)
		}
	}
	if _, err = k.run(ctx, nil, "--context", c, "--namespace", n, "rollout", "status", "deployment/"+name, "--timeout=180s"); err != nil {
		return status, attempted, fmt.Errorf("member rollout not ready: %w", err)
	}
	_, remaining, status, err := k.planMemberUpgrade(ctx, r)
	if err == nil && len(remaining) > 0 {
		err = fmt.Errorf("member Deployment did not converge")
	}
	return status, attempted, err
}
