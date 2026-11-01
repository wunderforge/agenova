// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type captureLocalFormat struct{ schema json.RawMessage }

func (p *captureLocalFormat) Complete(_ context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	p.schema = r.OutputSchema
	return modelprovider.Result{}, nil
}
func TestLocalDecoderIsExplicitAndLeavesOtherWorkerFormatsUnchanged(t *testing.T) {
	capture := &captureLocalFormat{}
	client := localResearchProvider{client: capture}
	ordinary := json.RawMessage(workerprotocol.FinishSchema)
	if _, err := client.Complete(context.Background(), modelprovider.Request{OutputSchema: ordinary}); err != nil || string(capture.schema) != string(ordinary) {
		t.Fatal("ordinary Worker format changed", err)
	}
	research, _ := workerprotocol.ResearchSchema(nil, true)
	if _, err := client.Complete(context.Background(), modelprovider.Request{OutputSchema: research}); err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if json.Unmarshal(capture.schema, &shape) != nil || shape["anyOf"] == nil || shape["properties"] != nil {
		t.Fatal("local research profile was not separated")
	}
}

func TestLiveWorkerStartFailureIsSafeAndDoesNotCallGateway(t *testing.T) {
	called := false
	_, err := (localProcessExecutor{binary: filepath.Join(t.TempDir(), "missing-worker.exe")}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, workerprotocol.Task{}, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
		called = true
		return workerprotocol.Reply{}, nil
	})
	if !errors.Is(err, workerprotocol.ErrWorkerStart) || called {
		t.Fatal(err, called)
	}
}
func TestLocalWorkerEnvironmentDoesNotInheritCredentials(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-only-do-not-inherit")
	t.Setenv("OPENAI_API_KEY", "test-only-do-not-inherit")
	t.Setenv("CUSTOM_PRIVATE_VALUE", "test-only-do-not-inherit")
	for _, pair := range localProcessEnv() {
		key, _, _ := strings.Cut(pair, "=")
		if key != "SystemRoot" && key != "WINDIR" && key != "SYSTEMDRIVE" {
			t.Fatal("environment is not allowlisted", key)
		}
	}
}
func TestLiveToolsRequireDedicatedLiteralLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:5216", "localhost:5216", "https://example.com"} {
		_, stop, _, err := startLiveTools(context.Background(), &liveConfig{MCPListen: addr, MCP: os.Args[0]})
		if err == nil || stop != nil {
			t.Fatal("unsafe address accepted", addr)
		}
	}
}
