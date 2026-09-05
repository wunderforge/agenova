// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ClaimRequestAPIVersion = "agenova.io/v1alpha1"
	ClaimRequestKind       = "ClaimRequest"
)

// ValidationCategorySelfAssertedPrincipal rejects caller-controlled
// authoritative principal data inside a declarative request; the trusted
// principal always arrives out-of-band.
const ValidationCategorySelfAssertedPrincipal ValidationCategory = "self-asserted-principal"

// ClaimRequest is the backend-neutral, declarative v0 request for one agent
// worker assignment: which reusable template, what task, what requested
// access, and what runtime requirements. Requested access is intent for later
// resolution; the request never carries an authoritative principal, issued
// authority, or credential values.
type ClaimRequest struct {
	APIVersion string           `json:"apiVersion" yaml:"apiVersion"`
	Kind       string           `json:"kind" yaml:"kind"`
	Metadata   ObjectMeta       `json:"metadata" yaml:"metadata"`
	Spec       ClaimRequestSpec `json:"spec" yaml:"spec"`
}

type ClaimRequestSpec struct {
	TemplateRef     string                    `json:"templateRef" yaml:"templateRef"`
	Task            *ClaimRequestTask         `json:"task" yaml:"task"`
	RequestedAccess ClaimRequestedAccess      `json:"requestedAccess,omitempty" yaml:"requestedAccess,omitempty"`
	Runtime         *ClaimRuntimeRequirements `json:"runtime" yaml:"runtime"`
}

// ClaimRequestTask is the work definition. Its input is task data and stays
// distinct from requested resource scopes, which are access intent. Input is
// JSON-compatible structured data: string-keyed mappings whose values are
// strings, finite numbers, booleans, nulls, arrays, or nested string-keyed
// mappings — the same invariants are enforced on the document tree at parse
// time and on decoded values in ValidateClaimRequest.
type ClaimRequestTask struct {
	Type  string         `json:"type" yaml:"type"`
	Input map[string]any `json:"input,omitempty" yaml:"input,omitempty"`
}

// ClaimRequestedAccess expresses requested access only. Nothing here grants
// authority; resolution intersects it with template and policy limits.
type ClaimRequestedAccess struct {
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	ResourceScopes []string `json:"resourceScopes,omitempty" yaml:"resourceScopes,omitempty"`
	ModelProfile   string   `json:"modelProfile,omitempty" yaml:"modelProfile,omitempty"`
	MemoryScopes   []string `json:"memoryScopes,omitempty" yaml:"memoryScopes,omitempty"`
}

type ClaimRuntimeRequirements struct {
	ProfileRef string    `json:"profileRef" yaml:"profileRef"`
	Timeout    *Duration `json:"timeout" yaml:"timeout"`
}

// ParseClaimRequestYAML parses exactly one strict v0 ClaimRequest document
// from the canonical human-authored YAML surface. Reserved fields are
// classified by exact path before decoding; all other unknown fields fail
// closed.
func ParseClaimRequestYAML(data []byte) (*ClaimRequest, *ValidationError) {
	return parseClaimRequest(data)
}

// ParseClaimRequestJSON parses the equivalent API JSON surface. The input
// must be valid JSON; shape classification and semantic validation are shared
// with the YAML surface so the two forms cannot drift.
func ParseClaimRequestJSON(data []byte) (*ClaimRequest, *ValidationError) {
	if !json.Valid(data) {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "input is not valid JSON")
	}
	return parseClaimRequest(data)
}

func parseClaimRequest(data []byte) (*ClaimRequest, *ValidationError) {
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
	if err := validateClaimRequestDocumentShape(root); err != nil {
		return nil, err
	}

	var request ClaimRequest
	if err := root.Decode(&request); err != nil {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if err := ValidateClaimRequest(&request); err != nil {
		return nil, err
	}
	return &request, nil
}

// ValidateClaimRequest validates semantic invariants on an already decoded
// request.
func ValidateClaimRequest(request *ClaimRequest) *ValidationError {
	if request == nil {
		return validationError(ValidationCategoryRequiredField, "$", "request is required")
	}
	if strings.TrimSpace(request.APIVersion) == "" {
		return validationError(ValidationCategoryRequiredField, "apiVersion", "value is required")
	}
	if request.APIVersion != ClaimRequestAPIVersion {
		return validationError(ValidationCategoryInvalidValue, "apiVersion", "unsupported API version")
	}
	if strings.TrimSpace(request.Kind) == "" {
		return validationError(ValidationCategoryRequiredField, "kind", "value is required")
	}
	if request.Kind != ClaimRequestKind {
		return validationError(ValidationCategoryInvalidValue, "kind", "unsupported kind")
	}
	if strings.TrimSpace(request.Metadata.Name) == "" {
		return validationError(ValidationCategoryRequiredField, "metadata.name", "value is required")
	}
	if strings.TrimSpace(request.Spec.TemplateRef) == "" {
		return validationError(ValidationCategoryRequiredField, "spec.templateRef", "value is required")
	}
	if request.Spec.Task == nil {
		return validationError(ValidationCategoryRequiredField, "spec.task", "value is required")
	}
	if strings.TrimSpace(request.Spec.Task.Type) == "" {
		return validationError(ValidationCategoryRequiredField, "spec.task.type", "value is required")
	}
	if err := validateTaskInputValues("spec.task.input", request.Spec.Task.Input); err != nil {
		return err
	}
	if err := validateRequestList("spec.requestedAccess.tools", request.Spec.RequestedAccess.Tools); err != nil {
		return err
	}
	if err := validateRequestList("spec.requestedAccess.resourceScopes", request.Spec.RequestedAccess.ResourceScopes); err != nil {
		return err
	}
	if err := validateRequestList("spec.requestedAccess.memoryScopes", request.Spec.RequestedAccess.MemoryScopes); err != nil {
		return err
	}
	if request.Spec.Runtime == nil {
		return validationError(ValidationCategoryRequiredField, "spec.runtime", "value is required")
	}
	if strings.TrimSpace(request.Spec.Runtime.ProfileRef) == "" {
		return validationError(ValidationCategoryRequiredField, "spec.runtime.profileRef", "value is required")
	}
	if request.Spec.Runtime.Timeout == nil {
		return validationError(ValidationCategoryRequiredField, "spec.runtime.timeout", "value is required")
	}
	if time.Duration(*request.Spec.Runtime.Timeout) <= 0 {
		return validationError(ValidationCategoryInvalidValue, "spec.runtime.timeout", "timeout must be positive")
	}
	return nil
}

// validateTaskInputValues re-checks the JSON-compatibility contract on decoded
// Go values, because callers can construct or mutate the public
// map[string]any directly — including with NaN, time.Time, or []byte — and
// the contract must fail closed there too, not only at the parser. Keys are
// walked in sorted order so the reported path is deterministic, and containers
// on the current path are tracked so a cyclic value is rejected instead of
// recursing without bound.
func validateTaskInputValues(path string, values map[string]any) *ValidationError {
	return validateTaskInputValue(path, values, make(map[uintptr]struct{}))
}

func validateTaskInputValue(path string, value any, active map[uintptr]struct{}) *ValidationError {
	switch v := value.(type) {
	case nil, string, bool:
		return nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		return validateTaskInputFloat(path, float64(v))
	case float64:
		return validateTaskInputFloat(path, v)
	case []byte:
		// encoding/json emits []byte as base64, not as a JSON array, so its
		// representation is inconsistent across the two surfaces.
		return validationError(ValidationCategoryInvalidValue, path, "value has no consistent JSON representation")
	}
	// Typed containers such as []string, [2]int, or map[string]string are
	// JSON-compatible too; walk them by kind rather than by concrete type.
	container := reflect.ValueOf(value)
	switch container.Kind() {
	case reflect.Slice:
		if container.Len() == 0 {
			return nil
		}
		if _, cyclic := active[container.Pointer()]; cyclic {
			return validationError(ValidationCategoryInvalidValue, path, "cyclic value has no JSON representation")
		}
		active[container.Pointer()] = struct{}{}
		defer delete(active, container.Pointer())
		fallthrough
	case reflect.Array:
		for i := 0; i < container.Len(); i++ {
			if err := validateTaskInputValue(fmt.Sprintf("%s[%d]", path, i), container.Index(i).Interface(), active); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if container.Type().Key().Kind() != reflect.String {
			return validationError(ValidationCategoryInvalidValue, path, "mapping keys must be strings")
		}
		if container.Len() == 0 {
			return nil
		}
		if _, cyclic := active[container.Pointer()]; cyclic {
			return validationError(ValidationCategoryInvalidValue, path, "cyclic value has no JSON representation")
		}
		active[container.Pointer()] = struct{}{}
		defer delete(active, container.Pointer())
		keys := make([]string, 0, container.Len())
		for _, key := range container.MapKeys() {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		for _, key := range keys {
			item := container.MapIndex(reflect.ValueOf(key).Convert(container.Type().Key())).Interface()
			if err := validateTaskInputValue(joinPath(path, key), item, active); err != nil {
				return err
			}
		}
		return nil
	default:
		return validationError(ValidationCategoryInvalidValue, path, "value has no consistent JSON representation")
	}
}

func validateTaskInputFloat(path string, value float64) *ValidationError {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return validationError(ValidationCategoryInvalidValue, path, "floats must be finite JSON numbers")
	}
	return nil
}

func validateRequestList(path string, values []string) *ValidationError {
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return validationError(ValidationCategoryInvalidValue, fmt.Sprintf("%s[%d]", path, i), "value must be non-blank")
		}
		if _, duplicate := seen[value]; duplicate {
			return validationError(ValidationCategoryInvalidValue, fmt.Sprintf("%s[%d]", path, i), "duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateClaimRequestDocumentShape(root *yaml.Node) *ValidationError {
	return validateMapping(root, "$", map[string]nodeValidator{
		"apiVersion": nullable(validateStringScalar),
		"kind":       nullable(validateStringScalar),
		"metadata":   nullable(validateClaimRequestMetadataShape),
		"spec":       nullable(validateClaimRequestSpecShape),
	}, nil)
}

func validateClaimRequestMetadataShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"name": nullable(validateStringScalar),
	}, nil)
}

// nullable accepts a key written with no value: YAML null means "empty" for an
// optional field, and for a required one it decodes to the zero value so the
// semantic check reports `required-field` rather than a shape error.
func nullable(validator nodeValidator) nodeValidator {
	return func(node *yaml.Node, path string) *ValidationError {
		if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
			return nil
		}
		return validator(node, path)
	}
}

func validateClaimRequestSpecShape(node *yaml.Node, path string) *ValidationError {
	reserved := map[string]ValidationCategory{
		"principal": ValidationCategorySelfAssertedPrincipal,
		"secrets":   ValidationCategorySecretValue,
	}
	return validateMapping(node, path, map[string]nodeValidator{
		"templateRef":     nullable(validateStringScalar),
		"task":            nullable(validateClaimTaskShape),
		"requestedAccess": nullable(validateClaimAccessShape),
		"runtime":         nullable(validateClaimRuntimeShape),
	}, reserved)
}

func validateClaimTaskShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"type":  nullable(validateStringScalar),
		"input": nullable(validateTaskInputShape),
	}, nil)
}

func validateClaimAccessShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"tools":          nullable(validateStringSequence),
		"resourceScopes": nullable(validateStringSequence),
		"modelProfile":   nullable(validateStringScalar),
		"memoryScopes":   nullable(validateStringSequence),
	}, nil)
}

func validateClaimRuntimeShape(node *yaml.Node, path string) *ValidationError {
	return validateMapping(node, path, map[string]nodeValidator{
		"profileRef": nullable(validateStringScalar),
		"timeout":    nullable(validateClaimTimeoutScalar),
	}, nil)
}

// validateTaskInputShape enforces the JSON-compatibility contract on the
// spec.task.input document tree before decoding, because decoding can erase
// the offending construct (a !!binary scalar, for example, need not survive
// as []byte). Aliases and merge keys are rejected in v0: they have no JSON
// syntax, and expanding them reintroduces duplicate-key ambiguity.
func validateTaskInputShape(node *yaml.Node, path string) *ValidationError {
	if node.Kind == yaml.AliasNode {
		return validationError(ValidationCategoryInvalidDocument, path, "aliases have no JSON representation")
	}
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		return validationError(ValidationCategoryInvalidDocument, path, "expected a string-keyed mapping")
	}
	return validateTaskInputMappingNode(node, path)
}

func validateTaskInputMappingNode(node *yaml.Node, path string) *ValidationError {
	seen := make(map[string]struct{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return validationError(ValidationCategoryInvalidDocument, path, "mapping keys must be strings")
		}
		fieldPath := joinPath(path, key.Value)
		if _, duplicate := seen[key.Value]; duplicate {
			return validationError(ValidationCategoryInvalidDocument, fieldPath, "duplicate key")
		}
		seen[key.Value] = struct{}{}
		if err := validateTaskInputValueNode(value, fieldPath); err != nil {
			return err
		}
	}
	return nil
}

func validateTaskInputValueNode(node *yaml.Node, path string) *ValidationError {
	switch node.Kind {
	case yaml.AliasNode:
		return validationError(ValidationCategoryInvalidDocument, path, "aliases have no JSON representation")
	case yaml.MappingNode:
		if node.Tag != "!!map" {
			return validationError(ValidationCategoryInvalidDocument, path, "value has no consistent JSON representation")
		}
		return validateTaskInputMappingNode(node, path)
	case yaml.SequenceNode:
		if node.Tag != "!!seq" {
			return validationError(ValidationCategoryInvalidDocument, path, "value has no consistent JSON representation")
		}
		for i, item := range node.Content {
			if err := validateTaskInputValueNode(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str", "!!bool", "!!null", "!!int":
			return nil
		case "!!float":
			var value float64
			if err := node.Decode(&value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return validationError(ValidationCategoryInvalidDocument, path, "floats must be finite JSON numbers")
			}
			return nil
		default:
			return validationError(ValidationCategoryInvalidDocument, path, "value has no consistent JSON representation")
		}
	default:
		return validationError(ValidationCategoryInvalidDocument, path, "value has no consistent JSON representation")
	}
}

func validateClaimTimeoutScalar(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return validationError(ValidationCategoryInvalidValue, path, "expected a Go duration string")
	}
	duration, err := time.ParseDuration(node.Value)
	if err != nil || duration <= 0 {
		return validationError(ValidationCategoryInvalidValue, path, "timeout must be a positive Go duration")
	}
	return nil
}
