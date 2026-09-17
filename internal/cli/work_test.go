// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
)

func TestConnectedWorkQueryAndLocalAPI(t *testing.T) {
	view := evidence.View{Version: "agenova.evidence/v0", RequestRef: "work-1", Request: &v0.ClaimRequest{}, Outcome: &evidence.Outcome{Status: "Succeeded", Text: "done"}}
	services := Services{
		ShowConnected: func(ref, state string) (evidence.View, error) {
			if ref != "work-1" || state != "test-state" {
				t.Fatalf("show %q %q", ref, state)
			}
			return view, nil
		},
		ListConnected: func(state string) ([]evidence.View, error) {
			if state != "test-state" {
				t.Fatalf("list state %q", state)
			}
			return []evidence.View{view}, nil
		},
		ConnectAPI: func(state string, port int, stdout, _ io.Writer) error {
			if state != "test-state" || port != 18088 {
				t.Fatalf("connect %q %d", state, port)
			}
			_, _ = stdout.Write([]byte("connected\n"))
			return nil
		},
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"agenova", "work", "list", "--state-dir", "test-state"}, "work-1\tSucceeded"},
		{[]string{"agenova", "work", "show", "work-1", "--state-dir", "test-state"}, "request: work-1"},
		{[]string{"agenova", "work", "show", "work-1", "--json", "--state-dir", "test-state"}, `"requestRef":"work-1"`},
		{[]string{"agenova", "api", "connect", "--port", "18088", "--state-dir", "test-state"}, "connected"},
	} {
		var out, errs bytes.Buffer
		if code := MainWithServices(test.args, &out, &errs, services); code != 0 || !strings.Contains(out.String(), test.want) {
			t.Fatalf("%v: exit %d, out %q, err %q", test.args, code, out.String(), errs.String())
		}
	}
}

func TestWorkQueryFailuresAreNotSuccess(t *testing.T) {
	services := Services{ShowConnected: func(string, string) (evidence.View, error) { return evidence.View{}, errors.New("not found") }}
	for _, args := range [][]string{
		{"agenova", "work", "show", "missing"},
		{"agenova", "work", "list"},
		{"agenova", "work", "show"},
		{"agenova", "api", "connect", "--port", "0"},
		{"agenova", "run", "--port", "8088", "-f", "work.yaml"},
	} {
		var out, errs bytes.Buffer
		if code := MainWithServices(args, &out, &errs, services); code == 0 || out.Len() != 0 || errs.Len() == 0 {
			t.Fatalf("%v: exit %d, out %q, err %q", args, code, out.String(), errs.String())
		}
	}
}
