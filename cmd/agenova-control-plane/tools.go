// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/toolbackend"
)

type toolRegistry interface {
	platform.DescriptorLookup
	Construct(string, string, platform.Capability) (any, error)
}

// buildInstalledTools consumes the applied revision using the same registry
// descriptors/factories as Platform resolution. It never contacts a server.
func buildInstalledTools(resolved *platform.ResolvedPlatform, registry toolRegistry) (*toolbackend.Set, error) {
	if resolved == nil || registry == nil {
		return nil, fmt.Errorf("installed tool configuration is unavailable")
	}
	adapters := map[string]platform.ResolvedAdapter{}
	for _, adapter := range resolved.Adapters {
		if _, ok := adapters[adapter.Name]; ok {
			return nil, fmt.Errorf("duplicate installed adapter")
		}
		adapters[adapter.Name] = adapter
	}
	instances := map[string]platform.ResolvedInstance{}
	for _, instance := range resolved.Instances {
		if instance.Category == platform.CapabilityTool {
			if _, ok := instances[instance.Name]; ok {
				return nil, fmt.Errorf("duplicate installed tool backend")
			}
			instances[instance.Name] = instance
		}
	}
	profiles := map[string][]platform.ResolvedProfile{}
	profileNames := map[string]bool{}
	for _, profile := range resolved.Profiles {
		if profile.Capability != platform.CapabilityTool {
			continue
		}
		if _, ok := instances[profile.BackendRef]; !ok || profileNames[profile.Name] {
			return nil, fmt.Errorf("invalid installed tool profile reference")
		}
		profileNames[profile.Name] = true
		profiles[profile.BackendRef] = append(profiles[profile.BackendRef], profile)
	}
	if len(instances) == 0 && len(profiles) == 0 && len(resolved.ToolRoutes) == 0 {
		return nil, nil
	}
	var bindings []toolbackend.Binding
	var routes []platform.ToolRoute
	names := make([]string, 0, len(instances))
	for name := range instances {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		instance := instances[name]
		adapter, ok := adapters[instance.AdapterRef]
		if !ok {
			return nil, fmt.Errorf("installed tool adapter reference is missing")
		}
		descriptor, ok := registry.Lookup(adapter.ID, adapter.Version)
		if !ok || descriptor.DescribeTool == nil {
			return nil, fmt.Errorf("installed tool adapter is unsupported")
		}
		implementation, err := registry.Construct(adapter.ID, adapter.Version, platform.CapabilityTool)
		if err != nil {
			return nil, fmt.Errorf("installed tool adapter cannot be constructed")
		}
		factory, ok := implementation.(toolbackend.Factory)
		if !ok {
			return nil, fmt.Errorf("installed tool adapter has no provider factory")
		}
		backend, err := descriptor.CanonicalizeInstance(platform.CapabilityTool, copyToolConfig(instance.Config))
		if err != nil {
			return nil, fmt.Errorf("invalid installed tool backend")
		}
		configs := []map[string]any{}
		var backendRoutes []platform.ToolRoute
		for _, profile := range profiles[name] {
			config, err := descriptor.CanonicalizeProfile(platform.CapabilityTool, copyToolConfig(backend), copyToolConfig(profile.Config))
			if err != nil {
				return nil, fmt.Errorf("invalid installed tool profile")
			}
			tool, limit, err := descriptor.DescribeTool(copyToolConfig(backend), copyToolConfig(config))
			if err != nil {
				return nil, fmt.Errorf("invalid installed tool descriptor")
			}
			configs = append(configs, config)
			backendRoutes = append(backendRoutes, platform.ToolRoute{Profile: profile.Name, BackendRef: name, Tool: tool, MaxObservationBytes: limit})
		}
		provider, err := factory.NewToolProvider(backend, configs)
		if err != nil {
			return nil, fmt.Errorf("installed tool provider cannot be configured")
		}
		for _, route := range backendRoutes {
			bindings = append(bindings, toolbackend.Binding{Descriptor: route.Tool, Provider: provider, MaxObservationBytes: route.MaxObservationBytes})
			routes = append(routes, route)
		}
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Profile < routes[j].Profile })
	// Recompute the catalog instead of trusting a separately editable projection.
	if !reflect.DeepEqual(routes, resolved.ToolRoutes) {
		return nil, fmt.Errorf("installed tool catalog does not match its configuration")
	}
	return toolbackend.NewSet(bindings)
}
func copyToolConfig(config map[string]any) map[string]any {
	data, err := json.Marshal(config)
	if err != nil {
		return nil
	}
	var copy map[string]any
	if json.Unmarshal(data, &copy) != nil {
		return nil
	}
	return copy
}
