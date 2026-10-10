// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"reflect"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
)

func credentialReferencePlatform() (*v1alpha1.Platform, lookupMap) {
	input := referencePlatform()
	lookup := referenceLookup()
	const id = "agenova.io/credential/kubernetes-secret"
	input.Spec.Adapters = append(input.Spec.Adapters, v1alpha1.PlatformAdapterRequirement{Name: "secrets", ID: id, Version: "0.1.0"})
	input.Spec.Services.CredentialResolvers = []v1alpha1.PlatformInstance{{Name: "host-secrets", AdapterRef: "secrets", Config: map[string]any{"namespace": "agenova-system"}}}
	input.Spec.Services.ModelBackends[0].CredentialRef = &v1alpha1.PlatformCredentialReference{ResolverRef: "host-secrets", Name: "model-key", Key: "api-key"}
	lookup[id+"@0.1.0"] = Descriptor{ID: id, Version: "0.1.0", Capabilities: []Capability{CapabilityCredential}, CanonicalizeInstance: func(_ Capability, config map[string]any) (map[string]any, error) { return config, nil }}
	return input, lookup
}

func modelInstance(resolved *ResolvedPlatform) *ResolvedInstance {
	for i := range resolved.Instances {
		if resolved.Instances[i].Category == CapabilityModel {
			return &resolved.Instances[i]
		}
	}
	return nil
}

func TestPlatformCredentialResolutionSnapshotsRefsAndIncludesTheirDigests(t *testing.T) {
	input, lookup := credentialReferencePlatform()
	resolved, lock, err := Resolve(input, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Instances) != 4 || resolved.Instances[0].Category != CapabilityCredential {
		t.Fatal("credential instance was not resolved")
	}
	if err := VerifyResolvedLock(resolved, lock); err != nil {
		t.Fatal(err)
	}
	original := *input.Spec.Services.ModelBackends[0].CredentialRef
	input.Spec.Services.ModelBackends[0].CredentialRef.Name = "mutated-input"
	if got := modelInstance(resolved).CredentialRef; got == nil || *got != original {
		t.Fatal("caller retargeted captured reference")
	}
	encoded, marshalErr := CanonicalJSON(lock)
	if marshalErr != nil || strings.Contains(string(encoded), "model-key") || strings.Contains(string(encoded), "api-key") {
		t.Fatal("digest-only lock exposed credential selection")
	}
	changedInput, _ := credentialReferencePlatform()
	changedInput.Spec.Services.ModelBackends[0].CredentialRef.Key = "replacement-key"
	changed, changedLock, changedErr := Resolve(changedInput, lookup)
	if changedErr != nil || changed.Revision == resolved.Revision {
		t.Fatal("typed reference did not affect revision")
	}
	var oldDigest, newDigest string
	for _, item := range lock.Instances {
		if item.Category == CapabilityModel {
			oldDigest = item.ConfigDigest
		}
	}
	for _, item := range changedLock.Instances {
		if item.Category == CapabilityModel {
			newDigest = item.ConfigDigest
		}
	}
	if oldDigest == newDigest {
		t.Fatal("typed reference did not affect instance digest")
	}
	modelInstance(resolved).CredentialRef.Name = "retargeted-effective"
	if VerifyResolvedLock(resolved, lock) == nil {
		t.Fatal("effective reference drift accepted")
	}
}

func TestPlatformCredentialResolutionOrderingAndCapabilityFailures(t *testing.T) {
	first, lookup := credentialReferencePlatform()
	second, _ := credentialReferencePlatform()
	second.Spec.Adapters[0], second.Spec.Adapters[3] = second.Spec.Adapters[3], second.Spec.Adapters[0]
	a, al, ae := Resolve(first, lookup)
	b, bl, be := Resolve(second, lookup)
	if ae != nil || be != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(al, bl) {
		t.Fatal("adapter ordering changed credential resolution")
	}
	for _, capability := range []Capability{CapabilityModel, CapabilityRuntime, CapabilityDeployment} {
		input, badLookup := credentialReferencePlatform()
		descriptor := badLookup["agenova.io/credential/kubernetes-secret@0.1.0"]
		descriptor.Capabilities = []Capability{capability}
		badLookup[descriptor.ID+"@"+descriptor.Version] = descriptor
		resolved, lock, err := Resolve(input, badLookup)
		if resolved != nil || lock != nil || err == nil || err.Category != ErrorCapability {
			t.Fatal("wrong resolver capability returned partial resolution")
		}
	}
}
