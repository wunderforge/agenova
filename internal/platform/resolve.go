// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package platform resolves the declarative Platform contract into a
// deterministic, secret-free plan. It has no mutation or backend dependency.
package platform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
)

const CoreModelGateway = "agenova-core"

type Capability string

const (
	CapabilityDeployment Capability = "deployment"
	CapabilityRuntime    Capability = "runtime"
	CapabilityModel      Capability = "model"
)

type ErrorCategory string

const (
	ErrorInvalidContract   ErrorCategory = "invalid-contract"
	ErrorUnknownAdapter    ErrorCategory = "unknown-adapter"
	ErrorIdentityMismatch  ErrorCategory = "identity-mismatch"
	ErrorCapability        ErrorCategory = "capability-mismatch"
	ErrorInvalidConfig     ErrorCategory = "invalid-config"
	ErrorCanonicalEncoding ErrorCategory = "canonical-encoding"
)

type ResolveError struct {
	Category  ErrorCategory
	FieldPath string
	Detail    string
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Category, e.FieldPath, e.Detail)
}

// Descriptor is metadata plus side-effect-free adapter-owned config logic.
// Pair canonicalization receives the referenced backend's canonical config.
type Descriptor struct {
	ID                   string
	Version              string
	Capabilities         []Capability
	CanonicalizeInstance func(Capability, map[string]any) (map[string]any, error)
	CanonicalizeProfile  func(Capability, map[string]any, map[string]any) (map[string]any, error)
}

type DescriptorLookup interface {
	Lookup(id, version string) (Descriptor, bool)
}

// AdapterConfigError carries only a stable developer-authored code and field
// path. Arbitrary adapter error strings are never returned to callers because
// they may contain rejected provider values or credentials.
type AdapterConfigError struct {
	code      string
	fieldPath string
}

func NewAdapterConfigError(code, fieldPath string) error {
	return &AdapterConfigError{code: safeDiagnosticToken(code), fieldPath: safeDiagnosticPath(fieldPath)}
}

func (e *AdapterConfigError) Error() string {
	return e.code + " at " + e.fieldPath
}

type ResolvedAdapter struct {
	Name         string       `json:"name"`
	ID           string       `json:"id"`
	Version      string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`
}

type ResolvedInstance struct {
	Category   Capability     `json:"category"`
	Name       string         `json:"name"`
	AdapterRef string         `json:"adapterRef"`
	Config     map[string]any `json:"config"`
}

type ResolvedProfile struct {
	Capability Capability     `json:"capability"`
	Name       string         `json:"name"`
	BackendRef string         `json:"backendRef"`
	Config     map[string]any `json:"config"`
}

type ModelRoute struct {
	Profile    string `json:"profile"`
	Gateway    string `json:"gateway"`
	BackendRef string `json:"backendRef"`
}

type ResolvedPlatform struct {
	PlatformName     string                           `json:"platformName"`
	Revision         string                           `json:"revision"`
	Adapters         []ResolvedAdapter                `json:"adapters"`
	Instances        []ResolvedInstance               `json:"instances"`
	Profiles         []ResolvedProfile                `json:"profiles"`
	ModelRoutes      []ModelRoute                     `json:"modelRoutes"`
	InitialPolicyRef v1alpha1.PlatformPolicyReference `json:"initialPolicyRef"`
}

type LockedConfig struct {
	Category     Capability `json:"category"`
	Name         string     `json:"name"`
	Reference    string     `json:"reference"`
	ConfigDigest string     `json:"configDigest"`
}

type PlatformLock struct {
	PlatformName     string                           `json:"platformName"`
	Revision         string                           `json:"revision"`
	Adapters         []ResolvedAdapter                `json:"adapters"`
	Instances        []LockedConfig                   `json:"instances"`
	Profiles         []LockedConfig                   `json:"profiles"`
	ModelRoutes      []ModelRoute                     `json:"modelRoutes"`
	InitialPolicyRef v1alpha1.PlatformPolicyReference `json:"initialPolicyRef"`
}

type resolvedRequirement struct {
	requirement v1alpha1.PlatformAdapterRequirement
	descriptor  Descriptor
}

// Resolve returns no partial state on any contract, reference, capability or
// adapter-owned validation failure.
func Resolve(input *v1alpha1.Platform, lookup DescriptorLookup) (*ResolvedPlatform, *PlatformLock, *ResolveError) {
	if err := v1alpha1.ValidatePlatform(input); err != nil {
		return nil, nil, &ResolveError{Category: ErrorInvalidContract, FieldPath: err.FieldPath, Detail: err.Error()}
	}
	if lookup == nil {
		return nil, nil, &ResolveError{Category: ErrorUnknownAdapter, FieldPath: "spec.adapters", Detail: "descriptor lookup is required"}
	}

	requirements := make(map[string]resolvedRequirement, len(input.Spec.Adapters))
	adapters := make([]ResolvedAdapter, 0, len(input.Spec.Adapters))
	for i, requirement := range input.Spec.Adapters {
		path := fmt.Sprintf("spec.adapters[%d]", i)
		descriptor, ok := lookup.Lookup(requirement.ID, requirement.Version)
		if !ok {
			return nil, nil, &ResolveError{Category: ErrorUnknownAdapter, FieldPath: path + ".id", Detail: "adapter implementation is unavailable"}
		}
		if descriptor.ID != requirement.ID || descriptor.Version != requirement.Version {
			return nil, nil, &ResolveError{Category: ErrorIdentityMismatch, FieldPath: path, Detail: "descriptor identity/version does not match the requirement"}
		}
		capabilities := append([]Capability(nil), descriptor.Capabilities...)
		sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
		requirements[requirement.Name] = resolvedRequirement{requirement: requirement, descriptor: descriptor}
		adapters = append(adapters, ResolvedAdapter{Name: requirement.Name, ID: requirement.ID, Version: requirement.Version, Capabilities: capabilities})
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Name < adapters[j].Name })

	instances := make([]ResolvedInstance, 0, 1+len(input.Spec.Infrastructure.RuntimeBackends)+len(input.Spec.Services.ModelBackends))
	instanceByKey := map[string]ResolvedInstance{}
	descriptorByKey := map[string]Descriptor{}
	if failure := resolveInstance(CapabilityDeployment, "spec.infrastructure.deployment", *input.Spec.Infrastructure.Deployment, requirements, &instances, instanceByKey, descriptorByKey); failure != nil {
		return nil, nil, failure
	}
	for i, instance := range input.Spec.Infrastructure.RuntimeBackends {
		if failure := resolveInstance(CapabilityRuntime, fmt.Sprintf("spec.infrastructure.runtimeBackends[%d]", i), instance, requirements, &instances, instanceByKey, descriptorByKey); failure != nil {
			return nil, nil, failure
		}
	}
	for i, instance := range input.Spec.Services.ModelBackends {
		if failure := resolveInstance(CapabilityModel, fmt.Sprintf("spec.services.modelBackends[%d]", i), instance, requirements, &instances, instanceByKey, descriptorByKey); failure != nil {
			return nil, nil, failure
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Category == instances[j].Category {
			return instances[i].Name < instances[j].Name
		}
		return instances[i].Category < instances[j].Category
	})

	profiles := make([]ResolvedProfile, 0, len(input.Spec.Infrastructure.RuntimeProfiles)+len(input.Spec.Services.ModelProfiles))
	modelRoutes := make([]ModelRoute, 0, len(input.Spec.Services.ModelProfiles))
	for i, profile := range input.Spec.Infrastructure.RuntimeProfiles {
		resolved, failure := resolveProfile(CapabilityRuntime, fmt.Sprintf("spec.infrastructure.runtimeProfiles[%d]", i), profile, instanceByKey, descriptorByKey)
		if failure != nil {
			return nil, nil, failure
		}
		profiles = append(profiles, resolved)
	}
	for i, profile := range input.Spec.Services.ModelProfiles {
		resolved, failure := resolveProfile(CapabilityModel, fmt.Sprintf("spec.services.modelProfiles[%d]", i), profile, instanceByKey, descriptorByKey)
		if failure != nil {
			return nil, nil, failure
		}
		profiles = append(profiles, resolved)
		modelRoutes = append(modelRoutes, ModelRoute{Profile: profile.Name, Gateway: CoreModelGateway, BackendRef: profile.BackendRef})
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Capability == profiles[j].Capability {
			return profiles[i].Name < profiles[j].Name
		}
		return profiles[i].Capability < profiles[j].Capability
	})
	sort.Slice(modelRoutes, func(i, j int) bool { return modelRoutes[i].Profile < modelRoutes[j].Profile })

	policy := *input.Spec.InitialPolicyRef
	payload := revisionPayload{PlatformName: input.Metadata.Name, Adapters: adapters, Instances: instances, Profiles: profiles, ModelRoutes: modelRoutes, InitialPolicyRef: policy}
	revision, err := digestValue(payload)
	if err != nil {
		return nil, nil, &ResolveError{Category: ErrorCanonicalEncoding, FieldPath: "$", Detail: err.Error()}
	}
	resolved := &ResolvedPlatform{PlatformName: input.Metadata.Name, Revision: revision, Adapters: adapters, Instances: instances, Profiles: profiles, ModelRoutes: modelRoutes, InitialPolicyRef: policy}
	lock, failure := lockFromResolved(resolved)
	if failure != nil {
		return nil, nil, failure
	}
	return resolved, lock, nil
}

func resolveInstance(capability Capability, path string, input v1alpha1.PlatformInstance, requirements map[string]resolvedRequirement, output *[]ResolvedInstance, byKey map[string]ResolvedInstance, descriptorByKey map[string]Descriptor) *ResolveError {
	requirement := requirements[input.AdapterRef]
	if !hasCapability(requirement.descriptor.Capabilities, capability) {
		return &ResolveError{Category: ErrorCapability, FieldPath: path + ".adapterRef", Detail: fmt.Sprintf("adapter does not provide %s capability", capability)}
	}
	if requirement.descriptor.CanonicalizeInstance == nil {
		return &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: "adapter has no instance canonicalizer"}
	}
	canonical, err := requirement.descriptor.CanonicalizeInstance(capability, cloneConfig(input.Config))
	if err != nil {
		return &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: safeAdapterErrorDetail(err)}
	}
	canonical = cloneConfig(canonical)
	if validationErr := v1alpha1.ValidatePlatformConfig(canonical); validationErr != nil {
		return &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: validationErr.Error()}
	}
	resolved := ResolvedInstance{Category: capability, Name: input.Name, AdapterRef: input.AdapterRef, Config: canonical}
	*output = append(*output, resolved)
	key := instanceKey(capability, input.Name)
	byKey[key] = resolved
	descriptorByKey[key] = requirement.descriptor
	return nil
}

func resolveProfile(capability Capability, path string, input v1alpha1.PlatformProfile, instances map[string]ResolvedInstance, descriptors map[string]Descriptor) (ResolvedProfile, *ResolveError) {
	key := instanceKey(capability, input.BackendRef)
	backend, ok := instances[key]
	if !ok {
		return ResolvedProfile{}, &ResolveError{Category: ErrorInvalidContract, FieldPath: path + ".backendRef", Detail: "unknown backend instance"}
	}
	descriptor := descriptors[key]
	if descriptor.CanonicalizeProfile == nil {
		return ResolvedProfile{}, &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: "adapter has no profile-pair canonicalizer"}
	}
	canonical, err := descriptor.CanonicalizeProfile(capability, cloneConfig(backend.Config), cloneConfig(input.Config))
	if err != nil {
		return ResolvedProfile{}, &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: safeAdapterErrorDetail(err)}
	}
	canonical = cloneConfig(canonical)
	if validationErr := v1alpha1.ValidatePlatformConfig(canonical); validationErr != nil {
		return ResolvedProfile{}, &ResolveError{Category: ErrorInvalidConfig, FieldPath: path + ".config", Detail: validationErr.Error()}
	}
	return ResolvedProfile{Capability: capability, Name: input.Name, BackendRef: input.BackendRef, Config: canonical}, nil
}

type revisionPayload struct {
	PlatformName     string                           `json:"platformName"`
	Adapters         []ResolvedAdapter                `json:"adapters"`
	Instances        []ResolvedInstance               `json:"instances"`
	Profiles         []ResolvedProfile                `json:"profiles"`
	ModelRoutes      []ModelRoute                     `json:"modelRoutes"`
	InitialPolicyRef v1alpha1.PlatformPolicyReference `json:"initialPolicyRef"`
}

func lockFromResolved(resolved *ResolvedPlatform) (*PlatformLock, *ResolveError) {
	lock := &PlatformLock{PlatformName: resolved.PlatformName, Revision: resolved.Revision, Adapters: append([]ResolvedAdapter(nil), resolved.Adapters...), ModelRoutes: append([]ModelRoute(nil), resolved.ModelRoutes...), InitialPolicyRef: resolved.InitialPolicyRef}
	for _, instance := range resolved.Instances {
		digest, err := digestValue(instance.Config)
		if err != nil {
			return nil, &ResolveError{Category: ErrorCanonicalEncoding, FieldPath: "instances." + instance.Name, Detail: err.Error()}
		}
		lock.Instances = append(lock.Instances, LockedConfig{Category: instance.Category, Name: instance.Name, Reference: instance.AdapterRef, ConfigDigest: digest})
	}
	for _, profile := range resolved.Profiles {
		digest, err := digestValue(profile.Config)
		if err != nil {
			return nil, &ResolveError{Category: ErrorCanonicalEncoding, FieldPath: "profiles." + profile.Name, Detail: err.Error()}
		}
		lock.Profiles = append(lock.Profiles, LockedConfig{Category: profile.Capability, Name: profile.Name, Reference: profile.BackendRef, ConfigDigest: digest})
	}
	return lock, nil
}

// VerifyResolvedLock rejects a stored effective Platform whose contents no
// longer match either its revision or its digest-only lock. This is an
// integrity check, not authentication of the local state file's author.
func VerifyResolvedLock(resolved *ResolvedPlatform, lock *PlatformLock) error {
	if resolved == nil || lock == nil || resolved.Revision == "" || resolved.Revision != lock.Revision {
		return fmt.Errorf("effective Platform and lock revision mismatch")
	}
	payload := revisionPayload{
		PlatformName:     resolved.PlatformName,
		Adapters:         resolved.Adapters,
		Instances:        resolved.Instances,
		Profiles:         resolved.Profiles,
		ModelRoutes:      resolved.ModelRoutes,
		InitialPolicyRef: resolved.InitialPolicyRef,
	}
	revision, err := digestValue(payload)
	if err != nil || revision != resolved.Revision {
		return fmt.Errorf("effective Platform content revision mismatch")
	}
	expected, failure := lockFromResolved(resolved)
	if failure != nil || !reflect.DeepEqual(expected, lock) {
		return fmt.Errorf("effective Platform lock content mismatch")
	}
	return nil
}

func digestValue(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	encoded := buffer.Bytes()
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
		return "", fmt.Errorf("canonical encoder did not emit one trailing newline")
	}
	encoded = encoded[:len(encoded)-1]
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func hasCapability(values []Capability, target Capability) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func instanceKey(capability Capability, name string) string {
	return string(capability) + "\x00" + name
}

func cloneConfig(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	return cloneJSONValue(reflect.ValueOf(value))
}

func cloneJSONValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return cloneJSONValue(value.Elem())
	}
	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		result := make(map[string]any, value.Len())
		for _, key := range value.MapKeys() {
			result[key.String()] = cloneJSONValue(value.MapIndex(key))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		fallthrough
	case reflect.Array:
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = cloneJSONValue(value.Index(i))
		}
		return result
	default:
		return value.Interface()
	}
}

func safeAdapterErrorDetail(err error) string {
	var safe *AdapterConfigError
	if errors.As(err, &safe) {
		return safe.Error()
	}
	return "adapter rejected configuration"
}

func safeDiagnosticToken(value string) string {
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return "invalid-adapter-config"
		}
	}
	if value == "" || len(value) > 64 {
		return "invalid-adapter-config"
	}
	return value
}

func safeDiagnosticPath(value string) string {
	if value == "" || len(value) > 128 {
		return "config"
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '[' && char != ']' && char != '-' && char != '_' {
			return "config"
		}
	}
	return value
}

// CanonicalJSON exposes the frozen inspectable encoding used by golden tests
// and future plan output. It never appends a newline.
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(buffer.String(), "\n")), nil
}
