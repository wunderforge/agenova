// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/credentials"
	k8scredentials "github.com/wunderforge/agenova/internal/credentials/kubernetes"
	"github.com/wunderforge/agenova/internal/platform"
)

// referenceCredentialSelection checks metadata only. The reference service has
// one HTTP model client, so every backend must select the same credential (or
// every backend must be uncredentialed). No Secret I/O occurs here.
func referenceCredentialSelection(resolved *platform.ResolvedPlatform, namespace string) (*v0.PlatformCredentialReference, error) {
	if resolved == nil {
		return nil, credentials.ErrConfiguration
	}
	adapters := map[string]platform.ResolvedAdapter{}
	for _, adapter := range resolved.Adapters {
		adapters[adapter.Name] = adapter
	}
	var resolver *platform.ResolvedInstance
	var selected *v0.PlatformCredentialReference
	haveModel := false
	for i := range resolved.Instances {
		instance := &resolved.Instances[i]
		if instance.CredentialRef != nil && instance.Category != platform.CapabilityModel {
			return nil, credentials.ErrConfiguration
		}
		if instance.Category == platform.CapabilityCredential {
			if resolver != nil {
				return nil, credentials.ErrConfiguration
			}
			resolver = instance
			identity := adapters[instance.AdapterRef]
			if identity.ID != k8scredentials.ResolverID || identity.Version != ReferenceVersion || len(instance.Config) != 1 || instance.Config["namespace"] != namespace {
				return nil, credentials.ErrConfiguration
			}
		}
		if instance.Category != platform.CapabilityModel {
			continue
		}
		ref := instance.CredentialRef
		if haveModel && ((ref == nil) != (selected == nil) || (ref != nil && *ref != *selected)) {
			return nil, credentials.ErrConfiguration
		}
		haveModel = true
		if ref != nil {
			identity := adapters[instance.AdapterRef]
			if identity.ID != OpenAICompatibleModelID || identity.Version != ReferenceVersion {
				return nil, credentials.ErrConfiguration
			}
			copied := *ref
			selected = &copied
		}
	}
	if selected == nil {
		if resolver != nil {
			return nil, credentials.ErrConfiguration
		}
		return nil, nil
	}
	if resolver == nil || selected.ResolverRef != resolver.Name {
		return nil, credentials.ErrConfiguration
	}
	ref := credentials.Reference{Resolver: k8scredentials.ResolverID, Name: selected.Name, Key: selected.Key}
	if credentials.ValidateReference(ref) != nil {
		return nil, credentials.ErrConfiguration
	}
	// The backend validates its private namespace/name/key rules without I/O.
	_, err := k8scredentials.New(namespace, []k8scredentials.Key{{Name: selected.Name, Key: selected.Key}}, func(context.Context, string, string) ([]byte, error) { return nil, credentials.ErrUnavailable })
	if err != nil {
		return nil, credentials.ErrConfiguration
	}
	return selected, nil
}

// ReferenceModelCredential captures an immutable host binding. Getter is the
// explicitly selected host identity, not input from a worker or request.
// Construction and Platform validation never retrieve a Secret.
func ReferenceModelCredential(resolved *platform.ResolvedPlatform, namespace string, get k8scredentials.Getter) (*credentials.Binding, error) {
	selected, err := referenceCredentialSelection(resolved, namespace)
	if err != nil || selected == nil {
		return nil, err
	}
	resolver, err := k8scredentials.New(namespace, []k8scredentials.Key{{Name: selected.Name, Key: selected.Key}}, get)
	if err != nil {
		return nil, credentials.ErrConfiguration
	}
	ref := credentials.Reference{Resolver: k8scredentials.ResolverID, Name: selected.Name, Key: selected.Key}
	registry, err := credentials.New([]credentials.Registration{{ID: k8scredentials.ResolverID, Version: ReferenceVersion, Resolver: resolver, Allowed: []credentials.Reference{ref}}})
	if err != nil {
		return nil, err
	}
	return registry.Bind(ref)
}

// CheckReferenceCredentials is an authorized readiness check used only by
// apply/startup. It resolves the selected key without making a provider call.
func CheckReferenceCredentials(ctx context.Context, resolved *platform.ResolvedPlatform, namespace string, get k8scredentials.Getter) error {
	binding, err := ReferenceModelCredential(resolved, namespace, get)
	if err != nil || binding == nil {
		return err
	}
	return binding.Use(ctx, func(context.Context, []byte) error { return nil })
}

func referenceSecretNames(resolved *platform.ResolvedPlatform, namespace string) ([]string, error) {
	selected, err := referenceCredentialSelection(resolved, namespace)
	if err != nil || selected == nil {
		return nil, err
	}
	return []string{selected.Name}, nil
}
