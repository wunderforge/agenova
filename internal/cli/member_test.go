// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
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

func TestRememberedMemberServer(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		var out, errs bytes.Buffer
		target := "https://saved.example"
		args := []string{"agenova", "work", "list"}
		if explicit {
			target = "https://override.example"
			args = append(args, "--server", target)
		}
		called := false
		services := Services{DefaultServer: func() (string, error) {
			if explicit {
				t.Fatal("read default despite override")
			}
			return target, nil
		}, HTTPS: func(server string) (Services, error) {
			if server != target {
				t.Fatal(server)
			}
			return Services{ListConnected: func(state string) ([]evidence.View, error) {
				called = true
				if state != "" {
					t.Fatal("local state leaked")
				}
				return nil, nil
			}}, nil
		}}
		if code := MainWithServices(args, &out, &errs, services); code != 0 || !called {
			t.Fatalf("%d %s", code, errs.String())
		}
	}
}
func TestDefaultDoesNotAffectLocalOrHelp(t *testing.T) {
	for _, args := range [][]string{{"work", "list", "--state-dir", "local"}, {"run", "-f", "work.yaml", "--backend", "memory"}, {"platform", "--help"}, {"--version"}, {"--help"}} {
		var out, errs bytes.Buffer
		MainWithServices(append([]string{"agenova"}, args...), &out, &errs, Services{DefaultServer: func() (string, error) { t.Fatal("default read for local command"); return "", nil }})
	}
}

func TestInvalidDefaultDoesNotFallback(t *testing.T) {
	var out, errs bytes.Buffer
	code := MainWithServices([]string{"agenova", "work", "list"}, &out, &errs, Services{
		DefaultServer: func() (string, error) { return "", errors.New("invalid saved server") },
		ListConnected: func(string) ([]evidence.View, error) { t.Fatal("fell back to local state"); return nil, nil },
	})
	if code == 0 {
		t.Fatal("invalid default accepted")
	}
}
