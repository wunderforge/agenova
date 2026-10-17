// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.(?:[a-z0-9.-]+)/(?:deployment|runtime|model|tool|memory|telemetry)/[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-z0-9.-]+)?$`)
	namePattern    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
)

type Registry struct {
	byKey map[string]Registration
}

func New(registrations ...Registration) (*Registry, error) {
	validated := make(map[string]Registration, len(registrations))
	for i, registration := range registrations {
		copy, err := validateRegistration(registration)
		if err != nil {
			return nil, fmt.Errorf("registration[%d]: %w", i, err)
		}
		key := registryKey(copy.Manifest.ID, copy.Manifest.Version)
		if _, exists := validated[key]; exists {
			return nil, fmt.Errorf("registration[%d]: duplicate adapter %s@%s", i, copy.Manifest.ID, copy.Manifest.Version)
		}
		validated[key] = copy
	}
	return &Registry{byKey: validated}, nil
}

func (r *Registry) Catalog() []Manifest {
	if r == nil {
		return []Manifest{}
	}
	result := make([]Manifest, 0, len(r.byKey))
	for _, registration := range r.byKey {
		result = append(result, cloneManifest(registration.Manifest))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID == result[j].ID {
			return result[i].Version < result[j].Version
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// Lookup implements platform.DescriptorLookup.
func (r *Registry) Lookup(id, version string) (platform.Descriptor, bool) {
	registration, ok := r.registration(id, version)
	if !ok {
		return platform.Descriptor{}, false
	}
	return cloneDescriptor(registration.Descriptor), true
}

func (r *Registry) Resolve(reference string) (Registration, error) {
	id, version := splitReference(reference)
	if id == "" {
		return Registration{}, fmt.Errorf("adapter reference is required")
	}
	if version != "" {
		if err := validateIdentity(id, version); err != nil {
			return Registration{}, err
		}
		if registration, ok := r.registration(id, version); ok {
			return cloneRegistration(registration), nil
		}
		return Registration{}, fmt.Errorf("adapter %s@%s is not available", id, version)
	}
	if !idPattern.MatchString(id) {
		return Registration{}, fmt.Errorf("invalid qualified adapter ID %q", id)
	}
	var matches []Registration
	for _, registration := range r.byKey {
		if registration.Manifest.ID == id {
			matches = append(matches, registration)
		}
	}
	if len(matches) == 0 {
		return Registration{}, fmt.Errorf("adapter %s is not available", id)
	}
	if len(matches) > 1 {
		return Registration{}, fmt.Errorf("adapter %s has multiple available versions; use id@version", id)
	}
	return cloneRegistration(matches[0]), nil
}

func (r *Registry) Construct(id, version string, capability platform.Capability) (any, error) {
	registration, ok := r.registration(id, version)
	if !ok {
		return nil, fmt.Errorf("adapter %s@%s is not available", id, version)
	}
	factory, ok := registration.Factories[capability]
	if !ok {
		return nil, fmt.Errorf("adapter %s@%s does not provide %s", id, version, capability)
	}
	implementation, err := factory()
	if err != nil {
		return nil, fmt.Errorf("construct %s adapter %s@%s: factory failed", capability, id, version)
	}
	if implementation == nil || isNilImplementation(implementation) {
		return nil, fmt.Errorf("construct %s adapter %s@%s: factory returned no implementation", capability, id, version)
	}
	return implementation, nil
}

func (r *Registry) registration(id, version string) (Registration, bool) {
	if r == nil {
		return Registration{}, false
	}
	registration, ok := r.byKey[registryKey(id, version)]
	return registration, ok
}

func validateRegistration(input Registration) (Registration, error) {
	manifest := cloneManifest(input.Manifest)
	if err := validateIdentity(manifest.ID, manifest.Version); err != nil {
		return Registration{}, err
	}
	if manifest.Protocol != ProtocolVersion {
		return Registration{}, fmt.Errorf("adapter %s@%s uses unsupported protocol %q", manifest.ID, manifest.Version, manifest.Protocol)
	}
	if len(manifest.Capabilities) == 0 {
		return Registration{}, fmt.Errorf("adapter %s@%s declares no capabilities", manifest.ID, manifest.Version)
	}
	manifest.Capabilities = sortedCapabilities(manifest.Capabilities)
	for i, capability := range manifest.Capabilities {
		if i > 0 && capability == manifest.Capabilities[i-1] {
			return Registration{}, fmt.Errorf("adapter %s@%s repeats capability %q", manifest.ID, manifest.Version, capability)
		}
		if capability != platform.CapabilityDeployment && capability != platform.CapabilityRuntime && capability != platform.CapabilityModel && capability != platform.CapabilityTool {
			return Registration{}, fmt.Errorf("adapter %s@%s advertises unsupported capability %q", manifest.ID, manifest.Version, capability)
		}
		if input.Factories[capability] == nil {
			return Registration{}, fmt.Errorf("adapter %s@%s has no factory for %q", manifest.ID, manifest.Version, capability)
		}
	}
	category, categoryErr := identityCapability(manifest.ID)
	if categoryErr != nil {
		return Registration{}, categoryErr
	}
	if !containsCapability(manifest.Capabilities, category) {
		return Registration{}, fmt.Errorf("adapter ID category %q is not among advertised capabilities", category)
	}
	if len(input.Factories) != len(manifest.Capabilities) {
		return Registration{}, fmt.Errorf("adapter %s@%s has factories outside its advertised capabilities", manifest.ID, manifest.Version)
	}
	if input.Descriptor.ID != manifest.ID || input.Descriptor.Version != manifest.Version {
		return Registration{}, fmt.Errorf("descriptor identity/version does not match manifest")
	}
	if !sameCapabilities(input.Descriptor.Capabilities, manifest.Capabilities) {
		return Registration{}, fmt.Errorf("descriptor capabilities do not match manifest")
	}
	if input.Descriptor.CanonicalizeInstance == nil {
		return Registration{}, fmt.Errorf("adapter %s@%s has no instance canonicalizer", manifest.ID, manifest.Version)
	}
	if err := validateSchema(manifest.InstanceSchema, "instanceSchema"); err != nil {
		return Registration{}, err
	}
	if containsCapability(manifest.Capabilities, platform.CapabilityTool) && input.Descriptor.DescribeTool == nil {
		return Registration{}, fmt.Errorf("tool adapter has no catalog descriptor")
	}
	needsProfile := containsCapability(manifest.Capabilities, platform.CapabilityRuntime) || containsCapability(manifest.Capabilities, platform.CapabilityModel) || containsCapability(manifest.Capabilities, platform.CapabilityTool)
	if needsProfile && input.Descriptor.CanonicalizeProfile == nil {
		return Registration{}, fmt.Errorf("adapter %s@%s has no profile canonicalizer", manifest.ID, manifest.Version)
	}
	if needsProfile {
		if err := validateSchema(manifest.ProfileSchema, "profileSchema"); err != nil {
			return Registration{}, err
		}
	} else if len(manifest.ProfileSchema.Fields) != 0 {
		return Registration{}, fmt.Errorf("adapter %s@%s cannot declare a profile schema without runtime, model or tool capability", manifest.ID, manifest.Version)
	}
	factories := make(map[platform.Capability]Factory, len(input.Factories))
	for capability, factory := range input.Factories {
		factories[capability] = factory
	}
	return Registration{Manifest: manifest, Descriptor: cloneDescriptor(input.Descriptor), Factories: factories}, nil
}

func validateSchema(schema ConfigSchema, label string) error {
	seen := map[string]struct{}{}
	defaults := map[string]any{}
	probe := map[string]any{}
	for i, field := range schema.Fields {
		if !validFieldPath(field.Path) {
			return fmt.Errorf("%s.fields[%d] has invalid path %q", label, i, field.Path)
		}
		if _, exists := seen[field.Path]; exists {
			return fmt.Errorf("%s repeats field %q", label, field.Path)
		}
		seen[field.Path] = struct{}{}
		if field.Kind != ValueString && field.Kind != ValueBoolean && field.Kind != ValueInteger {
			return fmt.Errorf("%s field %q has unsupported kind %q", label, field.Path, field.Kind)
		}
		if err := setPath(probe, field.Path, probeValue(field.Kind)); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if field.Default == nil {
			if field.Required {
				return fmt.Errorf("%s required field %q needs an initialization default", label, field.Path)
			}
			continue
		}
		if !defaultMatchesKind(field.Default, field.Kind) {
			return fmt.Errorf("%s field %q default does not match %s", label, field.Path, field.Kind)
		}
		if err := setPath(defaults, field.Path, field.Default); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if validationErr := v1alpha1.ValidatePlatformConfig(defaults); validationErr != nil {
		return fmt.Errorf("%s contains forbidden config: %s", label, validationErr.Error())
	}
	if validationErr := v1alpha1.ValidatePlatformConfig(probe); validationErr != nil {
		return fmt.Errorf("%s contains forbidden field: %s", label, validationErr.Error())
	}
	return nil
}

func probeValue(kind ValueKind) any {
	switch kind {
	case ValueBoolean:
		return false
	case ValueInteger:
		return 0
	default:
		return "configured"
	}
}

func validFieldPath(path string) bool {
	if path == "" {
		return false
	}
	for _, part := range strings.Split(path, ".") {
		if !namePattern.MatchString(part) {
			return false
		}
	}
	return true
}

func defaultMatchesKind(value any, kind ValueKind) bool {
	switch kind {
	case ValueString:
		_, ok := value.(string)
		return ok
	case ValueBoolean:
		_, ok := value.(bool)
		return ok
	case ValueInteger:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func validateIdentity(id, version string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid qualified adapter ID %q", id)
	}
	if len(id) > 256 {
		return fmt.Errorf("qualified adapter ID exceeds 256 characters")
	}
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("invalid adapter version %q", version)
	}
	if len(version) > 64 {
		return fmt.Errorf("adapter version exceeds 64 characters")
	}
	return nil
}

func identityCapability(id string) (platform.Capability, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid qualified adapter ID %q", id)
	}
	capability := platform.Capability(parts[1])
	if capability != platform.CapabilityDeployment && capability != platform.CapabilityRuntime && capability != platform.CapabilityModel && capability != platform.CapabilityTool {
		return "", fmt.Errorf("adapter ID category %q is not supported", parts[1])
	}
	return capability, nil
}

func isNilImplementation(value any) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func splitReference(reference string) (string, string) {
	reference = strings.TrimSpace(reference)
	index := strings.LastIndex(reference, "@")
	if index <= 0 || index == len(reference)-1 {
		return reference, ""
	}
	return reference[:index], reference[index+1:]
}

func registryKey(id, version string) string { return id + "\x00" + version }

func sameCapabilities(left, right []platform.Capability) bool {
	left = sortedCapabilities(left)
	right = sortedCapabilities(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func containsCapability(values []platform.Capability, target platform.Capability) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneManifest(input Manifest) Manifest {
	result := input
	result.Capabilities = append([]platform.Capability(nil), input.Capabilities...)
	result.InstanceSchema.Fields = append([]Field(nil), input.InstanceSchema.Fields...)
	result.ProfileSchema.Fields = append([]Field(nil), input.ProfileSchema.Fields...)
	return result
}

func cloneDescriptor(input platform.Descriptor) platform.Descriptor {
	result := input
	result.Capabilities = append([]platform.Capability(nil), input.Capabilities...)
	return result
}

func cloneRegistration(input Registration) Registration {
	factories := make(map[platform.Capability]Factory, len(input.Factories))
	for capability, factory := range input.Factories {
		factories[capability] = factory
	}
	return Registration{Manifest: cloneManifest(input.Manifest), Descriptor: cloneDescriptor(input.Descriptor), Factories: factories}
}
