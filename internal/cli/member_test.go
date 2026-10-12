// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"github.com/wunderforge/agenova/internal/evidence"
	"testing"
)

func TestMemberServerNeverUsesOperatorState(t *testing.T) {
	var out, errout bytes.Buffer
	invoked := false
	services := Services{HTTPS: func(server string) (Services, error) {
		if server != "https://agenova.example" {
			t.Fatal(server)
		}
		return Services{ListConnected: func(state string) ([]evidence.View, error) {
			if state != "" {
				t.Fatal("operator state leaked")
			}
			invoked = true
			return []evidence.View{}, nil
		}}, nil
	}}
	if code := MainWithServices([]string{"agenova", "work", "list", "--server", "https://agenova.example"}, &out, &errout, services); code != 0 || !invoked {
		t.Fatalf("%d %s", code, &errout)
	}
	for _, args := range [][]string{{"platform", "apply", "--server", "https://agenova.example"}, {"work", "list", "--server", "https://agenova.example", "--state-dir", "operator"}, {"run", "--server", "https://agenova.example", "--backend", "memory"}, {"login"}} {
		out.Reset()
		errout.Reset()
		if code := MainWithServices(append([]string{"agenova"}, args...), &out, &errout, services); code != ExitUsage {
			t.Fatalf("accepted ambiguous/admin request: %v %d", args, code)
		}
	}
}
