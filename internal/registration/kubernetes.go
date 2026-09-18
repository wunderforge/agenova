// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

// KubernetesStore uses the operator's Kubernetes identity for registration.
// An empty context uses the pod's in-cluster kubeconfig. The transport is not
// an authentication shortcut and no management HTTP endpoint is exposed.
type KubernetesStore struct {
	Context   string
	Namespace string
	Kubectl   string
	Invoke    func(context.Context, []byte, ...string) ([]byte, error)
}

// New reference installs reserve one immutable slot atomically. Older
// reference installs used a name-hashed record; reads and idempotent reapply
// continue to recognize that record without creating a second slot.
const templateSlot = "agenova-template-slot"

func (s KubernetesStore) PutPolicy(bundle policy.PolicyBundle) (bool, error) {
	if err := policy.ValidateBundle(bundle); err != nil {
		return false, err
	}
	seed := policy.ReferenceBundle()
	if bundle.ID == seed.ID && bundle.Version == seed.Version && !EqualJSON(bundle, seed) {
		return false, ErrConflict
	}
	return s.put(recordName("policy", bundle.ID+"@"+bundle.Version), "policy.json", bundle)
}

// CanActivatePolicy checks the pointer's required management authority before
// an immutable policy record is created. This prevents an identity with only
// ConfigMap create rights from occupying a policy version it cannot activate.
func (s KubernetesStore) CanActivatePolicy(PolicyReference) error {
	current, err := s.run(nil, "get", "configmap", "agenova-active-policy", "-o", "json")
	if errors.Is(err, errMissingRecord) {
		return s.requireConfigMapMutation("create", "")
	}
	if err != nil {
		return err
	}
	var pointer struct {
		Metadata struct {
			ResourceVersion string            `json:"resourceVersion"`
			Labels          map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(current, &pointer); err != nil || pointer.Metadata.ResourceVersion == "" || pointer.Metadata.Labels["app.kubernetes.io/managed-by"] != "agenova" {
		return fmt.Errorf("active PolicyBundle pointer is not Agenova-managed")
	}
	return s.requireConfigMapMutation("patch", "agenova-active-policy")
}

func (s KubernetesStore) ActivatePolicy(ref PolicyReference) (bool, error) {
	if _, err := s.get(recordName("policy", ref.ID+"@"+ref.Version), "policy.json"); err != nil {
		return false, err
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return false, err
	}
	manifest := map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "agenova-active-policy", "namespace": s.Namespace, "labels": managedLabels()}, "data": map[string]string{"reference.json": string(data)}}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return false, err
	}
	current, err := s.run(nil, "get", "configmap", "agenova-active-policy", "-o", "json")
	if err != nil {
		if !errors.Is(err, errMissingRecord) {
			return false, err
		}
		_, err = s.run(encoded, "create", "-f", "-")
		return err == nil, err
	}
	var object struct {
		Metadata struct {
			ResourceVersion string            `json:"resourceVersion"`
			Labels          map[string]string `json:"labels"`
		} `json:"metadata"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(current, &object); err != nil {
		return false, fmt.Errorf("decode active PolicyBundle pointer: %w", err)
	}
	if object.Metadata.Labels["app.kubernetes.io/managed-by"] != "agenova" || object.Metadata.ResourceVersion == "" {
		return false, fmt.Errorf("active PolicyBundle pointer is not Agenova-managed")
	}
	if object.Data["reference.json"] == string(data) {
		if err := s.requireConfigMapMutation("patch", "agenova-active-policy"); err != nil {
			return false, err
		}
		return false, nil
	}
	patch, err := json.Marshal([]map[string]any{
		{"op": "test", "path": "/metadata/resourceVersion", "value": object.Metadata.ResourceVersion},
		{"op": "replace", "path": "/data/reference.json", "value": string(data)},
	})
	if err != nil {
		return false, err
	}
	_, err = s.run(nil, "patch", "configmap", "agenova-active-policy", "--type=json", "-p", string(patch))
	return err == nil, err
}

func (s KubernetesStore) ActivePolicy() (policy.PolicyBundle, error) {
	data, err := s.get("agenova-active-policy", "reference.json")
	if err != nil {
		return policy.PolicyBundle{}, err
	}
	var ref PolicyReference
	if err := json.Unmarshal(data, &ref); err != nil {
		return policy.PolicyBundle{}, fmt.Errorf("decode active PolicyBundle reference: %w", err)
	}
	data, err = s.get(recordName("policy", ref.ID+"@"+ref.Version), "policy.json")
	if err != nil {
		return policy.PolicyBundle{}, err
	}
	var bundle policy.PolicyBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return policy.PolicyBundle{}, fmt.Errorf("decode registered PolicyBundle: %w", err)
	}
	if err := policy.ValidateBundle(bundle); err != nil {
		return policy.PolicyBundle{}, err
	}
	if bundle.ID != ref.ID || bundle.Version != ref.Version {
		return policy.PolicyBundle{}, fmt.Errorf("active PolicyBundle record identity mismatch")
	}
	return bundle, nil
}

func (s KubernetesStore) PutTemplate(template *v0.AgentTemplate) (bool, error) {
	if err := v0.ValidateAgentTemplate(template); err != nil {
		return false, err
	}
	// This reference installation has one Portal template slot. Reject a
	// second name before creating an immutable record that would make setup
	// unavailable. General multi-template selection belongs to #147.
	templates, err := s.Templates()
	if err != nil {
		return false, err
	}
	for _, existing := range templates {
		if existing.Metadata.Name != template.Metadata.Name {
			return false, fmt.Errorf("reference installation supports one AgentTemplate; %s is already registered", existing.Metadata.Name)
		}
	}
	if len(templates) == 0 {
		// Two concurrent first registrations race on this one Kubernetes name;
		// create-or-equal admits at most one identity.
		return s.put(templateSlot, "template.json", template)
	}
	if _, err := s.get(templateSlot, "template.json"); err == nil {
		return s.put(templateSlot, "template.json", template)
	} else if !errors.Is(err, errMissingRecord) {
		return false, err
	}
	return s.put(recordName("template", template.Metadata.Name), "template.json", template)
}

func (s KubernetesStore) Template(name string) (*v0.AgentTemplate, error) {
	data, err := s.get(templateSlot, "template.json")
	if errors.Is(err, errMissingRecord) {
		data, err = s.get(recordName("template", name), "template.json")
	}
	if err != nil {
		return nil, err
	}
	var template v0.AgentTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("decode registered AgentTemplate: %w", err)
	}
	if err := v0.ValidateAgentTemplate(&template); err != nil {
		return nil, err
	}
	if template.Metadata.Name != name {
		return nil, fmt.Errorf("registered AgentTemplate identity mismatch")
	}
	return &template, nil
}

func (s KubernetesStore) Lookup(name string) (*v0.AgentTemplate, error) { return s.Template(name) }

// Templates returns only validated, Agenova-managed template records.
func (s KubernetesStore) Templates() ([]*v0.AgentTemplate, error) {
	output, err := s.run(nil, "get", "configmap", "-l", "app.kubernetes.io/managed-by=agenova,app.kubernetes.io/part-of=agenova", "-o", "json")
	if err != nil {
		return nil, err
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, fmt.Errorf("decode registered AgentTemplate list: %w", err)
	}
	templates := make([]*v0.AgentTemplate, 0)
	for _, item := range list.Items {
		if !strings.HasPrefix(item.Metadata.Name, "agenova-template-") {
			continue
		}
		data, ok := item.Data["template.json"]
		if !ok {
			return nil, fmt.Errorf("registered AgentTemplate record is incomplete")
		}
		var template v0.AgentTemplate
		if err := json.Unmarshal([]byte(data), &template); err != nil {
			return nil, fmt.Errorf("decode registered AgentTemplate: %w", err)
		}
		if err := v0.ValidateAgentTemplate(&template); err != nil {
			return nil, err
		}
		if item.Metadata.Name != templateSlot && item.Metadata.Name != recordName("template", template.Metadata.Name) {
			return nil, fmt.Errorf("registered AgentTemplate identity mismatch")
		}
		templates = append(templates, &template)
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Metadata.Name < templates[j].Metadata.Name })
	return templates, nil
}

func (s KubernetesStore) put(name, key string, value any) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	if previous, err := s.get(name, key); err == nil {
		if !jsonEqual(previous, data) {
			return false, ErrConflict
		}
		if err := s.requireConfigMapMutation("patch", name); err != nil {
			return false, err
		}
		return false, nil
	} else if !errors.Is(err, errMissingRecord) {
		return false, err
	}
	manifest := map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": name, "namespace": s.Namespace, "labels": managedLabels()}, "data": map[string]string{key: string(data)}}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return false, err
	}
	if _, err := s.run(encoded, "create", "-f", "-"); err != nil {
		// A concurrent creator may have won. Compare rather than replacing it.
		if previous, readErr := s.get(name, key); readErr == nil {
			if jsonEqual(previous, data) {
				if authErr := s.requireConfigMapMutation("patch", name); authErr != nil {
					return false, authErr
				}
				return false, nil
			}
			return false, ErrConflict
		}
		return false, err
	}
	return true, nil
}

// requireConfigMapMutation makes idempotent management actions prove the
// operator has the corresponding write authority. A read-only identity must
// not be able to reapply an identical record merely because no patch is
// needed after comparison.
func (s KubernetesStore) requireConfigMapMutation(verb, name string) error {
	resource := "configmaps"
	if name != "" {
		resource += "/" + name
	}
	output, err := s.run(nil, "auth", "can-i", verb, resource)
	if err != nil {
		return fmt.Errorf("verify ConfigMap %s authority: %w", verb, err)
	}
	if strings.TrimSpace(strings.ToLower(string(output))) != "yes" {
		return fmt.Errorf("operator lacks ConfigMap %s authority", verb)
	}
	return nil
}

var errMissingRecord = errors.New("registration record not found")

func (s KubernetesStore) get(name, key string) ([]byte, error) {
	output, err := s.run(nil, "get", "configmap", name, "-o", "json")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errMissingRecord
		}
		return nil, err
	}
	var object struct {
		Data     map[string]string `json:"data"`
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(output, &object); err != nil {
		return nil, fmt.Errorf("decode Kubernetes registration record: %w", err)
	}
	if object.Metadata.Labels["app.kubernetes.io/managed-by"] != "agenova" {
		return nil, fmt.Errorf("registration record %s is not Agenova-managed", name)
	}
	value, ok := object.Data[key]
	if !ok {
		return nil, fmt.Errorf("registration record %s lacks %s", name, key)
	}
	return []byte(value), nil
}

func (s KubernetesStore) run(input []byte, args ...string) ([]byte, error) {
	if s.Namespace == "" || s.Namespace == "default" {
		return nil, fmt.Errorf("explicit non-default registration namespace is required")
	}
	path := s.Kubectl
	if path == "" {
		path = "kubectl"
	}
	commandArgs := []string{}
	if s.Context != "" {
		commandArgs = append(commandArgs, "--context", s.Context)
	}
	commandArgs = append(commandArgs, "--namespace", s.Namespace)
	commandArgs = append(commandArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if s.Invoke != nil {
		return s.Invoke(ctx, input, commandArgs...)
	}
	cmd := exec.CommandContext(ctx, path, commandArgs...)
	cmd.WaitDelay = time.Second
	if input != nil {
		cmd.Stdin = strings.NewReader(string(input))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "notfound") || strings.Contains(strings.ToLower(string(output)), "not found") {
			return nil, errMissingRecord
		}
		return nil, fmt.Errorf("kubectl registration command failed; verify cluster access and namespace RBAC")
	}
	return output, nil
}

func recordName(kind, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "agenova-" + kind + "-" + hex.EncodeToString(sum[:12])
}

func jsonEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && EqualJSON(a, b)
}

func managedLabels() map[string]string {
	return map[string]string{"app.kubernetes.io/managed-by": "agenova", "app.kubernetes.io/part-of": "agenova"}
}
