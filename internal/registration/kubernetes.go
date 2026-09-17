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

func (s KubernetesStore) ActivatePolicy(ref PolicyReference) error {
	if _, err := s.get(recordName("policy", ref.ID+"@"+ref.Version), "policy.json"); err != nil {
		return err
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	manifest := map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "agenova-active-policy", "namespace": s.Namespace, "labels": managedLabels()}, "data": map[string]string{"reference.json": string(data)}}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	current, err := s.run(nil, "get", "configmap", "agenova-active-policy", "-o", "json")
	if err != nil {
		if !errors.Is(err, errMissingRecord) {
			return err
		}
		_, err = s.run(encoded, "create", "-f", "-")
		return err
	}
	var object struct {
		Metadata struct {
			ResourceVersion string            `json:"resourceVersion"`
			Labels          map[string]string `json:"labels"`
		} `json:"metadata"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(current, &object); err != nil {
		return fmt.Errorf("decode active PolicyBundle pointer: %w", err)
	}
	if object.Metadata.Labels["app.kubernetes.io/managed-by"] != "agenova" || object.Metadata.ResourceVersion == "" {
		return fmt.Errorf("active PolicyBundle pointer is not Agenova-managed")
	}
	if object.Data["reference.json"] == string(data) {
		return nil
	}
	patch, err := json.Marshal([]map[string]any{
		{"op": "test", "path": "/metadata/resourceVersion", "value": object.Metadata.ResourceVersion},
		{"op": "replace", "path": "/data/reference.json", "value": string(data)},
	})
	if err != nil {
		return err
	}
	_, err = s.run(nil, "patch", "configmap", "agenova-active-policy", "--type=json", "-p", string(patch))
	return err
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
	return bundle, nil
}

func (s KubernetesStore) PutTemplate(template *v0.AgentTemplate) (bool, error) {
	if err := v0.ValidateAgentTemplate(template); err != nil {
		return false, err
	}
	return s.put(recordName("template", template.Metadata.Name), "template.json", template)
}

func (s KubernetesStore) Template(name string) (*v0.AgentTemplate, error) {
	data, err := s.get(recordName("template", name), "template.json")
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

func (s KubernetesStore) put(name, key string, value any) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	if previous, err := s.get(name, key); err == nil {
		if !jsonEqual(previous, data) {
			return false, ErrConflict
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
				return false, nil
			}
			return false, ErrConflict
		}
		return false, err
	}
	return true, nil
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
