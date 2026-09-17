// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package bundled explicitly registers adapters shipped with Agenova.
package bundled

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
)

const (
	KubernetesDeploymentID  = "agenova.io/deployment/kubernetes"
	AgentSandboxRuntimeID   = "agenova.io/runtime/agent-sandbox"
	OpenAICompatibleModelID = "agenova.io/model/openai-compatible"
	ReferenceVersion        = "0.1.0"
	// This reference distribution includes only the controlled demo worker.
	// Other images need their own reviewed compatibility contract/adapter.
	referenceControlledWorkerImage = "agenova-testworker:kind"
)

// These types are capability-owned construction results. #45 can define the
// narrow operational interfaces it needs without a universal adapter API.
type KubernetesDeployment struct{ runner kubectlRunner }
type AgentSandboxRuntime struct{}
type OpenAICompatibleModel struct{}

func NewRegistry() (*adapterregistry.Registry, error) {
	return adapterregistry.New(
		kubernetesDeploymentRegistration(),
		agentSandboxRuntimeRegistration(),
		openAICompatibleModelRegistration(),
	)
}

func kubernetesDeploymentRegistration() adapterregistry.Registration {
	manifest := adapterregistry.Manifest{
		ID: KubernetesDeploymentID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion,
		Capabilities: []platform.Capability{platform.CapabilityDeployment},
		InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			{Path: "context", Kind: adapterregistry.ValueString, Required: true, Description: "Existing Kubernetes context selected by the operator", Default: "kind-agenova"},
			{Path: "namespace", Kind: adapterregistry.ValueString, Required: true, Description: "Namespace for Agenova control-plane resources", Default: "agenova-system"},
		}},
	}
	return adapterregistry.Registration{
		Manifest: manifest,
		Descriptor: platform.Descriptor{
			ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities,
			CanonicalizeInstance: canonicalizeKubernetesDeployment,
		},
		Factories: map[platform.Capability]adapterregistry.Factory{
			platform.CapabilityDeployment: func() (any, error) { return newKubernetesDeployment(nil), nil },
		},
	}
}

func agentSandboxRuntimeRegistration() adapterregistry.Registration {
	manifest := adapterregistry.Manifest{
		ID: AgentSandboxRuntimeID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion,
		Capabilities: []platform.Capability{platform.CapabilityRuntime},
		InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			{Path: "connection.mode", Kind: adapterregistry.ValueString, Required: true, Description: "Control-plane-to-runtime connection mode", Default: "in-cluster"},
			{Path: "connection.namespace", Kind: adapterregistry.ValueString, Required: true, Description: "Namespace containing Agent Sandbox resources", Default: "agenova-system"},
			{Path: "compatible-worker-image", Kind: adapterregistry.ValueString, Required: true, Description: "Reference worker image allowed for this runtime", Default: referenceControlledWorkerImage},
		}},
		ProfileSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			{Path: "isolation", Kind: adapterregistry.ValueString, Required: true, Description: "Requested supported isolation shape", Default: "dedicated"},
		}},
	}
	return adapterregistry.Registration{
		Manifest: manifest,
		Descriptor: platform.Descriptor{
			ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities,
			CanonicalizeInstance: canonicalizeAgentSandboxInstance,
			CanonicalizeProfile:  canonicalizeAgentSandboxProfile,
		},
		Factories: map[platform.Capability]adapterregistry.Factory{
			platform.CapabilityRuntime: func() (any, error) { return &AgentSandboxRuntime{}, nil },
		},
	}
}

func openAICompatibleModelRegistration() adapterregistry.Registration {
	manifest := adapterregistry.Manifest{
		ID: OpenAICompatibleModelID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion,
		Capabilities: []platform.Capability{platform.CapabilityModel},
		InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			{Path: "endpoint", Kind: adapterregistry.ValueString, Required: true, Description: "OpenAI-compatible backend endpoint behind Agenova Model Gateway", Default: "http://host.docker.internal:11434/v1"},
		}},
		ProfileSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			{Path: "model", Kind: adapterregistry.ValueString, Required: true, Description: "Backend model selected by this logical profile", Default: "qwen2.5:0.5b"},
		}},
	}
	return adapterregistry.Registration{
		Manifest: manifest,
		Descriptor: platform.Descriptor{
			ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities,
			CanonicalizeInstance: canonicalizeOpenAIInstance,
			CanonicalizeProfile:  canonicalizeOpenAIProfile,
		},
		Factories: map[platform.Capability]adapterregistry.Factory{
			platform.CapabilityModel: func() (any, error) { return &OpenAICompatibleModel{}, nil },
		},
	}
}

func canonicalizeKubernetesDeployment(_ platform.Capability, input map[string]any) (map[string]any, error) {
	if err := onlyKeys(input, "context", "namespace"); err != nil {
		return nil, err
	}
	context, err := requiredString(input, "context")
	if err != nil {
		return nil, err
	}
	namespace, err := requiredString(input, "namespace")
	if err != nil {
		return nil, err
	}
	return map[string]any{"context": context, "namespace": namespace}, nil
}

func canonicalizeAgentSandboxInstance(_ platform.Capability, input map[string]any) (map[string]any, error) {
	if err := onlyKeys(input, "connection", "compatible-worker-image"); err != nil {
		return nil, err
	}
	connection, ok := input["connection"].(map[string]any)
	if !ok {
		return nil, platform.NewAdapterConfigError("invalid-object", "connection")
	}
	if err := onlyKeys(connection, "mode", "namespace"); err != nil {
		return nil, err
	}
	mode, err := requiredString(connection, "mode")
	if err != nil {
		return nil, err
	}
	if mode != "in-cluster" {
		return nil, platform.NewAdapterConfigError("unsupported-connection-mode", "connection.mode")
	}
	namespace, err := requiredString(connection, "namespace")
	if err != nil {
		return nil, err
	}
	image, err := requiredString(input, "compatible-worker-image")
	if err != nil || image != referenceControlledWorkerImage {
		return nil, platform.NewAdapterConfigError("unsupported-worker-image", "compatible-worker-image")
	}
	return map[string]any{"connection": map[string]any{"mode": mode, "namespace": namespace}, "compatible-worker-image": image}, nil
}

func canonicalizeAgentSandboxProfile(_ platform.Capability, backend, input map[string]any) (map[string]any, error) {
	connection, ok := backend["connection"].(map[string]any)
	if !ok || connection["mode"] != "in-cluster" {
		return nil, platform.NewAdapterConfigError("unsupported-connection-mode", "connection.mode")
	}
	if err := onlyKeys(input, "isolation"); err != nil {
		return nil, err
	}
	isolation, err := requiredString(input, "isolation")
	if err != nil {
		return nil, err
	}
	if isolation != "dedicated" {
		return nil, platform.NewAdapterConfigError("unsupported-isolation", "isolation")
	}
	return map[string]any{"isolation": isolation}, nil
}

func canonicalizeOpenAIInstance(_ platform.Capability, input map[string]any) (map[string]any, error) {
	if err := onlyKeys(input, "endpoint"); err != nil {
		return nil, err
	}
	endpoint, err := requiredString(input, "endpoint")
	if err != nil {
		return nil, err
	}
	parsed, parseErr := url.Parse(endpoint)
	if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, platform.NewAdapterConfigError("invalid-endpoint", "endpoint")
	}
	// Match the installed provider's transport boundary during validation,
	// before any Kubernetes mutation. The local kind exception is exact-host.
	_, providerErr := modelprovider.New(modelprovider.Config{Endpoint: endpoint, AllowDockerHostHTTP: parsed.Hostname() == "host.docker.internal", Models: map[string]string{"validation": "validation"}})
	if providerErr != nil {
		return nil, platform.NewAdapterConfigError("unsupported-endpoint", "endpoint")
	}
	return map[string]any{"endpoint": strings.TrimRight(endpoint, "/")}, nil
}

func canonicalizeOpenAIProfile(_ platform.Capability, _ map[string]any, input map[string]any) (map[string]any, error) {
	if err := onlyKeys(input, "model"); err != nil {
		return nil, err
	}
	model, err := requiredString(input, "model")
	if err != nil {
		return nil, err
	}
	return map[string]any{"model": model}, nil
}

func requiredString(input map[string]any, key string) (string, error) {
	value, ok := input[key].(string)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", platform.NewAdapterConfigError("required-string", key)
	}
	return value, nil
}

func onlyKeys(input map[string]any, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range input {
		if _, ok := set[key]; !ok {
			return platform.NewAdapterConfigError("unknown-field", key)
		}
	}
	return nil
}

func DescribeImplementation(value any) string {
	switch value.(type) {
	case *KubernetesDeployment:
		return fmt.Sprintf("%s@%s", KubernetesDeploymentID, ReferenceVersion)
	case *AgentSandboxRuntime:
		return fmt.Sprintf("%s@%s", AgentSandboxRuntimeID, ReferenceVersion)
	case *OpenAICompatibleModel:
		return fmt.Sprintf("%s@%s", OpenAICompatibleModelID, ReferenceVersion)
	default:
		return "unknown"
	}
}
