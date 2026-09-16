// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"fmt"
	"path"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

type Store interface {
	List() (InstallationLock, error)
	Install(InstalledAdapter) (changed bool, err error)
}

type Lifecycle struct {
	registry *Registry
	store    Store
}

func NewLifecycle(registry *Registry, store Store) (*Lifecycle, error) {
	if registry == nil {
		return nil, fmt.Errorf("adapter registry is required")
	}
	if store == nil {
		return nil, fmt.Errorf("adapter installation store is required")
	}
	return &Lifecycle{registry: registry, store: store}, nil
}

func (l *Lifecycle) Catalog() []Manifest { return l.registry.Catalog() }

func (l *Lifecycle) Lookup(id, version string) (platform.Descriptor, bool) {
	return l.registry.Lookup(id, version)
}

// Construct creates one capability implementation from the same immutable
// registry used for lookup and activation. Platform orchestration therefore
// cannot validate one registration and accidentally execute another.
func (l *Lifecycle) Construct(id, version string, capability platform.Capability) (any, error) {
	return l.registry.Construct(id, version, capability)
}

func (l *Lifecycle) List() (InstallationLock, error) {
	lock, err := l.store.List()
	if err != nil {
		return InstallationLock{}, fmt.Errorf("read adapter installation lock: %w", err)
	}
	return normalizeLock(lock)
}

func (l *Lifecycle) Inspect(reference string) (InspectResult, error) {
	registration, err := l.registry.Resolve(reference)
	if err != nil {
		return InspectResult{}, err
	}
	lock, err := l.List()
	if err != nil {
		return InspectResult{}, err
	}
	installed := false
	for _, item := range lock.Adapters {
		if item.ID == registration.Manifest.ID && item.Version == registration.Manifest.Version {
			installed = true
			break
		}
	}
	return InspectResult{Manifest: registration.Manifest, Installed: installed}, nil
}

func (l *Lifecycle) Install(reference string) (InstallResult, error) {
	registration, err := l.registry.Resolve(reference)
	if err != nil {
		return InstallResult{}, err
	}
	adapter := installedFromManifest(registration.Manifest)
	changed, err := l.store.Install(adapter)
	if err != nil {
		return InstallResult{}, fmt.Errorf("install adapter %s@%s: %w", adapter.ID, adapter.Version, err)
	}
	return InstallResult{Adapter: adapter, Changed: changed}, nil
}

func (l *Lifecycle) Init(reference, localName string) (PlatformFragment, error) {
	registration, err := l.registry.Resolve(reference)
	if err != nil {
		return PlatformFragment{}, err
	}
	if len(registration.Manifest.Capabilities) != 1 {
		return PlatformFragment{}, fmt.Errorf("adapter %s@%s has multiple capabilities; init requires an explicit capability", registration.Manifest.ID, registration.Manifest.Version)
	}
	if localName == "" {
		localName = path.Base(registration.Manifest.ID)
	}
	capability := registration.Manifest.Capabilities[0]
	if !validPlatformLocalName(localName) {
		return PlatformFragment{}, fmt.Errorf("invalid local adapter name %q", localName)
	}
	if (capability == platform.CapabilityRuntime || capability == platform.CapabilityModel) && !validPlatformLocalName(localName+"-profile") {
		return PlatformFragment{}, fmt.Errorf("generated profile name %q exceeds the Platform name limit", localName+"-profile")
	}
	instanceConfig, err := defaultsFromSchema(registration.Manifest.InstanceSchema)
	if err != nil {
		return PlatformFragment{}, fmt.Errorf("initialize instance config: %w", err)
	}
	canonicalInstance, err := registration.Descriptor.CanonicalizeInstance(capability, instanceConfig)
	if err != nil {
		return PlatformFragment{}, fmt.Errorf("initialize adapter config: adapter rejected defaults")
	}
	if validationErr := v1alpha1.ValidatePlatformConfig(canonicalInstance); validationErr != nil {
		return PlatformFragment{}, fmt.Errorf("initialize adapter config: adapter emitted forbidden configuration")
	}
	requirement := adapterRequirement(localName, registration.Manifest)
	instance := platformInstance(localName, localName, canonicalInstance)
	fragment := PlatformFragment{Spec: FragmentSpec{Adapters: []v1alpha1.PlatformAdapterRequirement{requirement}}}

	switch capability {
	case platform.CapabilityDeployment:
		fragment.Spec.Infrastructure = &FragmentInfrastructure{Deployment: &instance}
	case platform.CapabilityRuntime:
		profile, err := initializedProfile(registration, capability, instance, localName+"-profile")
		if err != nil {
			return PlatformFragment{}, err
		}
		fragment.Spec.Infrastructure = &FragmentInfrastructure{RuntimeBackends: []v1alpha1.PlatformInstance{instance}, RuntimeProfiles: []v1alpha1.PlatformProfile{profile}}
	case platform.CapabilityModel:
		profile, err := initializedProfile(registration, capability, instance, localName+"-profile")
		if err != nil {
			return PlatformFragment{}, err
		}
		fragment.Spec.Services = &FragmentServices{ModelBackends: []v1alpha1.PlatformInstance{instance}, ModelProfiles: []v1alpha1.PlatformProfile{profile}}
	default:
		return PlatformFragment{}, fmt.Errorf("adapter capability %q cannot initialize a Platform fragment", capability)
	}
	return fragment, nil
}

func adapterRequirement(name string, manifest Manifest) v1alpha1.PlatformAdapterRequirement {
	return v1alpha1.PlatformAdapterRequirement{Name: name, ID: manifest.ID, Version: manifest.Version}
}

func platformInstance(name, adapterRef string, config map[string]any) v1alpha1.PlatformInstance {
	return v1alpha1.PlatformInstance{Name: name, AdapterRef: adapterRef, Config: config}
}

func initializedProfile(registration Registration, capability platform.Capability, instance v1alpha1.PlatformInstance, name string) (v1alpha1.PlatformProfile, error) {
	profileConfig, err := defaultsFromSchema(registration.Manifest.ProfileSchema)
	if err != nil {
		return v1alpha1.PlatformProfile{}, fmt.Errorf("initialize profile config: %w", err)
	}
	backendConfig, err := cloneConfigMap(instance.Config)
	if err != nil {
		return v1alpha1.PlatformProfile{}, fmt.Errorf("initialize profile config: clone backend config: %w", err)
	}
	canonical, err := registration.Descriptor.CanonicalizeProfile(capability, backendConfig, profileConfig)
	if err != nil {
		return v1alpha1.PlatformProfile{}, fmt.Errorf("initialize profile config: adapter rejected defaults")
	}
	if validationErr := v1alpha1.ValidatePlatformConfig(canonical); validationErr != nil {
		return v1alpha1.PlatformProfile{}, fmt.Errorf("initialize profile config: adapter emitted forbidden configuration")
	}
	return v1alpha1.PlatformProfile{Name: name, BackendRef: instance.Name, Config: canonical}, nil
}

func validPlatformLocalName(value string) bool {
	return len(value) <= 63 && namePattern.MatchString(value)
}

func cloneConfigMap(input map[string]any) (map[string]any, error) {
	cloned, err := cloneJSON(input)
	if err != nil {
		return nil, err
	}
	result, ok := cloned.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configuration is not an object")
	}
	return result, nil
}

func defaultsFromSchema(schema ConfigSchema) (map[string]any, error) {
	result := map[string]any{}
	for _, field := range schema.Fields {
		if field.Default == nil {
			if field.Required {
				return nil, fmt.Errorf("required field %q has no initialization default", field.Path)
			}
			continue
		}
		if err := setPath(result, field.Path, field.Default); err != nil {
			return nil, err
		}
	}
	return result, nil
}
