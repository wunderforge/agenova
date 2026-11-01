// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaimModelBudgetStrictScalarAndLegacyOmission(t *testing.T) {
	for _, tc := range loadClaimRequestCases(t) {
		if tc.Expected.Outcome != "valid" || tc.Format != "yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(claimRequestFixtureRoot(), tc.Input))
		if err != nil {
			t.Fatal(err)
		}
		base := string(raw)
		if !strings.Contains(base, "  runtime:") {
			continue
		}
		legacy, e := ParseClaimRequestYAML(raw)
		if e != nil || legacy.Spec.Runtime.MaxModelCalls != nil {
			t.Fatal("legacy request changed", e)
		}
		for _, value := range []string{"1", "20", "0", "-1", "1001", "null", "true", "1.5", "\"20\"", "{}"} {
			text := strings.Replace(base, "  runtime:", "  runtime:\n    maxModelCalls: "+value, 1)
			r, e := ParseClaimRequestYAML([]byte(text))
			valid := value == "1" || value == "20"
			if valid && (e != nil || r.Spec.Runtime.MaxModelCalls == nil) {
				t.Fatal(value, e)
			}
			if !valid && e == nil {
				t.Fatal("unsafe budget accepted", value)
			}
		}
		return
	}
	t.Fatal("missing valid runtime fixture")
}
