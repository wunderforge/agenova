// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestReservedCredentialFieldNameUsesExactNormalizedVocabulary(t *testing.T) {
	for _, name := range []string{
		"githubToken",
		"GITHUB_TOKEN",
		"API_KEY",
		"AWS_SECRET_ACCESS_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_APPLICATION_CREDENTIALS",
	} {
		if !ReservedCredentialFieldName(name) {
			t.Errorf("%q was not reserved", name)
		}
	}
	for _, name := range []string{"objective", "tokenizer", "passwordPolicyRef", "secretsManagerArn"} {
		if ReservedCredentialFieldName(name) {
			t.Errorf("%q was reserved by fuzzy matching", name)
		}
	}
}

func TestFindReservedCredentialFieldNameIsStable(t *testing.T) {
	values := map[string]any{"password": "not-inspected", "API_KEY": "not-inspected"}
	for range 20 {
		if key, found := FindReservedCredentialFieldName(values); !found || key != "API_KEY" {
			t.Fatalf("FindReservedCredentialFieldName = %q, %v; want API_KEY, true", key, found)
		}
	}
}
