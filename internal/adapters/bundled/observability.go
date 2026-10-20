// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package bundled

import (
	"fmt"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"net/url"
	"regexp"
)

const OTLPAdapterID = "agenova.io/telemetry/otlp"

type OTLP struct{}

func otlpRegistration() adapterregistry.Registration {
	manifest := adapterregistry.Manifest{ID: OTLPAdapterID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion, Capabilities: []platform.Capability{platform.CapabilityTelemetry}, InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{{Path: "endpoint", Kind: adapterregistry.ValueString, Required: true, Description: "Host-side OTLP HTTP endpoint", Default: "http://agenova-otel:4318"}}}}
	return adapterregistry.Registration{Manifest: manifest, Descriptor: platform.Descriptor{ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities, CanonicalizeInstance: func(_ platform.Capability, input map[string]any) (map[string]any, error) {
		if err := onlyKeys(input, "endpoint"); err != nil {
			return nil, err
		}
		endpoint, err := requiredString(input, "endpoint")
		if err != nil {
			return nil, err
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, platform.NewAdapterConfigError("invalid-endpoint", "endpoint")
		}
		return map[string]any{"endpoint": endpoint}, nil
	}}, Factories: map[platform.Capability]adapterregistry.Factory{platform.CapabilityTelemetry: func() (any, error) { return &OTLP{}, nil }}}
}

var k8sName = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9.]{0,61}[a-z0-9])?$`)

func canonicalizeStack(input map[string]any) (map[string]any, error) {
	if err := onlyKeys(input, "storageClass", "grafanaAdminSecretRef"); err != nil {
		return nil, err
	}
	result := map[string]any{}
	for _, key := range []string{"storageClass", "grafanaAdminSecretRef"} {
		value, err := requiredString(input, key)
		if err != nil || !k8sName.MatchString(value) {
			return nil, platform.NewAdapterConfigError("invalid-reference", "observability."+key)
		}
		result[key] = value
	}
	return result, nil
}
func stackConfig(request platformapply.DeploymentRequest) (map[string]any, bool) {
	c, ok := request.Config["observability"].(map[string]any)
	return c, ok
}
func validateStackComposition(request platformapply.DeploymentRequest) error {
	_, managed := stackConfig(request)
	count := 0
	ids := map[string]string{}
	for _, a := range request.Platform.Adapters {
		ids[a.Name] = a.ID
	}
	for _, instance := range request.Platform.Instances {
		if instance.Category == platform.CapabilityTelemetry {
			count++
			if ids[instance.AdapterRef] != OTLPAdapterID {
				return fmt.Errorf("unsupported reference telemetry adapter")
			}
			if managed && instance.Config["endpoint"] != "http://agenova-otel:4318" {
				return fmt.Errorf("managed stack requires its internal OTLP endpoint")
			}
		}
	}
	if managed && count != 1 {
		return fmt.Errorf("managed observability requires one explicit OTLP service")
	}
	return nil
}
