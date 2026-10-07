// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
	"strings"
	"time"
)

const BedrockModelID = "agenova.io/model/bedrock"

type BedrockModel struct{}

func bedrockModelRegistration() adapterregistry.Registration {
	manifest := adapterregistry.Manifest{ID: BedrockModelID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion, Capabilities: []platform.Capability{platform.CapabilityModel}, InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{{Path: "region", Kind: adapterregistry.ValueString, Required: true, Description: "AWS region for trusted server-side Bedrock inference", Default: "ap-southeast-2"}}}, ProfileSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{{Path: "model", Kind: adapterregistry.ValueString, Required: true, Description: "Operator-selected Bedrock model ID", Default: "amazon.nova-micro-v1:0"}}}}
	return adapterregistry.Registration{Manifest: manifest, Descriptor: platform.Descriptor{ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities, CanonicalizeInstance: func(_ platform.Capability, input map[string]any) (map[string]any, error) {
		if err := onlyKeys(input, "region"); err != nil {
			return nil, err
		}
		region, err := requiredString(input, "region")
		if err != nil {
			return nil, err
		}
		if err := modelprovider.ValidateBedrockConfig(modelprovider.BedrockConfig{Region: region, Models: map[string]string{"validation": "validation"}}); err != nil {
			return nil, platform.NewAdapterConfigError("invalid-region", "region")
		}
		return map[string]any{"region": region}, nil
	}, CanonicalizeProfile: func(_ platform.Capability, backend, input map[string]any) (map[string]any, error) {
		result, err := canonicalizeOpenAIProfile(platform.CapabilityModel, backend, input)
		if err != nil {
			return nil, err
		}
		region, _ := backend["region"].(string)
		model, _ := result["model"].(string)
		if err := modelprovider.ValidateBedrockConfig(modelprovider.BedrockConfig{Region: region, Models: map[string]string{"validation": model}}); err != nil {
			return nil, platform.NewAdapterConfigError("invalid-model", "model")
		}
		return result, nil
	}}, Factories: map[platform.Capability]adapterregistry.Factory{platform.CapabilityModel: func() (any, error) { return &BedrockModel{}, nil }}}
}

// ModelComposition validates the installed service's single-provider binding
// without loading credentials or making external calls.
func ModelComposition(resolved *platform.ResolvedPlatform) (string, modelprovider.Config, modelprovider.BedrockConfig, error) {
	openai := modelprovider.Config{Models: map[string]string{}}
	bedrock := modelprovider.BedrockConfig{Models: map[string]string{}}
	if resolved == nil {
		return "", openai, bedrock, fmt.Errorf("resolved Platform is required")
	}
	ids := map[string]string{}
	for _, a := range resolved.Adapters {
		ids[a.Name] = a.ID
	}
	backends := map[string]platform.ResolvedInstance{}
	for _, b := range resolved.Instances {
		if b.Category == platform.CapabilityModel {
			backends[b.Name] = b
		}
	}
	selected, adapterID := "", ""
	for _, profile := range resolved.Profiles {
		if profile.Capability != platform.CapabilityModel {
			continue
		}
		b, ok := backends[profile.BackendRef]
		if !ok || (selected != "" && selected != b.Name) {
			return "", openai, bedrock, fmt.Errorf("installed service requires profiles on one model backend")
		}
		selected = b.Name
		adapterID = ids[b.AdapterRef]

		model, _ := profile.Config["model"].(string)
		switch adapterID {
		case OpenAICompatibleModelID:
			openai.Endpoint, _ = b.Config["endpoint"].(string)
			openai.Models[profile.Name] = model
		case BedrockModelID:
			bedrock.Region, _ = b.Config["region"].(string)
			bedrock.Models[profile.Name] = model
		default:
			return "", openai, bedrock, fmt.Errorf("installed service does not support selected model adapter")
		}
	}
	switch adapterID {
	case OpenAICompatibleModelID:
		openai.AllowDockerHostHTTP = strings.HasPrefix(openai.Endpoint, "http://host.docker.internal:")
		_, err := modelprovider.New(openai)
		return adapterID, openai, bedrock, err
	case BedrockModelID:
		return adapterID, openai, bedrock, modelprovider.ValidateBedrockConfig(bedrock)
	default:
		return "", openai, bedrock, fmt.Errorf("installed service requires model profiles")
	}
}

func NewInstalledModelClient(ctx context.Context, resolved *platform.ResolvedPlatform, schema json.RawMessage) (modelprovider.Client, error) {
	id, openai, bedrock, err := ModelComposition(resolved)
	if err != nil {
		return nil, err
	}
	if id == BedrockModelID {
		bedrock.MaxTokens = 512
		bedrock.Timeout = 2 * time.Minute
		bedrock.OutputSchema = schema
		return modelprovider.NewBedrock(ctx, bedrock)
	}
	openai.MaxTokens = 512
	openai.Timeout = 2 * time.Minute
	openai.OutputSchema = schema
	return modelprovider.New(openai)
}
