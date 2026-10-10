// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"reflect"
	"testing"

	"github.com/wunderforge/agenova/internal/platform"
)

func credentialRegistration() Registration {
	registration := testRegistration("example.com/credential/scoped", "0.1.0")
	registration.Manifest.Capabilities = []platform.Capability{platform.CapabilityCredential}
	registration.Descriptor.Capabilities = []platform.Capability{platform.CapabilityCredential}
	registration.Factories = map[platform.Capability]Factory{platform.CapabilityCredential: func() (any, error) { return &struct{}{}, nil }}
	return registration
}

func TestCredentialRegistryActivationLockAndTypedFragment(t *testing.T) {
	registration := credentialRegistration()
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycle(registry, NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := lifecycle.Init(registration.Manifest.ID+"@0.1.0", "host-secrets")
	if err != nil || fragment.Spec.Services == nil || len(fragment.Spec.Services.CredentialResolvers) != 1 || fragment.Spec.Infrastructure != nil || len(fragment.Spec.Services.ModelProfiles) != 0 || len(fragment.Spec.Services.ModelBackends) != 0 {
		t.Fatalf("credential fragment failed: %#v %v", fragment, err)
	}
	if fragment.Spec.Services.CredentialResolvers[0].CredentialRef != nil {
		t.Fatal("init invented credential selection")
	}
	first, err := lifecycle.Install(registration.Manifest.ID + "@0.1.0")
	if err != nil || !first.Changed {
		t.Fatal("credential activation failed")
	}
	second, err := lifecycle.Install(registration.Manifest.ID + "@0.1.0")
	if err != nil || second.Changed {
		t.Fatal("credential activation not idempotent")
	}
	lock, err := lifecycle.List()
	if err != nil || len(lock.Adapters) != 1 || !reflect.DeepEqual(lock.Adapters[0].Capabilities, []platform.Capability{platform.CapabilityCredential}) {
		t.Fatal("credential lock did not preserve category")
	}
	if _, err := lifecycle.Construct(registration.Manifest.ID, "0.1.0", platform.CapabilityCredential); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Construct(registration.Manifest.ID, "0.1.0", platform.CapabilityModel); err == nil {
		t.Fatal("credential factory exposed model capability")
	}
}

func TestCredentialRegistryKeepsConfigSecretRejectionAndProfileBoundary(t *testing.T) {
	for _, field := range []string{"credentialRef", "apiKey", "nested.tokenRef"} {
		registration := credentialRegistration()
		registration.Manifest.InstanceSchema.Fields = append(registration.Manifest.InstanceSchema.Fields, Field{Path: field, Kind: ValueString})
		if registry, err := New(registration); registry != nil || err == nil {
			t.Fatal("credential category weakened schema rejection")
		}
	}
	registration := credentialRegistration()
	registration.Manifest.ProfileSchema.Fields = []Field{{Path: "name", Kind: ValueString}}
	if registry, err := New(registration); registry != nil || err == nil {
		t.Fatal("credential category accepted profile schema")
	}
}
