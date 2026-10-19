// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command terminalprobe is an opt-in real-backend #199 composition probe.
// It captures the production console's legitimate invocation callback while
// forwarding execution to the real Agent Sandbox worker, then replays it only
// after a terminal state. It is not an installed-service HTTP endpoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type capture struct {
	adapter               *agentsandbox.ControlledAdapter
	mode, cid, kubeconfig string
	mu                    sync.Mutex
	handler               workerprotocol.Handler
	task                  workerprotocol.Task
}

type templates struct{ template *v0.AgentTemplate }

func (t templates) Lookup(name string) (*v0.AgentTemplate, error) {
	if name != t.template.Metadata.Name {
		return nil, errors.New("unknown configured template")
	}
	return t.template, nil
}

func command(argv ...string) ([]byte, error) {
	return exec.Command(argv[0], argv[1:]...).CombinedOutput()
}

func providerCounts(cid string) (map[string]int, error) {
	data, err := command("docker", "logs", cid)
	if err != nil {
		return nil, err
	}
	result := map[string]int{"responses": 0, "completed": 0}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "[GIN]") && strings.Contains(line, "/v1/chat/completions") {
			result["responses"]++
			if strings.Contains(line, "| 200 |") {
				result["completed"]++
			}
		}
	}
	return result, nil
}

func (c *capture) Execute(ctx context.Context, id v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handler workerprotocol.Handler) (string, error) {
	c.mu.Lock()
	c.handler = handler
	c.task = task
	c.mu.Unlock()
	if c.mode == "expired" {
		if _, err := command("docker", "pause", c.cid); err != nil {
			return "", err
		}
		defer func() { _, _ = command("docker", "unpause", c.cid) }()
	}
	if c.mode == "failed" {
		args := []string{"kubectl", "--kubeconfig", c.kubeconfig, "--context", "kind-agenova-e15-199-faults", "-n", "agenova-e15-199-terminal", "get", "pod", id.WorkerID, "-o", "json"}
		data, err := command(args...)
		if err != nil {
			return "", err
		}
		var pod struct {
			Metadata struct {
				UID string `json:"uid"`
			} `json:"metadata"`
		}
		if err = json.Unmarshal(data, &pod); err != nil {
			return "", err
		}
		if pod.Metadata.UID == "" {
			return "", errors.New("missing owned pod UID")
		}
		body, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": map[string]string{"uid": pod.Metadata.UID}, "gracePeriodSeconds": 0})
		cmd := exec.Command("kubectl", "--kubeconfig", c.kubeconfig, "--context", "kind-agenova-e15-199-faults", "delete", "--raw", "/api/v1/namespaces/agenova-e15-199-terminal/pods/"+id.WorkerID, "-f", "-")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", err
		}
		if err = cmd.Start(); err != nil {
			return "", err
		}
		_, _ = stdin.Write(body)
		_ = stdin.Close()
		if err = cmd.Wait(); err != nil {
			return "", err
		}
	}
	return c.adapter.Execute(ctx, id, task, handler)
}

func run() error {
	mode := flag.String("mode", "succeeded", "succeeded, failed or expired")
	cid := flag.String("container-id", "", "dedicated ticket-owned provider ID")
	config := flag.String("kubeconfig", "", "dedicated cluster kubeconfig")
	output := flag.String("output", "", "evidence output file")
	flag.Parse()
	if (*mode != "succeeded" && *mode != "failed" && *mode != "expired") || *config == "" || *output == "" || *cid == "" {
		return errors.New("explicit mode, kubeconfig, container ID and output required")
	}
	data, err := command("docker", "inspect", *cid)
	if err != nil {
		return err
	}
	var containers []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err = json.Unmarshal(data, &containers); err != nil {
		return err
	}
	if len(containers) != 1 || containers[0].ID != *cid || containers[0].Name != "/agenova-e15-199-full-provider" || containers[0].Config.Labels["agenova.ticket"] != "199" {
		return errors.New("dedicated provider identity mismatch")
	}
	_ = os.Setenv("KUBECONFIG", *config)
	adapter := agentsandbox.NewControlled("kind-agenova-e15-199-faults", "agenova-e15-199-terminal")
	if err = adapter.AddTemplate(v0.AgentSandboxTemplate{Metadata: v0.ObjectMeta{Name: app.ReferenceRuntimeTemplateRef}, Spec: v0.AgentSandboxTemplateSpec{Image: "agenova-testworker:kind", Command: []string{"/agenova-workerctl", "serve"}}}); err != nil {
		return err
	}
	if err = adapter.AddWarmPool(v0.SandboxWarmPool{Metadata: v0.ObjectMeta{Name: "terminal-pool"}, Spec: v0.SandboxWarmPoolSpec{TemplateRef: app.ReferenceRuntimeTemplateRef, Replicas: 1}}); err != nil {
		return err
	}
	provider, err := modelprovider.New(modelprovider.Config{Endpoint: "http://127.0.0.1:11439/v1", Models: map[string]string{"coding-standard": "llama3.1:latest"}, OutputSchema: json.RawMessage(workerprotocol.FinishSchema), Timeout: time.Minute})
	if err != nil {
		return err
	}
	executor := &capture{adapter: adapter, mode: *mode, cid: *cid, kubeconfig: *config}
	templateData, err := os.ReadFile("deploy/reference/demo/engineer.yaml")
	if err != nil {
		return err
	}
	template, parseErr := v0.ParseAgentTemplateYAML(templateData)
	if parseErr != nil {
		return parseErr
	}
	policies := &policy.Loader{}
	if err = policies.Load(app.ReferencePolicy()); err != nil {
		return err
	}
	principals, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	if err != nil {
		return err
	}
	service, err := console.NewServiceWithOptions(adapter, executor, provider, app.ReferencePrincipalTeamA, console.Options{Prepare: func(data []byte) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principals, policies, templates{template: template})
	}})
	if err != nil {
		return err
	}
	defer service.Close()
	ref := "e15-199-terminal-" + *mode + "-" + fmt.Sprint(time.Now().UnixNano())
	duration := "90s"
	if *mode == "expired" {
		duration = "12s"
	}
	request := map[string]any{"apiVersion": "agenova.io/v1alpha1", "kind": "ClaimRequest", "metadata": map[string]string{"name": ref}, "spec": map[string]any{"templateRef": "engineer", "projectRef": "payments", "task": map[string]any{"type": "investigation", "input": map[string]string{"objective": "Explain why synthetic payment retries need a deadline. Do not use tools."}}, "requestedAccess": map[string]any{"tools": []string{"git.read"}, "resourceScopes": []string{"repo:acme/payments"}, "modelProfile": "coding-standard"}, "runtime": map[string]string{"profileRef": "standard-isolated", "timeout": duration}}}
	initialCounts, err := providerCounts(*cid)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(request)
	if _, err = service.Submit(body); err != nil {
		return err
	}
	end := time.Now().Add(140 * time.Second)
	for time.Now().Before(end) {
		view, e := service.QueryRequest(ref)
		if e != nil {
			return e
		}
		if view.Outcome == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		expected := map[string]string{"succeeded": "Succeeded", "failed": "Failed", "expired": "Expired"}[*mode]
		if view.State == nil || view.State.Claim == nil || string(view.State.Claim.Phase) != expected {
			return fmt.Errorf("unexpected terminal outcome: %v", view.Outcome)
		}
		executor.mu.Lock()
		handler, task := executor.handler, executor.task
		executor.mu.Unlock()
		if handler == nil {
			return errors.New("real execution never established a trusted handler")
		}
		time.Sleep(time.Second)
		beforeCounts, err := providerCounts(*cid)
		if err != nil {
			return err
		}
		attempts := []map[string]any{}
		for _, kind := range []string{"model", "tool"} {
			op := workerprotocol.Operation{ClaimID: task.ClaimID, Kind: kind, Profile: task.ModelProfile, Prompt: "synthetic terminal replay", Tool: "git.read", ResourceScope: "repo:acme/payments", Input: "README.md"}
			reply, replayErr := handler(context.Background(), op)
			attempts = append(attempts, map[string]any{"kind": kind, "allowed": reply.Allowed, "error": fmt.Sprint(replayErr)})
			if replayErr == nil || reply.Allowed {
				return errors.New("post-terminal trusted replay was not rejected")
			}
		}
		after, e := service.QueryRequest(ref)
		if e != nil {
			return e
		}
		beforeJSON, _ := json.Marshal(view)
		afterJSON, _ := json.Marshal(after)
		if string(beforeJSON) != string(afterJSON) {
			return errors.New("post-terminal replay changed facts")
		}
		time.Sleep(time.Second)
		afterCounts, err := providerCounts(*cid)
		if err != nil {
			return err
		}
		if beforeCounts["responses"] != afterCounts["responses"] || beforeCounts["completed"] != afterCounts["completed"] {
			return errors.New("provider log changed during terminal replay")
		}
		if *mode == "succeeded" && beforeCounts["completed"] <= initialCounts["completed"] {
			return errors.New("missing independent positive provider control")
		}
		result := map[string]any{"providerStart": initialCounts, "providerBeforeReplay": beforeCounts, "providerAfterReplay": afterCounts, "mode": *mode, "scope": "supplementary real-backend console composition, not installed HTTP replay", "evidence": view, "replays": attempts, "factsUnchanged": true}
		report, _ := json.MarshalIndent(result, "", "  ")
		if err = os.WriteFile(*output, append(report, '\n'), 0600); err != nil {
			return err
		}
		return nil
	}
	return errors.New("bounded real-backend probe did not finish")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
