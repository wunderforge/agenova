// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	AgentTemplateAPIVersion = "agenova.io/v1alpha1"
	AgentTemplateKind       = "AgentTemplate"
)

// ValidationCategory is a stable, machine-readable class of contract failure.
type ValidationCategory string

const (
	ValidationCategoryRequiredField            ValidationCategory = "required-field"
	ValidationCategoryInvalidCapabilityCeiling ValidationCategory = "invalid-capability-ceiling"
	ValidationCategorySystemManagedField       ValidationCategory = "system-managed-field"
	ValidationCategorySecretValue              ValidationCategory = "secret-value"
	ValidationCategoryUnknownField             ValidationCategory = "unknown-field"
	ValidationCategoryInvalidValue             ValidationCategory = "invalid-value"
	ValidationCategoryInvalidDocument          ValidationCategory = "invalid-document"
)

// ValidationError describes one deterministic contract failure without
// requiring callers to inspect its human-readable text.
type ValidationError struct {
	Category  ValidationCategory `json:"category"`
	FieldPath string             `json:"fieldPath"`
	Detail    string             `json:"detail,omitempty"`
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return fmt.Sprintf("%s: %s", e.Category, e.FieldPath)
	}
	return fmt.Sprintf("%s: %s: %s", e.Category, e.FieldPath, e.Detail)
}

func validationError(category ValidationCategory, fieldPath, detail string) *ValidationError {
	return &ValidationError{Category: category, FieldPath: fieldPath, Detail: detail}
}

// AgentTemplate is the backend-neutral, reusable agent-role contract.
// It declares an artifact, its entrypoint, safe defaults, and an upper bound
// on authority. It never contains issued authority or live credentials.
type AgentTemplate struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   ObjectMeta        `json:"metadata" yaml:"metadata"`
	Spec       AgentTemplateSpec `json:"spec" yaml:"spec"`
}

type AgentTemplateSpec struct {
	Artifact          *AgentTemplateArtifact          `json:"artifact" yaml:"artifact"`
	Entrypoint        *AgentTemplateEntrypoint        `json:"entrypoint" yaml:"entrypoint"`
	Defaults          AgentTemplateDefaults           `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	CapabilityCeiling *AgentTemplateCapabilityCeiling `json:"capabilityCeiling" yaml:"capabilityCeiling"`
}

type AgentTemplateArtifact struct {
	Image string `json:"image" yaml:"image"`
}

type AgentTemplateEntrypoint struct {
	Command []string `json:"command" yaml:"command"`
}

type AgentTemplateDefaults struct {
	ModelProfile string   `json:"modelProfile,omitempty" yaml:"modelProfile,omitempty"`
	MemoryScopes []string `json:"memoryScopes,omitempty" yaml:"memoryScopes,omitempty"`
}

type AgentTemplateCapabilityCeiling struct {
	Tools           []string  `json:"tools,omitempty" yaml:"tools,omitempty"`
	ResourceScopes  []string  `json:"resourceScopes,omitempty" yaml:"resourceScopes,omitempty"`
	ModelProfiles   []string  `json:"modelProfiles,omitempty" yaml:"modelProfiles,omitempty"`
	MemoryScopes    []string  `json:"memoryScopes,omitempty" yaml:"memoryScopes,omitempty"`
	RuntimeProfiles []string  `json:"runtimeProfiles,omitempty" yaml:"runtimeProfiles,omitempty"`
	MaxTimeout      *Duration `json:"maxTimeout,omitempty" yaml:"maxTimeout,omitempty"`
}

// Duration preserves the human-authored Go duration syntax used by the v0
// contract while exposing a strongly typed value to Go callers.
type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// ParseAgentTemplateYAML parses exactly one strict v0 AgentTemplate document.
// Reserved fields are classified by exact path before decoding; all other
// unknown fields fail closed.
func ParseAgentTemplateYAML(data []byte) (*AgentTemplate, *ValidationError) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, validationError(ValidationCategoryInvalidDocument, "$", "document is empty")
		}
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if len(document.Content) != 1 {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "expected one YAML document")
	}

	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
		}
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "multiple YAML documents are not allowed")
	}

	root := document.Content[0]
	if err := validateAgentTemplateDocumentShape(root); err != nil {
		return nil, err
	}

	var template AgentTemplate
	if err := root.Decode(&template); err != nil {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if err := ValidateAgentTemplate(&template); err != nil {
		return nil, err
	}
	return &template, nil
}

// ValidateAgentTemplate validates semantic invariants on an already decoded
// template. An explicitly empty capability ceiling is valid and default-deny.
func ValidateAgentTemplate(template *AgentTemplate) *ValidationError {
	if template == nil {
		return validationError(ValidationCategoryRequiredField, "$", "template is required")
	}
	if strings.TrimSpace(template.APIVersion) == "" {
		return validationError(ValidationCategoryRequiredField, "apiVersion", "value is required")
	}
	if template.APIVersion != AgentTemplateAPIVersion {
		return validationError(ValidationCategoryInvalidValue, "apiVersion", "unsupported API version")
	}
	if strings.TrimSpace(template.Kind) == "" {
		return validationError(ValidationCategoryRequiredField, "kind", "value is required")
	}
	if template.Kind != AgentTemplateKind {
		return validationError(ValidationCategoryInvalidValue, "kind", "unsupported kind")
	}
	if strings.TrimSpace(template.Metadata.Name) == "" {
		return validationError(ValidationCategoryRequiredField, "metadata.name", "value is required")
	}
	if template.Spec.Artifact == nil {
		return validationError(ValidationCategoryRequiredField, "spec.artifact", "value is required")
	}
	if strings.TrimSpace(template.Spec.Artifact.Image) == "" {
		return validationError(ValidationCategoryRequiredField, "spec.artifact.image", "value is required")
	}
	if template.Spec.Entrypoint == nil {
		return validationError(ValidationCategoryRequiredField, "spec.entrypoint", "value is required")
	}
	if len(template.Spec.Entrypoint.Command) == 0 {
		return validationError(ValidationCategoryRequiredField, "spec.entrypoint.command", "at least one command element is required")
	}
	for i, command := range template.Spec.Entrypoint.Command {
		if strings.TrimSpace(command) == "" {
			return validationError(ValidationCategoryRequiredField, fmt.Sprintf("spec.entrypoint.command[%d]", i), "value is required")
		}
	}
	if template.Spec.CapabilityCeiling == nil {
		return validationError(ValidationCategoryRequiredField, "spec.capabilityCeiling", "value is required")
	}

	ceiling := template.Spec.CapabilityCeiling
	for _, list := range []struct {
		path   string
		values []string
	}{
		{"spec.capabilityCeiling.tools", ceiling.Tools},
		{"spec.capabilityCeiling.resourceScopes", ceiling.ResourceScopes},
		{"spec.capabilityCeiling.modelProfiles", ceiling.ModelProfiles},
		{"spec.capabilityCeiling.memoryScopes", ceiling.MemoryScopes},
		{"spec.capabilityCeiling.runtimeProfiles", ceiling.RuntimeProfiles},
	} {
		if err := validateUniqueNonBlankList(list.path, list.values); err != nil {
			return err
		}
	}
	if ceiling.MaxTimeout != nil && time.Duration(*ceiling.MaxTimeout) <= 0 {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, "spec.capabilityCeiling.maxTimeout", "duration must be positive")
	}

	defaults := template.Spec.Defaults
	if defaults.ModelProfile != "" && !contains(ceiling.ModelProfiles, defaults.ModelProfile) {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, "spec.defaults.modelProfile", "default is outside capability ceiling")
	}
	if err := validateUniqueNonBlankList("spec.defaults.memoryScopes", defaults.MemoryScopes); err != nil {
		return err
	}
	for i, scope := range defaults.MemoryScopes {
		if !contains(ceiling.MemoryScopes, scope) {
			return validationError(ValidationCategoryInvalidCapabilityCeiling, fmt.Sprintf("spec.defaults.memoryScopes[%d]", i), "default is outside capability ceiling")
		}
	}

	return nil
}

func validateUniqueNonBlankList(path string, values []string) *ValidationError {
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return validationError(ValidationCategoryInvalidCapabilityCeiling, fmt.Sprintf("%s[%d]", path, i), "value must be non-blank")
		}
		if _, duplicate := seen[value]; duplicate {
			return validationError(ValidationCategoryInvalidCapabilityCeiling, fmt.Sprintf("%s[%d]", path, i), "duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type nodeValidator func(*yaml.Node, string) *ValidationError

func validateAgentTemplateDocumentShape(root *yaml.Node) *ValidationError {
	return validateMapping(root, "$", map[string]nodeValidator{
		"apiVersion": validateStringScalar,
		"kind":       validateStringScalar,
		"metadata":   validateMetadataShape,
		"spec":       validateAgentTemplateSpecShape,
	}, nil)
}

func validateMetadataShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"name": validateStringScalar,
	}, nil)
}

func validateAgentTemplateSpecShape(node *yaml.Node, path string) *ValidationError {
	reserved := map[string]ValidationCategory{
		"effectiveAuthority": ValidationCategorySystemManagedField,
		"environment":        ValidationCategorySecretValue,
	}
	return validateMapping(node, path, map[string]nodeValidator{
		"artifact":          validateArtifactShape,
		"entrypoint":        validateEntrypointShape,
		"defaults":          validateDefaultsShape,
		"capabilityCeiling": validateCapabilityCeilingShape,
	}, reserved)
}

func validateArtifactShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"image": validateStringScalar,
	}, nil)
}

func validateEntrypointShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"command": validateStringSequence,
	}, nil)
}

func validateDefaultsShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"modelProfile": validateStringScalar,
		"memoryScopes": validateCapabilitySequence,
	}, nil)
}

func validateCapabilityCeilingShape(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.MappingNode || node.Tag == "!!null" {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, path, "expected a mapping")
	}
	return validateMapping(node, path, map[string]nodeValidator{
		"tools":           validateCapabilitySequence,
		"resourceScopes":  validateCapabilitySequence,
		"modelProfiles":   validateCapabilitySequence,
		"memoryScopes":    validateCapabilitySequence,
		"runtimeProfiles": validateCapabilitySequence,
		"maxTimeout":      validatePositiveDurationScalar,
	}, nil)
}

func validateMapping(node *yaml.Node, path string, allowed map[string]nodeValidator, reserved map[string]ValidationCategory) *ValidationError {
	if node.Kind != yaml.MappingNode || node.Tag == "!!null" {
		return validationError(ValidationCategoryInvalidDocument, path, "expected a mapping")
	}
	seen := make(map[string]struct{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return validationError(ValidationCategoryInvalidDocument, path, "mapping keys must be strings")
		}
		fieldPath := joinPath(path, key.Value)
		if _, duplicate := seen[key.Value]; duplicate {
			return validationError(ValidationCategoryInvalidDocument, fieldPath, "duplicate field")
		}
		seen[key.Value] = struct{}{}
		if category, ok := reserved[key.Value]; ok {
			return validationError(category, fieldPath, "reserved field is not allowed in caller-authored input")
		}
		validator, ok := allowed[key.Value]
		if !ok {
			return validationError(ValidationCategoryUnknownField, fieldPath, "field is not defined by this v0 contract")
		}
		if err := validator(value, fieldPath); err != nil {
			return err
		}
	}
	return nil
}

func validateStringScalar(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return validationError(ValidationCategoryInvalidDocument, path, "expected a string")
	}
	return nil
}

func validateStringSequence(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.SequenceNode {
		return validationError(ValidationCategoryInvalidDocument, path, "expected a list of strings")
	}
	for i, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return validationError(ValidationCategoryInvalidDocument, fmt.Sprintf("%s[%d]", path, i), "expected a string")
		}
	}
	return nil
}

func validateCapabilitySequence(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.SequenceNode {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, path, "expected a list of strings")
	}
	for i, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return validationError(ValidationCategoryInvalidCapabilityCeiling, fmt.Sprintf("%s[%d]", path, i), "expected a string")
		}
	}
	return nil
}

func validatePositiveDurationScalar(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, path, "expected a Go duration string")
	}
	duration, err := time.ParseDuration(node.Value)
	if err != nil || duration <= 0 {
		return validationError(ValidationCategoryInvalidCapabilityCeiling, path, "duration must be a positive Go duration")
	}
	return nil
}

func joinPath(parent, field string) string {
	if parent == "$" {
		return field
	}
	return parent + "." + field
}
