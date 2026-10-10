// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"reflect"
	"testing"

	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestBundledCredentialCatalogConstructAndNamespaceOnlyInit(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	marker, err := registry.Construct(KubernetesSecretCredentialID, ReferenceVersion, platform.CapabilityCredential)
	if err != nil || DescribeImplementation(marker) != KubernetesSecretCredentialID+"@"+ReferenceVersion {
		t.Fatal("credential marker is unavailable")
	}
	if _, ok := marker.(*KubernetesSecretCredential); !ok {
		t.Fatal("credential construction returned wrong marker")
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, adapterregistry.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	fragment := mustInit(t, lifecycle, KubernetesSecretCredentialID, "host-secrets")
	if len(fragment.Spec.Services.CredentialResolvers) != 1 || !reflect.DeepEqual(fragment.Spec.Services.CredentialResolvers[0].Config, map[string]any{"namespace": "agenova-system"}) {
		t.Fatal("credential init invented selection or unsupported config")
	}
	descriptor, ok := registry.Lookup(KubernetesSecretCredentialID, ReferenceVersion)
	if !ok {
		t.Fatal("credential descriptor missing")
	}
	for _, config := range []map[string]any{
		{}, {"namespace": ""}, {"namespace": "../outside"}, {"namespace": "default/other"}, {"namespace": "Uppercase"},
		{"namespace": "agenova-system", "allowed": []any{"model-key"}},
		{"namespace": "agenova-system", "credentialRef": "hidden"},
	} {
		if _, err := descriptor.CanonicalizeInstance(platform.CapabilityCredential, config); err == nil {
			t.Fatal("resolver canonicalizer accepted unsupported config")
		}
	}
}
