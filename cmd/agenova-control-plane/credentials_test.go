// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/credentials"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestInstalledCredentialStartupAndFreshInvocation(t *testing.T) {
	resolved := &platform.ResolvedPlatform{
		Adapters:  []platform.ResolvedAdapter{{Name: "model", ID: bundled.OpenAICompatibleModelID, Version: bundled.ReferenceVersion}, {Name: "secret", ID: bundled.KubernetesSecretCredentialID, Version: bundled.ReferenceVersion}},
		Instances: []platform.ResolvedInstance{{Category: platform.CapabilityModel, Name: "provider", AdapterRef: "model", CredentialRef: &v0.PlatformCredentialReference{ResolverRef: "host", Name: "provider-token", Key: "token"}}, {Category: platform.CapabilityCredential, Name: "host", AdapterRef: "secret", Config: map[string]any{"namespace": "agenova-test"}}},
	}
	gets := 0
	available := false
	value := []byte("synthetic-first")
	get := func(_ context.Context, namespace, name string) ([]byte, error) {
		gets++
		if namespace != "agenova-test" || name != "provider-token" {
			t.Fatal("unexpected Secret selection")
		}
		if !available {
			return nil, errors.New("private Secret failure")
		}
		return json.Marshal(map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"namespace": namespace, "name": name}, "type": "Opaque", "data": map[string][]byte{"token": value}})
	}
	binding, err := installedModelCredential(context.Background(), resolved, "agenova-test", get)
	if binding != nil || !errors.Is(err, credentials.ErrUnavailable) || gets != 1 {
		t.Fatalf("startup accepted missing material: %v %v %d", binding, err, gets)
	}
	available = true
	binding, err = installedModelCredential(context.Background(), resolved, "agenova-test", get)
	if binding == nil || err != nil || gets != 2 {
		t.Fatalf("startup positive control: %v %v %d", binding, err, gets)
	}
	value = []byte("synthetic-replacement")
	if err := binding.Use(context.Background(), func(_ context.Context, material []byte) error {
		if string(material) != "synthetic-replacement" {
			t.Fatal("startup cached material")
		}
		return nil
	}); err != nil || gets != 3 {
		t.Fatalf("fresh invocation: %v %d", err, gets)
	}
}
