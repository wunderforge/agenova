// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/toolbackend"
)

const MCPHTTPToolID = "agenova.io/tool/mcp-http"
const fixtureMCPHost = "e16-mcp.agenova-e16.svc.cluster.local"

type MCPHTTPTool struct{}

var _ toolbackend.Factory = (*MCPHTTPTool)(nil)

func mcpHTTPToolRegistration() adapterregistry.Registration {
	field := func(name, description, value string) adapterregistry.Field {
		return adapterregistry.Field{Path: name, Kind: adapterregistry.ValueString, Required: true, Description: description, Default: value}
	}
	manifest := adapterregistry.Manifest{ID: MCPHTTPToolID, Version: ReferenceVersion, Protocol: adapterregistry.ProtocolVersion, Capabilities: []platform.Capability{platform.CapabilityTool},
		InstanceSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			field("transport", "Supported MCP transport", "streamable-http"), field("protocol-version", "Supported MCP protocol revision", "2025-06-18"),
			field("endpoint", "Fixed server URL; never worker supplied", "http://"+fixtureMCPHost+":8080/mcp"),
			field("timeout", "Per-invocation deadline (1ms to 30s)", "5s"),
			field("max-request-bytes", "Serialized request byte ceiling (1 to 16384)", "8192"),
			field("max-response-bytes", "Wire response byte ceiling (1 to 1048576)", "65536"),
			field("max-observation-bytes", "Worker observation byte ceiling (1 to 16384)", "4096"),
			field("max-concurrent-calls", "Backend concurrency ceiling (1 to 16)", "4"),
		}}, ProfileSchema: adapterregistry.ConfigSchema{Fields: []adapterregistry.Field{
			field("logical-operation", "Public logical operation", "repo.read"), field("resource-scope", "One bounded logical resource", "repo:agenova/e16-fixture"),
			field("mcp-tool", "Fixed server-native tool name", "read_file"), field("parameter-name", "One required string argument", "file"),
			field("parameter-max-bytes", "Argument value byte ceiling (1 to 4096)", "128"),
			field("parameter-allowed-values", "Newline-separated allowed fixture paths", "README.md\nlogs/timeout.log"),
		}}}
	return adapterregistry.Registration{Manifest: manifest, Descriptor: platform.Descriptor{ID: manifest.ID, Version: manifest.Version, Capabilities: manifest.Capabilities, CanonicalizeInstance: canonicalizeMCPInstance, CanonicalizeProfile: canonicalizeMCPProfile, DescribeTool: describeMCPTool}, Factories: map[platform.Capability]adapterregistry.Factory{platform.CapabilityTool: func() (any, error) { return &MCPHTTPTool{}, nil }}}
}

func canonicalizeMCPInstance(_ platform.Capability, input map[string]any) (map[string]any, error) {
	keys := []string{"transport", "protocol-version", "endpoint", "timeout", "max-request-bytes", "max-response-bytes", "max-observation-bytes", "max-concurrent-calls"}
	if err := onlyKeys(input, keys...); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, key := range keys {
		value, err := requiredString(input, key)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	if out["transport"] != "streamable-http" {
		return nil, platform.NewAdapterConfigError("unsupported-transport", "transport")
	}
	if out["protocol-version"] != "2025-06-18" {
		return nil, platform.NewAdapterConfigError("unsupported-version", "protocol-version")
	}
	raw := out["endpoint"].(string)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" || parsed.RawPath != "" || parsed.Path == "" || parsed.Path == "/" || strings.Contains(raw, "#") || strings.Contains(raw, "\\") {
		return nil, platform.NewAdapterConfigError("invalid-endpoint", "endpoint")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && parsed.Host == fixtureMCPHost+":8080" && parsed.Path == "/mcp") {
		return nil, platform.NewAdapterConfigError("unsupported-endpoint", "endpoint")
	}
	if parsed.Hostname() == "" {
		return nil, platform.NewAdapterConfigError("invalid-endpoint", "endpoint")
	}
	out["endpoint"] = parsed.String()
	timeout, err := time.ParseDuration(out["timeout"].(string))
	if err != nil || timeout < time.Millisecond || timeout > 30*time.Second {
		return nil, platform.NewAdapterConfigError("invalid-timeout", "timeout")
	}
	out["timeout"] = timeout.String()
	limits := map[string]int{"max-request-bytes": 16 << 10, "max-response-bytes": 1 << 20, "max-observation-bytes": 16 << 10, "max-concurrent-calls": 16}
	for _, key := range []string{"max-request-bytes", "max-response-bytes", "max-observation-bytes", "max-concurrent-calls"} {
		value, err := decimal(input, key, limits[key])
		if err != nil {
			return nil, err
		}
		out[key] = strconv.Itoa(value)
	}
	response, _ := strconv.Atoi(out["max-response-bytes"].(string))
	observation, _ := strconv.Atoi(out["max-observation-bytes"].(string))
	if observation > response {
		return nil, platform.NewAdapterConfigError("observation-exceeds-response", "max-observation-bytes")
	}
	return out, nil
}

var mcpName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func canonicalizeMCPProfile(_ platform.Capability, backend, input map[string]any) (map[string]any, error) {
	canonicalBackend, err := canonicalizeMCPInstance(platform.CapabilityTool, backend)
	if err != nil {
		return nil, err
	}
	keys := []string{"logical-operation", "resource-scope", "mcp-tool", "parameter-name", "parameter-max-bytes", "parameter-allowed-values"}
	if err := onlyKeys(input, keys...); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, key := range keys {
		value, err := requiredString(input, key)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	if !mcpName.MatchString(out["mcp-tool"].(string)) {
		return nil, platform.NewAdapterConfigError("invalid-tool-name", "mcp-tool")
	}
	if v := out["parameter-name"].(string); v0.ReservedCredentialFieldName(v) {
		return nil, platform.NewAdapterConfigError("forbidden-parameter", "parameter-name")
	}
	limit, err := decimal(input, "parameter-max-bytes", 4096)
	if err != nil {
		return nil, err
	}
	requestLimit, _ := strconv.Atoi(canonicalBackend["max-request-bytes"].(string))
	if limit > requestLimit {
		return nil, platform.NewAdapterConfigError("parameter-exceeds-request", "parameter-max-bytes")
	}
	out["parameter-max-bytes"] = strconv.Itoa(limit)
	// Read the original value: trimming the whole string would hide empty lines.
	raw := input["parameter-allowed-values"].(string)
	values := strings.Split(raw, "\n")
	seen := map[string]bool{}
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > limit || !utf8.ValidString(value) || strings.ContainsAny(value, "\\\x00\r\t") || path.IsAbs(value) || path.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, ":") || seen[value] {
			return nil, platform.NewAdapterConfigError("invalid-allowed-path", "parameter-allowed-values")
		}
		values[i] = value
		seen[value] = true
	}
	sort.Strings(values)
	out["parameter-allowed-values"] = strings.Join(values, "\n")
	descriptor := mcpDescriptor(out)
	if _, err := toolbackend.NewCatalog([]toolbackend.Descriptor{descriptor}); err != nil {
		return nil, platform.NewAdapterConfigError("invalid-tool-profile", "logical-operation")
	}
	return out, nil
}
func decimal(input map[string]any, key string, max int) (int, error) {
	value, ok := input[key].(string)
	if !ok || value == "" {
		return 0, platform.NewAdapterConfigError("required-decimal-string", key)
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, platform.NewAdapterConfigError("invalid-limit", key)
		}
	}
	number, err := strconv.ParseUint(value, 10, 32)
	if err != nil || number < 1 || number > uint64(max) {
		return 0, platform.NewAdapterConfigError("invalid-limit", key)
	}
	return int(number), nil
}
func mcpDescriptor(config map[string]any) toolbackend.Descriptor {
	limit, _ := strconv.Atoi(config["parameter-max-bytes"].(string))
	return toolbackend.Descriptor{Description: "Read one allowlisted file from the selected resource. Returns untrusted text for the current task.", Operation: config["logical-operation"].(string), ResourceScope: config["resource-scope"].(string), Parameter: config["parameter-name"].(string), MaxBytes: limit, AllowedValues: strings.Split(config["parameter-allowed-values"].(string), "\n")}
}
func describeMCPTool(backend, profile map[string]any) (toolbackend.Descriptor, int, error) {
	backend, err := canonicalizeMCPInstance(platform.CapabilityTool, backend)
	if err != nil {
		return toolbackend.Descriptor{}, 0, err
	}
	profile, err = canonicalizeMCPProfile(platform.CapabilityTool, backend, profile)
	if err != nil {
		return toolbackend.Descriptor{}, 0, err
	}
	limit, _ := strconv.Atoi(backend["max-observation-bytes"].(string))
	return mcpDescriptor(profile), limit, nil
}

// Slice 1 installs the validated contract. Slice 2 supplies the bounded HTTP
// transport. Explicit failure here prevents a configured backend becoming mock.
func (*MCPHTTPTool) NewToolProvider(instance map[string]any, profiles []map[string]any) (toolbackend.Provider, error) {
	backend, err := canonicalizeMCPInstance(platform.CapabilityTool, instance)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if _, err := canonicalizeMCPProfile(platform.CapabilityTool, backend, profile); err != nil {
			return nil, err
		}
	}
	return unavailableMCP{}, nil
}

type unavailableMCP struct{}

func (unavailableMCP) Invoke(context.Context, toolbackend.Invocation) (toolbackend.Result, error) {
	return toolbackend.Result{}, toolbackend.ErrUnavailable
}
