// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/examples/engineer/toolclient"
)

const fixtureDir = "../../harness/fixtures/contract/v0/inputs/claim-request"

// canonicalInvocations is the tool sequence declared by the canonical Team A
// engineer request (case IDs claim-request.valid.team-a-engineer-yaml/-json).
var canonicalInvocations = []toolclient.Invocation{
	{Tool: "git.read", Scope: "repo:acme/payments"},
	{Tool: "git.write", Scope: "repo:acme/payments"},
	{Tool: "github.pull-request", Scope: "repo:acme/payments"},
}

func runEngineer(t *testing.T, mock *toolclient.Mock, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, mock)
	return code, out.String(), errOut.String()
}

func assertCanonicalRun(t *testing.T, fixture string) {
	t.Helper()
	mock := &toolclient.Mock{}
	code, stdout, stderr := runEngineer(t, mock, "--task", filepath.Join(fixtureDir, fixture))
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, toolclient.FixtureModeLabel) {
		t.Errorf("output does not carry the fixture-mode label %q:\n%s", toolclient.FixtureModeLabel, stdout)
	}
	// The recorded mock invocations are the proof that every tool call went
	// through the ToolClient boundary; a bypassed call would print without
	// being recorded and fail here.
	if got := mock.Invocations(); !reflect.DeepEqual(got, canonicalInvocations) {
		t.Errorf("recorded invocations = %v, want %v", got, canonicalInvocations)
	}
	for _, invocation := range canonicalInvocations {
		line := "engineer: invoked tool=" + invocation.Tool + " scope=" + invocation.Scope + " result=ok"
		if !strings.Contains(stdout, line) {
			t.Errorf("output missing invocation line %q:\n%s", line, stdout)
		}
	}
}

func TestRunCanonicalYAMLFixture(t *testing.T) {
	assertCanonicalRun(t, "valid-team-a-engineer.yaml")
}

func TestRunCanonicalJSONFixture(t *testing.T) {
	assertCanonicalRun(t, "valid-team-a-engineer.json")
}

func TestRunInvalidMissingTaskFixture(t *testing.T) {
	mock := &toolclient.Mock{}
	fixture := filepath.Join(fixtureDir, "invalid-missing-task.json")
	code, stdout, stderr := runEngineer(t, mock, "--task", fixture)
	if code == exitOK {
		t.Fatalf("exit code = %d, want non-zero for invalid input", code)
	}
	if !strings.Contains(stderr, fixture) || !strings.Contains(stderr, "spec.task") {
		t.Errorf("stderr should name the file and the failing field, got: %s", stderr)
	}
	if got := mock.Invocations(); len(got) != 0 {
		t.Errorf("tool sequence must not execute on invalid input, recorded: %v", got)
	}
	if stdout != "" {
		t.Errorf("no fixture output expected on invalid input, got:\n%s", stdout)
	}
}

func TestRunMissingTaskFlag(t *testing.T) {
	mock := &toolclient.Mock{}
	code, _, stderr := runEngineer(t, mock)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "--task") {
		t.Errorf("stderr should explain the missing --task flag, got: %s", stderr)
	}
}

func TestRunConfiguredToolFailureExitsNonZero(t *testing.T) {
	mock := &toolclient.Mock{Results: map[string]error{
		"git.write": errors.New("denied by fixture configuration"),
	}}
	code, _, stderr := runEngineer(t, mock, "--task", filepath.Join(fixtureDir, "valid-team-a-engineer.yaml"))
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, "git.write") {
		t.Errorf("stderr should name the failing tool, got: %s", stderr)
	}
	// The failing call is still recorded; the sequence stops there.
	want := canonicalInvocations[:2]
	if got := mock.Invocations(); !reflect.DeepEqual(got, want) {
		t.Errorf("recorded invocations = %v, want %v", got, want)
	}
}
