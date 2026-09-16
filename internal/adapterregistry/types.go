// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package adapterregistry owns the explicit catalog and activation lifecycle
// for Agenova adapter implementations. It does not load arbitrary code.
package adapterregistry

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

const (
	ProtocolVersion = "agenova.adapter/v1alpha1"
	LockAPIVersion  = "agenova.io/v1alpha1"
	LockKind        = "AdapterLock"
)

type ValueKind string

const (
	ValueString  ValueKind = "string"
	ValueBoolean ValueKind = "boolean"
	ValueInteger ValueKind = "integer"
)

// Field describes one adapter-owned configuration value. Defaults are
// documentation and initialization aids, never credentials or authority.
type Field struct {
	Path        string    `json:"path" yaml:"path"`
	Kind        ValueKind `json:"kind" yaml:"kind"`
	Required    bool      `json:"required,omitempty" yaml:"required,omitempty"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	Default     any       `json:"default,omitempty" yaml:"default,omitempty"`
}

type ConfigSchema struct {
	Fields []Field `json:"fields,omitempty" yaml:"fields,omitempty"`
}

type Manifest struct {
	ID             string                `json:"id" yaml:"id"`
	Version        string                `json:"version" yaml:"version"`
	Protocol       string                `json:"protocol" yaml:"protocol"`
	Capabilities   []platform.Capability `json:"capabilities" yaml:"capabilities"`
	InstanceSchema ConfigSchema          `json:"instanceSchema" yaml:"instanceSchema"`
	ProfileSchema  ConfigSchema          `json:"profileSchema,omitempty" yaml:"profileSchema,omitempty"`
}

// Factory constructs one capability-owned implementation. The result is
// intentionally untyped here: deployment, runtime and model consumers define
// their own narrow operational interfaces rather than sharing a fake one.
type Factory func() (any, error)

type Registration struct {
	Manifest   Manifest
	Descriptor platform.Descriptor
	Factories  map[platform.Capability]Factory
}

type InstalledAdapter struct {
	ID           string                `json:"id" yaml:"id"`
	Version      string                `json:"version" yaml:"version"`
	Protocol     string                `json:"protocol" yaml:"protocol"`
	Capabilities []platform.Capability `json:"capabilities" yaml:"capabilities"`
}

type InstallationLock struct {
	APIVersion string             `json:"apiVersion" yaml:"apiVersion"`
	Kind       string             `json:"kind" yaml:"kind"`
	Adapters   []InstalledAdapter `json:"adapters" yaml:"adapters"`
}

type InspectResult struct {
	Manifest  Manifest `json:"manifest" yaml:"manifest"`
	Installed bool     `json:"installed" yaml:"installed"`
}

type InstallResult struct {
	Adapter InstalledAdapter `json:"adapter" yaml:"adapter"`
	Changed bool             `json:"changed" yaml:"changed"`
}

// PlatformFragment is a mergeable portion of Platform.spec produced by init.
// It is not a second desired-state contract and does not grant authority.
type PlatformFragment struct {
	Spec FragmentSpec `json:"spec" yaml:"spec"`
}

type FragmentSpec struct {
	Adapters       []v1alpha1.PlatformAdapterRequirement `json:"adapters" yaml:"adapters"`
	Infrastructure *FragmentInfrastructure               `json:"infrastructure,omitempty" yaml:"infrastructure,omitempty"`
	Services       *FragmentServices                     `json:"services,omitempty" yaml:"services,omitempty"`
}

type FragmentInfrastructure struct {
	Deployment      *v1alpha1.PlatformInstance  `json:"deployment,omitempty" yaml:"deployment,omitempty"`
	RuntimeBackends []v1alpha1.PlatformInstance `json:"runtimeBackends,omitempty" yaml:"runtimeBackends,omitempty"`
	RuntimeProfiles []v1alpha1.PlatformProfile  `json:"runtimeProfiles,omitempty" yaml:"runtimeProfiles,omitempty"`
}

type FragmentServices struct {
	ModelBackends []v1alpha1.PlatformInstance `json:"modelBackends,omitempty" yaml:"modelBackends,omitempty"`
	ModelProfiles []v1alpha1.PlatformProfile  `json:"modelProfiles,omitempty" yaml:"modelProfiles,omitempty"`
}

func installedFromManifest(manifest Manifest) InstalledAdapter {
	return InstalledAdapter{
		ID: manifest.ID, Version: manifest.Version, Protocol: manifest.Protocol,
		Capabilities: append([]platform.Capability(nil), manifest.Capabilities...),
	}
}

func emptyLock() InstallationLock {
	return InstallationLock{APIVersion: LockAPIVersion, Kind: LockKind, Adapters: []InstalledAdapter{}}
}

func normalizeLock(lock InstallationLock) (InstallationLock, error) {
	if lock.APIVersion == "" && lock.Kind == "" && lock.Adapters == nil {
		return emptyLock(), nil
	}
	if lock.APIVersion != LockAPIVersion || lock.Kind != LockKind {
		return InstallationLock{}, fmt.Errorf("unsupported adapter lock %q %q", lock.APIVersion, lock.Kind)
	}
	seen := map[string]string{}
	for i := range lock.Adapters {
		item := &lock.Adapters[i]
		if err := validateIdentity(item.ID, item.Version); err != nil {
			return InstallationLock{}, fmt.Errorf("adapters[%d]: %w", i, err)
		}
		if item.Protocol != ProtocolVersion {
			return InstallationLock{}, fmt.Errorf("adapters[%d]: unsupported protocol %q", i, item.Protocol)
		}
		if len(item.Capabilities) == 0 {
			return InstallationLock{}, fmt.Errorf("adapters[%d]: at least one capability is required", i)
		}
		capabilitySeen := map[platform.Capability]struct{}{}
		for _, capability := range item.Capabilities {
			if capability != platform.CapabilityDeployment && capability != platform.CapabilityRuntime && capability != platform.CapabilityModel {
				return InstallationLock{}, fmt.Errorf("adapters[%d]: unsupported capability %q", i, capability)
			}
			if _, exists := capabilitySeen[capability]; exists {
				return InstallationLock{}, fmt.Errorf("adapters[%d]: duplicate capability %q", i, capability)
			}
			capabilitySeen[capability] = struct{}{}
		}
		if version, ok := seen[item.ID]; ok {
			return InstallationLock{}, fmt.Errorf("adapters[%d]: duplicate active adapter %q (versions %q and %q)", i, item.ID, version, item.Version)
		}
		seen[item.ID] = item.Version
		item.Capabilities = sortedCapabilities(item.Capabilities)
	}
	sort.Slice(lock.Adapters, func(i, j int) bool {
		if lock.Adapters[i].ID == lock.Adapters[j].ID {
			return lock.Adapters[i].Version < lock.Adapters[j].Version
		}
		return lock.Adapters[i].ID < lock.Adapters[j].ID
	})
	if lock.Adapters == nil {
		lock.Adapters = []InstalledAdapter{}
	}
	return lock, nil
}

func sortedCapabilities(values []platform.Capability) []platform.Capability {
	result := append([]platform.Capability(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func cloneJSON(value any) (any, error) {
	return cloneJSONValue(reflect.ValueOf(value))
}

func cloneJSONValue(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		return cloneJSONValue(value.Elem())
	}
	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return nil, nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("object key is not a string")
		}
		result := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			cloned, err := cloneJSONValue(iterator.Value())
			if err != nil {
				return nil, err
			}
			result[iterator.Key().String()] = cloned
		}
		return result, nil
	case reflect.Slice:
		if value.IsNil() {
			return nil, nil
		}
		fallthrough
	case reflect.Array:
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned, err := cloneJSONValue(value.Index(i))
			if err != nil {
				return nil, err
			}
			result[i] = cloned
		}
		return result, nil
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return value.Interface(), nil
	default:
		return nil, fmt.Errorf("unsupported JSON value kind %s", value.Kind())
	}
}

func setPath(target map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	cursor := target
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("invalid empty config path segment in %q", path)
		}
		if i == len(parts)-1 {
			if _, exists := cursor[part]; exists {
				return fmt.Errorf("duplicate config path %q", path)
			}
			cloned, err := cloneJSON(value)
			if err != nil {
				return fmt.Errorf("config path %q has unsupported default: %w", path, err)
			}
			cursor[part] = cloned
			return nil
		}
		next, exists := cursor[part]
		if !exists {
			child := map[string]any{}
			cursor[part] = child
			cursor = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("config path %q conflicts with %q", path, strings.Join(parts[:i+1], "."))
		}
		cursor = child
	}
	return nil
}
