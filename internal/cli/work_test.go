// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/platformapply"
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

func TestDeniedWorkHumanStatusAgreesWithPortal(t *testing.T) {
	denied := evidence.View{Version: "agenova.evidence/v0", RequestRef: "denied-work", Request: &v0.ClaimRequest{}, Outcome: &evidence.Outcome{Status: "Deny"}}
	services := Services{
		ShowConnected: func(string, string) (evidence.View, error) { return denied, nil },
		ListConnected: func(string) ([]evidence.View, error) { return []evidence.View{denied}, nil },
	}
	for _, args := range [][]string{{"agenova", "work", "list"}, {"agenova", "work", "show", "denied-work"}} {
		var out, errs bytes.Buffer
		if code := MainWithServices(args, &out, &errs, services); code != 0 || !strings.Contains(out.String(), "Denied") || strings.Contains(out.String(), "\tDeny\n") || strings.Contains(out.String(), "phase: Deny") {
			t.Fatalf("%v: exit %d, out %q, err %q", args, code, out.String(), errs.String())
		}
	}
}

func TestSucceededClaimWithoutOutcomeIsStillFinishing(t *testing.T) {
	view := evidence.View{Version: "agenova.evidence/v0", RequestRef: "finishing-work", Request: &v0.ClaimRequest{}, State: &v0.IssuedState{Claim: &v0.SandboxClaim{Phase: v0.ClaimPhaseSucceeded}}}
	if got := workPhase(view); got != "Finishing" {
		t.Fatalf("work phase = %q, want Finishing until the outcome is recorded", got)
	}
	view.Outcome = &evidence.Outcome{Status: "Succeeded"}
	if got := workPhase(view); got != "Succeeded" {
		t.Fatalf("work phase = %q, want final outcome", got)
	}
}

func TestBoundClaimIsStartingInHumanStatus(t *testing.T) {
	view := evidence.View{State: &v0.IssuedState{Claim: &v0.SandboxClaim{Phase: v0.ClaimPhaseBound}}}
	if got := workPhase(view); got != "Starting" {
		t.Fatalf("work phase = %q, want Starting", got)
	}
}

func TestPlatformStatusAdvertisesSelectedStateDirectory(t *testing.T) {
	const stateDir = `C:\Agenova's state`
	const command = `agenova api connect --state-dir 'C:\Agenova''s state'`
	for _, jsonOutput := range []bool{false, true} {
		var out, errs bytes.Buffer
		if code := printPlatformStatus(&out, &errs, platformapply.Plan{PlatformName: "reference"}, jsonOutput, stateDir); code != 0 {
			t.Fatalf("status exit %d: %s", code, errs.String())
		}
		if jsonOutput {
			var status struct {
				API struct {
					ConnectCommand string `json:"connectCommand"`
				} `json:"api"`
			}
			if err := json.Unmarshal(out.Bytes(), &status); err != nil || status.API.ConnectCommand != command {
				t.Fatalf("JSON command = %q, error = %v", status.API.ConnectCommand, err)
			}
		} else if !strings.Contains(out.String(), command) {
			t.Fatalf("human status omitted selected state directory: %s", out.String())
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
