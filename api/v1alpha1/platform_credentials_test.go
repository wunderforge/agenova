// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func credentialPlatform(t *testing.T) *Platform {
	t.Helper()
	input, err := ParsePlatformYAML([]byte(validPlatformYAML))
	if err != nil {
		t.Fatal(err)
	}
	input.Spec.Adapters = append(input.Spec.Adapters, PlatformAdapterRequirement{Name: "secrets", ID: "agenova.io/credential/kubernetes-secret", Version: "0.1.0"})
	input.Spec.Services.CredentialResolvers = []PlatformInstance{{Name: "host-secrets", AdapterRef: "secrets", Config: map[string]any{"namespace": "agenova-system"}}}
	input.Spec.Services.ModelBackends[0].CredentialRef = &PlatformCredentialReference{ResolverRef: "host-secrets", Name: "model-key", Key: "api-key"}
	return input
}

func TestPlatformTypedCredentialReferenceYAMLAndJSON(t *testing.T) {
	input := credentialPlatform(t)
	jsonData, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, jsonErr := ParsePlatformJSON(jsonData)
	fromYAML, yamlErr := ParsePlatformYAML(yamlData)
	if jsonErr != nil || yamlErr != nil || !reflect.DeepEqual(input, fromJSON) || !reflect.DeepEqual(fromJSON, fromYAML) {
		t.Fatalf("typed reference round trip failed: JSON=%v YAML=%v", jsonErr, yamlErr)
	}
}

func TestPlatformTypedCredentialReferenceRejectsUnsupportedPlacementAndMetadata(t *testing.T) {
	for _, test := range []struct {
		name     string
		mutate   func(*Platform)
		category ValidationCategory
		path     string
	}{
		{"deployment", func(p *Platform) {
			p.Spec.Infrastructure.Deployment.CredentialRef = p.Spec.Services.ModelBackends[0].CredentialRef
		}, ValidationCategorySecretValue, "spec.infrastructure.deployment.credentialRef"},
		{"runtime", func(p *Platform) {
			p.Spec.Infrastructure.RuntimeBackends[0].CredentialRef = p.Spec.Services.ModelBackends[0].CredentialRef
		}, ValidationCategorySecretValue, "spec.infrastructure.runtimeBackends[0].credentialRef"},
		{"resolver", func(p *Platform) {
			p.Spec.Services.CredentialResolvers[0].CredentialRef = p.Spec.Services.ModelBackends[0].CredentialRef
		}, ValidationCategorySecretValue, "spec.services.credentialResolvers[0].credentialRef"},
		{"unknown resolver", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.ResolverRef = "unknown" }, ValidationCategoryInvalidValue, "spec.services.modelBackends[0].credentialRef.resolverRef"},
		{"empty name", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.Name = "" }, ValidationCategoryRequiredField, "spec.services.modelBackends[0].credentialRef.name"},
		{"empty key", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.Key = "" }, ValidationCategoryRequiredField, "spec.services.modelBackends[0].credentialRef.key"},
		{"control name", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.Name = "name\x00" }, ValidationCategoryInvalidValue, "spec.services.modelBackends[0].credentialRef.name"},
		{"unicode space key", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.Key = "key\u2003key" }, ValidationCategoryInvalidValue, "spec.services.modelBackends[0].credentialRef.key"},
		{"oversized key", func(p *Platform) { p.Spec.Services.ModelBackends[0].CredentialRef.Key = strings.Repeat("k", 257) }, ValidationCategoryInvalidValue, "spec.services.modelBackends[0].credentialRef.key"},
		{"duplicate resolver", func(p *Platform) {
			p.Spec.Services.CredentialResolvers = append(p.Spec.Services.CredentialResolvers, p.Spec.Services.CredentialResolvers[0])
		}, ValidationCategoryInvalidValue, "spec.services.credentialResolvers[1].name"},
		{"nested credential ref", func(p *Platform) {
			p.Spec.Services.ModelBackends[0].Config["nested"] = map[string]any{"credentialRef": "other"}
		}, ValidationCategorySecretValue, "spec.services.modelBackends[0].config.nested.credentialRef"},
		{"resolver raw secret", func(p *Platform) { p.Spec.Services.CredentialResolvers[0].Config["token"] = "synthetic-secret" }, ValidationCategorySecretValue, "spec.services.credentialResolvers[0].config.token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := credentialPlatform(t)
			test.mutate(input)
			err := ValidatePlatform(input)
			if err == nil || err.Category != test.category || err.FieldPath != test.path {
				t.Fatalf("validation=%v, want %s at %s", err, test.category, test.path)
			}
			data, marshalErr := json.Marshal(input)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, err := ParsePlatformJSON(data); err == nil {
				t.Fatal("JSON parser accepted rejected reference")
			}
		})
	}
}

func TestPlatformTypedCredentialReferenceRejectsUnknownValueChannels(t *testing.T) {
	input := credentialPlatform(t)
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{
		`"credentialRef":"model-key"`,
		`"credentialRef":{"resolverRef":"host-secrets","name":"model-key","key":"api-key","value":"synthetic-secret"}`,
		`"credentialRef":{"resolverRef":"host-secrets","name":"model-key","key":"api-key","namespace":"elsewhere"}`,
	} {
		document := strings.Replace(string(data), `"credentialRef":{"resolverRef":"host-secrets","name":"model-key","key":"api-key"}`, replacement, 1)
		if document == string(data) {
			t.Fatal("reference fixture was not replaced")
		}
		if _, err := ParsePlatformJSON([]byte(document)); err == nil {
			t.Fatal("unknown credential channel accepted")
		}
	}
}
