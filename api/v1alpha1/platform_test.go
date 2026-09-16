// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPlatformContractParsesEquivalentYAMLAndJSON(t *testing.T) {
	yamlPlatform, validationErr := ParsePlatformYAML([]byte(validPlatformYAML))
	if validationErr != nil {
		t.Fatalf("ParsePlatformYAML() error = %v", validationErr)
	}
	encoded, err := json.Marshal(yamlPlatform)
	if err != nil {
		t.Fatal(err)
	}
	jsonPlatform, validationErr := ParsePlatformJSON(encoded)
	if validationErr != nil {
		t.Fatalf("ParsePlatformJSON() error = %v", validationErr)
	}
	if !reflect.DeepEqual(yamlPlatform, jsonPlatform) {
		t.Fatalf("YAML and JSON values differ:\nYAML=%#v\nJSON=%#v", yamlPlatform, jsonPlatform)
	}
}

func TestPlatformContractRejectsInvalidShapesAndReferences(t *testing.T) {
	tests := []struct {
		name     string
		document string
		category ValidationCategory
		path     string
	}{
		{"unknown field", validPlatformYAML + "status: {}\n", ValidationCategorySystemManagedField, "status"},
		{"unknown adapter", replaceOnce(validPlatformYAML, "adapterRef: agent-sandbox-runtime", "adapterRef: missing-runtime"), ValidationCategoryInvalidValue, "spec.infrastructure.runtimeBackends[0].adapterRef"},
		{"unknown backend", replaceOnce(validPlatformYAML, "backendRef: primary-runtime", "backendRef: missing-runtime"), ValidationCategoryInvalidValue, "spec.infrastructure.runtimeProfiles[0].backendRef"},
		{"credential reference", replaceOnce(validPlatformYAML, "endpoint: https://ollama.agenova-models.svc.cluster.local/v1", "endpoint: https://ollama.agenova-models.svc.cluster.local/v1\n          credentialRef: model-key"), ValidationCategorySecretValue, "spec.services.modelBackends[0].config.credentialRef"},
		{"direct secret", replaceOnce(validPlatformYAML, "model: llama3.1:latest", "model: llama3.1:latest\n          apiKey: forbidden"), ValidationCategorySecretValue, "spec.services.modelProfiles[0].config.apiKey"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParsePlatformYAML([]byte(test.document))
			if err == nil || err.Category != test.category || err.FieldPath != test.path {
				t.Fatalf("error = %#v, want category=%s path=%s", err, test.category, test.path)
			}
		})
	}
}

func TestPlatformContractRejectsMultipleDocumentsAndNonJSONConfig(t *testing.T) {
	if _, err := ParsePlatformYAML([]byte(validPlatformYAML + "---\n" + validPlatformYAML)); err == nil || err.Category != ValidationCategoryInvalidDocument {
		t.Fatalf("multiple documents error = %#v", err)
	}
	input := replaceOnce(validPlatformYAML, "namespace: agenova-workers", "namespace: !!binary YWdlbm92YQ==")
	if _, err := ParsePlatformYAML([]byte(input)); err == nil || err.Category != ValidationCategoryInvalidDocument {
		t.Fatalf("binary config error = %#v", err)
	}
}

func replaceOnce(input, old, replacement string) string {
	for i := 0; i+len(old) <= len(input); i++ {
		if input[i:i+len(old)] == old {
			return input[:i] + replacement + input[i+len(old):]
		}
	}
	return input
}

const validPlatformYAML = `apiVersion: agenova.io/v1alpha1
kind: Platform
metadata:
  name: reference-local
spec:
  adapters:
    - name: kubernetes-deployment
      id: agenova.io/deployment/kubernetes
      version: 0.1.0
    - name: agent-sandbox-runtime
      id: agenova.io/runtime/agent-sandbox
      version: 0.1.0
    - name: openai-compatible-backend
      id: agenova.io/model/openai-compatible
      version: 0.1.0
  infrastructure:
    deployment:
      name: control-plane
      adapterRef: kubernetes-deployment
      config:
        context: kind-agenova
        namespace: agenova-system
    runtimeBackends:
      - name: primary-runtime
        adapterRef: agent-sandbox-runtime
        config:
          connection:
            mode: in-cluster
            namespace: agenova-workers
    runtimeProfiles:
      - name: standard-isolated
        backendRef: primary-runtime
        config:
          isolation: dedicated
  services:
    modelBackends:
      - name: local-ollama
        adapterRef: openai-compatible-backend
        config:
          endpoint: https://ollama.agenova-models.svc.cluster.local/v1
    modelProfiles:
      - name: approved-coding-model
        backendRef: local-ollama
        config:
          model: llama3.1:latest
  initialPolicyRef:
    id: reference-default-deny
    version: "1"
`
