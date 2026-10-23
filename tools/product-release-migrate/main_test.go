// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestRuntimeGrantScope(t *testing.T) {
	for _, role := range []string{"", "PUBLIC", "agenova_operator", "agenova_app;DROP TABLE x", `agenova_"other`} {
		if _, err := grantSQL(role); err == nil {
			t.Fatalf("accepted invalid role %q", role)
		}
	}
	sql, err := grantSQL("agenova_runtime")
	if err != nil || !strings.Contains(sql, `TO "agenova_runtime"`) || strings.Contains(sql, "DELETE") || strings.Contains(sql, "ALL") || strings.Contains(sql, "agenova_members") || strings.Count(sql, "GRANT ") != 3 {
		t.Fatal("runtime grant scope changed")
	}
}
