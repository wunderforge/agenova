// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Host-owned binaries only. No executable, credentials or endpoints come from a task.
type liveConfig struct{ Worker, MCP, MCPListen, Model string }
type localProcessExecutor struct{ binary string }

type localResearchProvider struct {
	client  modelprovider.Client
	catalog []workerprotocol.Tool
}

func (p localResearchProvider) Complete(ctx context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	var shape struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(r.OutputSchema, &shape) == nil && shape.Properties["completion"] != nil {
		var err error
		r.OutputSchema, err = workerprotocol.GatewayResearchSchema(r.OutputSchema, p.catalog)
		if err != nil {
			return modelprovider.Result{}, err
		}
	}
	return p.client.Complete(ctx, r)
}

func localWorkerCatalog(tools *toolbackend.Set) []workerprotocol.Tool {
	var out []workerprotocol.Tool
	for _, d := range tools.Catalog().Entries() {
		out = append(out, workerprotocol.Tool{Operation: d.Operation, Description: d.Description, ResourceScope: d.ResourceScope, Parameter: d.Parameter, InputMode: d.InputMode, AllowedValues: d.AllowedValues, MaxBytes: d.MaxBytes})
	}
	return out
}

func startLiveTools(ctx context.Context, c *liveConfig) (*toolbackend.Set, func(), string, error) {
	host, _, err := net.SplitHostPort(c.MCPListen)
	if err != nil || host != "127.0.0.1" {
		return nil, nil, "", errors.New("live MCP requires literal loopback")
	}
	listener, err := net.Listen("tcp", c.MCPListen)
	if err != nil {
		return nil, nil, "", errors.New("dedicated live MCP port is unavailable")
	}
	listener.Close()
	binary, err := filepath.Abs(c.MCP)
	if err != nil {
		return nil, nil, "", err
	}
	root, err := filepath.Abs("harness/demo/incident-mcp/data")
	if err != nil {
		return nil, nil, "", err
	}
	receipts, err := os.CreateTemp(".tmp", "live-mcp-receipts-*.jsonl")
	if err != nil {
		return nil, nil, "", err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		receipts.Close()
		return nil, nil, "", err
	}
	instance := "local-" + hex.EncodeToString(nonce[:])
	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(localProcessEnv(), "FIXTURE_ADDR="+c.MCPListen, "FIXTURE_DATA_DIR="+root, "FIXTURE_RESEARCH_ENABLED=true", "POD_NAME="+instance)
	cmd.Stdout = receipts
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		receipts.Close()
		return nil, nil, "", errors.New("live MCP process could not start")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		receipts.Close()
	}
	client := http.Client{Timeout: time.Second}
	ready := false
	for i := 0; i < 40; i++ {
		select {
		case <-done:
			receipts.Close()
			return nil, nil, "", errors.New("live MCP exited during startup")
		case <-ctx.Done():
			stop()
			return nil, nil, "", ctx.Err()
		default:
		}
		res, e := client.Get("http://" + c.MCPListen + "/healthz")
		if e == nil {
			ready = res.StatusCode == 200 && res.Header.Get("X-Agenova-Fixture-Instance") == instance
			res.Body.Close()
			if ready {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		stop()
		return nil, nil, "", errors.New("owned live MCP did not become ready")
	}
	tools, err := researchToolsAt("http://" + c.MCPListen + "/mcp")
	if err != nil {
		stop()
		return nil, nil, "", err
	}
	return tools, stop, receipts.Name(), nil
}

func localProcessEnv() []string {
	var env []string
	for _, key := range []string{"SystemRoot", "WINDIR", "SYSTEMDRIVE"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}

func (e localProcessExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	path, err := filepath.Abs(e.binary)
	if err != nil {
		return "", workerprotocol.ErrWorkerStart
	}
	cmd := exec.CommandContext(ctx, path, "--gateway-only")
	cmd.Env = localProcessEnv()
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	input, err := cmd.StdinPipe()
	if err != nil {
		return "", workerprotocol.ErrWorkerTransport
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return "", workerprotocol.ErrWorkerTransport
	}
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return "", workerprotocol.ErrWorkerStart
	}
	defer func() {
		input.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	guarded := func(call context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		// This is an application-level local guard, not evidence of OS/CNI isolation.
		if op.Kind == "native-start" || op.Kind == "native-end" {
			return workerprotocol.Reply{Allowed: false, Error: "direct networking unavailable in local acceptance"}, nil
		}
		return handle(call, op)
	}
	result, err := workerprotocol.Exchange(ctx, output, input, task, guarded)
	if err != nil {
		return "", err
	}
	input.Close()
	if err = cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", workerprotocol.ErrNoFinalResult
	}
	return result, nil
}
