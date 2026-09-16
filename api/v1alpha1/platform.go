// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	PlatformAPIVersion = "agenova.io/v1alpha1"
	PlatformKind       = "Platform"
)

var platformLocalName = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9.]{0,61}[a-z0-9])?$`)

// Platform is one installation's backend-neutral desired state. Config maps
// remain adapter-owned; this shared contract only names and relates them.
type Platform struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Kind       string       `json:"kind" yaml:"kind"`
	Metadata   ObjectMeta   `json:"metadata" yaml:"metadata"`
	Spec       PlatformSpec `json:"spec" yaml:"spec"`
}

type PlatformSpec struct {
	Adapters         []PlatformAdapterRequirement `json:"adapters" yaml:"adapters"`
	Infrastructure   PlatformInfrastructure       `json:"infrastructure" yaml:"infrastructure"`
	Services         PlatformServices             `json:"services,omitempty" yaml:"services,omitempty"`
	InitialPolicyRef *PlatformPolicyReference     `json:"initialPolicyRef" yaml:"initialPolicyRef"`
}

type PlatformAdapterRequirement struct {
	Name    string `json:"name" yaml:"name"`
	ID      string `json:"id" yaml:"id"`
	Version string `json:"version" yaml:"version"`
}

type PlatformInfrastructure struct {
	Deployment      *PlatformInstance  `json:"deployment" yaml:"deployment"`
	RuntimeBackends []PlatformInstance `json:"runtimeBackends" yaml:"runtimeBackends"`
	RuntimeProfiles []PlatformProfile  `json:"runtimeProfiles" yaml:"runtimeProfiles"`
}

type PlatformServices struct {
	ModelBackends []PlatformInstance `json:"modelBackends,omitempty" yaml:"modelBackends,omitempty"`
	ModelProfiles []PlatformProfile  `json:"modelProfiles,omitempty" yaml:"modelProfiles,omitempty"`
}

type PlatformInstance struct {
	Name       string         `json:"name" yaml:"name"`
	AdapterRef string         `json:"adapterRef" yaml:"adapterRef"`
	Config     map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

type PlatformProfile struct {
	Name       string         `json:"name" yaml:"name"`
	BackendRef string         `json:"backendRef" yaml:"backendRef"`
	Config     map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

type PlatformPolicyReference struct {
	ID      string `json:"id" yaml:"id"`
	Version string `json:"version" yaml:"version"`
}

func ParsePlatformYAML(data []byte) (*Platform, *ValidationError) {
	return parsePlatform(data)
}

func ParsePlatformJSON(data []byte) (*Platform, *ValidationError) {
	if !json.Valid(data) {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "input is not valid JSON")
	}
	return parsePlatform(data)
}

func parsePlatform(data []byte) (*Platform, *ValidationError) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, validationError(ValidationCategoryInvalidDocument, "$", "document is empty")
		}
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if len(document.Content) != 1 {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "expected one document")
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
		}
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "multiple documents are not allowed")
	}
	root := document.Content[0]
	if err := validatePlatformDocumentShape(root); err != nil {
		return nil, err
	}
	var platform Platform
	if err := root.Decode(&platform); err != nil {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if err := ValidatePlatform(&platform); err != nil {
		return nil, err
	}
	return &platform, nil
}

// ValidatePlatform checks the shared structural and reference invariants.
// Adapter capability and config semantics are resolved in internal/platform.
func ValidatePlatform(platform *Platform) *ValidationError {
	if platform == nil {
		return validationError(ValidationCategoryRequiredField, "$", "platform is required")
	}
	if platform.APIVersion != PlatformAPIVersion {
		if strings.TrimSpace(platform.APIVersion) == "" {
			return validationError(ValidationCategoryRequiredField, "apiVersion", "value is required")
		}
		return validationError(ValidationCategoryInvalidValue, "apiVersion", "unsupported API version")
	}
	if platform.Kind != PlatformKind {
		if strings.TrimSpace(platform.Kind) == "" {
			return validationError(ValidationCategoryRequiredField, "kind", "value is required")
		}
		return validationError(ValidationCategoryInvalidValue, "kind", "unsupported kind")
	}
	if err := validatePlatformName("metadata.name", platform.Metadata.Name); err != nil {
		return err
	}
	if len(platform.Spec.Adapters) == 0 {
		return validationError(ValidationCategoryRequiredField, "spec.adapters", "at least one adapter requirement is required")
	}
	adapterNames := map[string]struct{}{}
	for i, adapter := range platform.Spec.Adapters {
		base := fmt.Sprintf("spec.adapters[%d]", i)
		if err := validatePlatformName(base+".name", adapter.Name); err != nil {
			return err
		}
		if _, duplicate := adapterNames[adapter.Name]; duplicate {
			return validationError(ValidationCategoryInvalidValue, base+".name", "duplicate adapter requirement")
		}
		adapterNames[adapter.Name] = struct{}{}
		if err := validateBoundedToken(base+".id", adapter.ID, 256); err != nil {
			return err
		}
		if err := validateBoundedToken(base+".version", adapter.Version, 64); err != nil {
			return err
		}
	}

	if platform.Spec.Infrastructure.Deployment == nil {
		return validationError(ValidationCategoryRequiredField, "spec.infrastructure.deployment", "exactly one deployment is required")
	}
	if err := validatePlatformInstance("spec.infrastructure.deployment", *platform.Spec.Infrastructure.Deployment, adapterNames); err != nil {
		return err
	}
	if len(platform.Spec.Infrastructure.RuntimeBackends) == 0 {
		return validationError(ValidationCategoryRequiredField, "spec.infrastructure.runtimeBackends", "at least one runtime backend is required")
	}
	runtimeNames, err := validatePlatformInstances("spec.infrastructure.runtimeBackends", platform.Spec.Infrastructure.RuntimeBackends, adapterNames)
	if err != nil {
		return err
	}
	if len(platform.Spec.Infrastructure.RuntimeProfiles) == 0 {
		return validationError(ValidationCategoryRequiredField, "spec.infrastructure.runtimeProfiles", "at least one runtime profile is required")
	}
	if err := validatePlatformProfiles("spec.infrastructure.runtimeProfiles", platform.Spec.Infrastructure.RuntimeProfiles, runtimeNames); err != nil {
		return err
	}

	modelNames, err := validatePlatformInstances("spec.services.modelBackends", platform.Spec.Services.ModelBackends, adapterNames)
	if err != nil {
		return err
	}
	if err := validatePlatformProfiles("spec.services.modelProfiles", platform.Spec.Services.ModelProfiles, modelNames); err != nil {
		return err
	}
	if platform.Spec.InitialPolicyRef == nil {
		return validationError(ValidationCategoryRequiredField, "spec.initialPolicyRef", "value is required")
	}
	if err := validateBoundedToken("spec.initialPolicyRef.id", platform.Spec.InitialPolicyRef.ID, 128); err != nil {
		return err
	}
	return validateBoundedToken("spec.initialPolicyRef.version", platform.Spec.InitialPolicyRef.Version, 64)
}

func validatePlatformInstances(path string, instances []PlatformInstance, adapterNames map[string]struct{}) (map[string]struct{}, *ValidationError) {
	names := make(map[string]struct{}, len(instances))
	for i, instance := range instances {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if err := validatePlatformInstance(itemPath, instance, adapterNames); err != nil {
			return nil, err
		}
		if _, duplicate := names[instance.Name]; duplicate {
			return nil, validationError(ValidationCategoryInvalidValue, itemPath+".name", "duplicate instance name")
		}
		names[instance.Name] = struct{}{}
	}
	return names, nil
}

func validatePlatformInstance(path string, instance PlatformInstance, adapterNames map[string]struct{}) *ValidationError {
	if err := validatePlatformName(path+".name", instance.Name); err != nil {
		return err
	}
	if err := validatePlatformName(path+".adapterRef", instance.AdapterRef); err != nil {
		return err
	}
	if _, ok := adapterNames[instance.AdapterRef]; !ok {
		return validationError(ValidationCategoryInvalidValue, path+".adapterRef", "unknown adapter requirement")
	}
	return validatePlatformConfigValues(path+".config", instance.Config)
}

func validatePlatformProfiles(path string, profiles []PlatformProfile, backendNames map[string]struct{}) *ValidationError {
	seen := map[string]struct{}{}
	for i, profile := range profiles {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if err := validatePlatformName(itemPath+".name", profile.Name); err != nil {
			return err
		}
		if _, duplicate := seen[profile.Name]; duplicate {
			return validationError(ValidationCategoryInvalidValue, itemPath+".name", "duplicate profile name")
		}
		seen[profile.Name] = struct{}{}
		if err := validatePlatformName(itemPath+".backendRef", profile.BackendRef); err != nil {
			return err
		}
		if _, ok := backendNames[profile.BackendRef]; !ok {
			return validationError(ValidationCategoryInvalidValue, itemPath+".backendRef", "unknown backend instance")
		}
		if err := validatePlatformConfigValues(itemPath+".config", profile.Config); err != nil {
			return err
		}
	}
	return nil
}

func validatePlatformName(path, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return validationError(ValidationCategoryRequiredField, path, "value is required")
	}
	if !platformLocalName.MatchString(value) {
		return validationError(ValidationCategoryInvalidValue, path, "must be a normalized local name of at most 63 characters")
	}
	return nil
}

func validateBoundedToken(path, value string, max int) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return validationError(ValidationCategoryRequiredField, path, "value is required")
	}
	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "\r\n\t ") || len(value) > max {
		return validationError(ValidationCategoryInvalidValue, path, "value must be normalized and bounded")
	}
	return nil
}

func validatePlatformConfigValues(path string, config map[string]any) *ValidationError {
	if config == nil {
		return nil
	}
	if err := validateTaskInputValues(path, config); err != nil {
		return err
	}
	return findPlatformCredentialReferenceValue(path, reflect.ValueOf(config))
}

// ValidatePlatformConfig validates adapter-returned canonical config with the
// same secret-free, JSON-compatible boundary used for caller-authored input.
func ValidatePlatformConfig(config map[string]any) *ValidationError {
	return validatePlatformConfigValues("config", config)
}

func findPlatformCredentialReferenceValue(path string, value reflect.Value) *ValidationError {
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return findPlatformCredentialReferenceValue(path, value.Elem())
	}
	switch value.Kind() {
	case reflect.Map:
		keys := make([]string, 0, value.Len())
		for _, key := range value.MapKeys() {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		for _, key := range keys {
			if platformCredentialReferenceField(key) {
				return validationError(ValidationCategorySecretValue, joinPath(path, key), "credential references are unavailable until the host-side resolver is implemented")
			}
			item := value.MapIndex(reflect.ValueOf(key).Convert(value.Type().Key()))
			if err := findPlatformCredentialReferenceValue(joinPath(path, key), item); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := findPlatformCredentialReferenceValue(fmt.Sprintf("%s[%d]", path, i), value.Index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func platformCredentialReferenceField(name string) bool {
	switch NormalizeCredentialFieldName(name) {
	case "credentialref", "credentialsref", "secretref":
		return true
	default:
		return false
	}
}

func validatePlatformDocumentShape(root *yaml.Node) *ValidationError {
	return validateMapping(root, "$", map[string]nodeValidator{
		"apiVersion": nullable(validateStringScalar),
		"kind":       nullable(validateStringScalar),
		"metadata":   nullable(validateMetadataShape),
		"spec":       nullable(validatePlatformSpecShape),
	}, map[string]ValidationCategory{"status": ValidationCategorySystemManagedField, "lock": ValidationCategorySystemManagedField})
}

func validatePlatformSpecShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"adapters":         nullable(validatePlatformAdaptersShape),
		"infrastructure":   nullable(validatePlatformInfrastructureShape),
		"services":         nullable(validatePlatformServicesShape),
		"initialPolicyRef": nullable(validatePlatformPolicyRefShape),
	}, nil)
}

func validatePlatformAdaptersShape(node *yaml.Node, path string) *ValidationError {
	return validatePlatformSequence(node, path, func(item *yaml.Node, itemPath string) *ValidationError {
		return validateMapping(item, itemPath, map[string]nodeValidator{
			"name": nullable(validateStringScalar), "id": nullable(validateStringScalar), "version": nullable(validateStringScalar),
		}, nil)
	})
}

func validatePlatformInfrastructureShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"deployment": nullable(validatePlatformInstanceShape), "runtimeBackends": nullable(validatePlatformInstancesShape), "runtimeProfiles": nullable(validatePlatformProfilesShape),
	}, nil)
}

func validatePlatformServicesShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"modelBackends": nullable(validatePlatformInstancesShape), "modelProfiles": nullable(validatePlatformProfilesShape),
	}, nil)
}

func validatePlatformInstancesShape(node *yaml.Node, path string) *ValidationError {
	return validatePlatformSequence(node, path, validatePlatformInstanceShape)
}

func validatePlatformProfilesShape(node *yaml.Node, path string) *ValidationError {
	return validatePlatformSequence(node, path, func(item *yaml.Node, itemPath string) *ValidationError {
		return validateMapping(item, itemPath, map[string]nodeValidator{
			"name": nullable(validateStringScalar), "backendRef": nullable(validateStringScalar), "config": nullable(validatePlatformConfigShape),
		}, nil)
	})
}

func validatePlatformInstanceShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"name": nullable(validateStringScalar), "adapterRef": nullable(validateStringScalar), "config": nullable(validatePlatformConfigShape),
	}, nil)
}

func validatePlatformPolicyRefShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"id": nullable(validateStringScalar), "version": nullable(validateStringScalar),
	}, nil)
}

func validatePlatformSequence(node *yaml.Node, path string, itemValidator nodeValidator) *ValidationError {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" {
		return validationError(ValidationCategoryInvalidDocument, path, "expected a list")
	}
	for i, item := range node.Content {
		if err := itemValidator(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func validatePlatformConfigShape(node *yaml.Node, path string) *ValidationError {
	if err := validateTaskInputShape(node, path); err != nil {
		return err
	}
	return findPlatformCredentialReferenceNode(node, path)
}

func findPlatformCredentialReferenceNode(node *yaml.Node, path string) *ValidationError {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			fieldPath := joinPath(path, key.Value)
			if platformCredentialReferenceField(key.Value) {
				return validationError(ValidationCategorySecretValue, fieldPath, "credential references are unavailable until the host-side resolver is implemented")
			}
			if err := findPlatformCredentialReferenceNode(value, fieldPath); err != nil {
				return err
			}
		}
	} else if node.Kind == yaml.SequenceNode {
		for i, item := range node.Content {
			if err := findPlatformCredentialReferenceNode(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
