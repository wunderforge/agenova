// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command acceptance records an explicitly authorized connected real-model
// rehearsal. It never installs resources, changes policy, reads credentials or
// treats this business-reliability report as a network/security acceptance.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/login"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type scenario struct {
	name, scope, file string
	files             []string
	// This is a mechanical screening rubric, not a claim of semantic grading.
	concepts [][]string
}

var scenarios = []scenario{
	{"payments", "repo:demo/payments", "work-payments.yaml", []string{"payments/INC-P1042/logs.txt", "payments/INC-P1042/retry.txt"}, [][]string{{"deadline", "budget", "timeout"}, {"retry", "backoff", "6 second"}, {"idempoten", "duplicate"}}},
	{"billing", "repo:demo/billing", "work-billing.yaml", []string{"billing/INC-B2081/logs.txt", "billing/INC-B2081/job-config.txt"}, [][]string{{"overlap", "concurren", "80"}, {"dedup", "unique", "idempoten", "lock"}}},
}

type receipt struct {
	Event       string `json:"event"`
	RPCMethod   string `json:"rpcMethod"`
	Tool        string `json:"tool"`
	File        string `json:"file"`
	Correlation string `json:"correlation"`
	Outcome     string `json:"outcome"`
}

type attempt struct {
	Campaign          string   `json:"campaign"`
	Scenario          string   `json:"scenario"`
	Number            int      `json:"number"`
	Started           string   `json:"started"`
	RequestRef        string   `json:"requestRef,omitempty"`
	ClaimID           string   `json:"claimId,omitempty"`
	ActorHash         string   `json:"actorHash,omitempty"`
	Policy            string   `json:"policy,omitempty"`
	Status            string   `json:"status"`
	Model             string   `json:"model,omitempty"`
	ProviderResponse  string   `json:"providerResponse,omitempty"`
	ModelCalls        int      `json:"modelCalls"`
	ToolInvocationIDs []string `json:"toolInvocationIds"`
	FilesRead         []string `json:"filesRead"`
	Answer            string   `json:"answer,omitempty"`
	Failures          []string `json:"failures"`
	ScreenPassed      bool     `json:"screenPassed"`
	HumanReviewNeeded bool     `json:"humanReviewNeeded"`
}

type config struct {
	cli, server, kubeContext, template, expectedModel, scenarioDir, out string
	execute                                                             bool
}

func main() {
	var c config
	flag.StringVar(&c.cli, "cli", "", "absolute path to the reviewed Agenova CLI")
	flag.StringVar(&c.server, "server", "", "explicit HTTPS origin; uses existing CLI login")
	flag.StringVar(&c.kubeContext, "context", "", "explicit kubectl context for read-only MCP log collection")
	flag.StringVar(&c.template, "template", "", "reviewed registered template shared by all 20 tasks")
	flag.StringVar(&c.expectedModel, "model", "", "exact real provider model ID expected in evidence")
	flag.StringVar(&c.scenarioDir, "scenario-dir", "harness/demo/incident-mcp", "canonical synthetic scenario directory")
	flag.StringVar(&c.out, "out", "", "new output directory; never overwritten")
	flag.BoolVar(&c.execute, "execute", false, "explicitly authorize 20 billable sequential tasks; no resource deployment")
	flag.Parse()
	if err := run(c); err != nil {
		// Errors deliberately exclude subprocess stderr, provider payloads,
		// authentication headers, Kubernetes configuration and secret contents.
		fmt.Fprintln(os.Stderr, "incident acceptance:", err)
		os.Exit(1)
	}
}

func run(c config) error {
	if !c.execute {
		return errors.New("no tasks submitted: pass -execute only after deployment and cost authorization")
	}
	if !filepath.IsAbs(c.cli) || !login.Origin(c.server) || c.kubeContext == "" || c.template == "" || c.expectedModel == "" || c.out == "" {
		return errors.New("cli absolute path, HTTPS server, context, template, exact model and new out directory are required")
	}
	if strings.Contains(strings.ToLower(c.expectedModel), "script") || strings.Contains(strings.ToLower(c.expectedModel), "mock") {
		return errors.New("a scripted/mock model is not real-model acceptance")
	}
	// Validate both canonical inputs before spending money; no arbitrary user
	// file discovery, environment files, token files or credential reads.
	requests := make([]*v0.ClaimRequest, len(scenarios))
	for i, s := range scenarios {
		data, err := os.ReadFile(filepath.Join(c.scenarioDir, s.file))
		if err != nil {
			return errors.New("canonical scenario input unavailable")
		}
		r, validation := v0.ParseClaimRequestYAML(data)
		if validation != nil || r.Spec.Runtime == nil || r.Spec.Runtime.Timeout == nil || time.Duration(*r.Spec.Runtime.Timeout) != 3*time.Minute || r.Spec.RequestedAccess.ModelProfile != "coding-standard" || len(r.Spec.RequestedAccess.ResourceScopes) != 1 || r.Spec.RequestedAccess.ResourceScopes[0] != s.scope || len(r.Spec.RequestedAccess.Tools) != 1 || r.Spec.RequestedAccess.Tools[0] != "repo.read" || len(r.Spec.RequestedAccess.NetworkProfiles) != 0 {
			return errors.New("canonical task scope/profile/three-minute timeout changed; review before rehearsal")
		}
		r.Spec.TemplateRef = c.template
		requests[i] = r
	}
	if err := os.Mkdir(c.out, 0700); err != nil {
		return errors.New("out must be a new directory with an existing parent; prior campaigns are never replaced")
	}
	f, err := os.OpenFile(filepath.Join(c.out, "attempts.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("cannot create exclusive campaign journal")
	}
	defer f.Close()
	journal := json.NewEncoder(f)
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return errors.New("campaign ID unavailable")
	}
	campaign := "incident-" + hex.EncodeToString(nonce[:])
	temp, err := os.MkdirTemp("", "agenova-incident-acceptance-")
	if err != nil {
		return errors.New("owned temporary request directory unavailable")
	}
	defer os.RemoveAll(temp) // only this process's fresh directory; no cloud cleanup
	// Read-only observer availability preflight before any submission. Actual
	// read receipts are the positive controls, not an empty successful log read.
	if _, err = collect(c); err != nil {
		return err
	}
	successes := map[string]int{}
	var actor, policy string
	for n := 1; n <= 10; n++ {
		for i, s := range scenarios {
			r := requests[i]
			r.Metadata.Name = fmt.Sprintf("%s-%s-%02d", campaign, s.name, n)
			data, _ := json.Marshal(r) // JSON is canonical YAML, no credentials
			path := filepath.Join(temp, s.name+".json")
			if err = os.WriteFile(path, data, 0600); err != nil {
				return errors.New("cannot prepare owned canonical request")
			}
			a := attempt{Campaign: campaign, Scenario: s.name, Number: n, Started: time.Now().UTC().Format(time.RFC3339Nano), Status: "Unknown", Failures: []string{}, ToolInvocationIDs: []string{}, FilesRead: []string{}, HumanReviewNeeded: true}
			// Persist an attempt START before calling the CLI: cancellation or a
			// crash cannot silently erase a failed/unknown attempt from the record.
			if err = journal.Encode(map[string]any{"event": "attempt-start", "attempt": a}); err != nil || f.Sync() != nil {
				return errors.New("campaign journal unavailable; dispatch stopped")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			output, cliErr := invoke(ctx, c.cli, "--server", c.server, "run", "-f", path, "--json")
			cancel()
			var v evidence.View
			if json.Unmarshal(output, &v) != nil || v.RequestRef == "" {
				a.Failures = append(a.Failures, "submission-or-result-unknown; no automatic retry")
			} else {
				receipts, receiptErr := collect(c)
				a = evaluate(a, s, v, receipts, c.expectedModel, c.template)
				if receiptErr != nil {
					a.Failures = append(a.Failures, "independent-receipt-collection-failed")
				}
			}
			if cliErr != nil {
				a.Failures = append(a.Failures, "CLI-returned-failure")
			}
			if a.ActorHash != "" && actor == "" {
				actor, policy = a.ActorHash, a.Policy
			}
			if a.ActorHash != actor || a.Policy != policy {
				a.Failures = append(a.Failures, "caller-or-policy-not-identical")
			}
			a.ScreenPassed = len(a.Failures) == 0
			if a.ScreenPassed {
				successes[s.name]++
			}
			if journal.Encode(map[string]any{"event": "attempt-result", "attempt": a}) != nil || f.Sync() != nil {
				return errors.New("campaign result journal unavailable; dispatch stopped")
			}
			fmt.Printf("%s %02d/10 status=%s screen=%t ref=%s\n", s.name, n, a.Status, a.ScreenPassed, a.RequestRef)
			// Unknown submission can still be executing remotely. Do not create
			// more Work or assert cleanup after merely killing the local client.
			if a.Status == "Unknown" || contains(a.Failures, "caller-or-policy-not-identical") || contains(a.Failures, "independent-receipt-collection-failed") || contains(a.Failures, "real-model-metadata-missing-or-unexpected") {
				return errors.New("campaign stopped; retained unknown, drifted or unverified attempt requires operator inspection")
			}
		}
	}
	passed := successes["payments"] >= 9 && successes["billing"] >= 9
	if journal.Encode(map[string]any{"event": "campaign-summary", "campaign": campaign, "attempts": 20, "screenPasses": successes, "mechanicalScreenPassed": passed, "humanReviewRequired": true, "securityAcceptance": "separate; NOT established by model behavior", "remoteCleanup": "not attempted; retain evidence and use owned-resource lifecycle"}) != nil || f.Sync() != nil {
		return errors.New("cannot commit campaign summary")
	}
	if !passed {
		return errors.New("business screen below 9/10 in at least one scenario; all attempts retained")
	}
	return nil
}

func invoke(ctx context.Context, binary string, args ...string) ([]byte, error) {
	var stdout limitedBuffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout, cmd.Stderr = &stdout, io.Discard
	err := cmd.Run()
	if stdout.exceeded {
		return nil, errors.New("bounded subprocess output exceeded")
	}
	return stdout.Bytes(), err
}

// 16 MiB bounds memory even if unrelated log traffic or a broken CLI is noisy.
type limitedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 16<<20 {
		b.exceeded = true
		return 0, errors.New("output limit")
	}
	return b.Buffer.Write(p)
}

func collect(c config) ([]receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := invoke(ctx, "kubectl", "--context", c.kubeContext, "-n", "agenova-e16", "logs", "deployment/e16-mcp", "-c", "mcp", "--tail=20000", "--limit-bytes=16777216")
	if err != nil {
		return nil, errors.New("read-only MCP observer unavailable; no tasks should be dispatched")
	}
	var result []receipt
	for _, line := range bytes.Split(data, []byte("\n")) {
		var r receipt
		if len(bytes.TrimSpace(line)) != 0 && json.Unmarshal(line, &r) != nil {
			return nil, errors.New("MCP receipt stream invalid or truncated")
		}
		if r.Event != "" {
			result = append(result, r)
		}
	}
	return result, nil
}

func evaluate(a attempt, s scenario, v evidence.View, receipts []receipt, expectedModel, template string) attempt {
	a.RequestRef = v.RequestRef
	if v.Outcome != nil {
		a.Status, a.Answer = v.Outcome.Status, v.Outcome.Text
		if v.Outcome.Model != nil {
			a.Model, a.ProviderResponse = v.Outcome.Model.Model, v.Outcome.Model.ResponseID
			if v.Outcome.Model.OutputTokens > 512 {
				a.Failures = append(a.Failures, "observed-final-model-output-exceeds-reviewed-cap")
			}
		}
	}
	if a.Status != "Succeeded" {
		a.Failures = append(a.Failures, "not-Succeeded")
	}
	if v.Request == nil || v.Request.Spec.TemplateRef != template || v.State == nil || v.State.Claim == nil || v.State.Claim.TemplateRef != template || v.State.Decision.Result != v0.DecisionResultAllow || v.State.EffectiveAuthority == nil {
		a.Failures = append(a.Failures, "missing-or-mismatched-authority")
		return a
	}
	a.ClaimID = v.State.Claim.ID
	principalJSON, _ := json.Marshal(v.State.Principal)
	sum := sha256.Sum256(principalJSON)
	a.ActorHash = hex.EncodeToString(sum[:8])
	a.Policy = v.State.PolicyRef.ID + "@" + v.State.PolicyRef.Version
	authority := v.State.EffectiveAuthority
	if len(authority.ResourceScopes) != 1 || authority.ResourceScopes[0] != s.scope || len(authority.Tools) != 1 || authority.Tools[0] != "repo.read" || authority.ModelProfile != "coding-standard" {
		a.Failures = append(a.Failures, "effective-scope-not-exact")
	}
	if len(authority.NetworkProfiles) != 0 {
		a.Failures = append(a.Failures, "unexpected-public-network-authority")
	}
	if a.Model != expectedModel || a.ProviderResponse == "" {
		a.Failures = append(a.Failures, "real-model-metadata-missing-or-unexpected")
	}
	for _, f := range v.Facts {
		if f.Kind != "ProviderAttempt" {
			continue
		}
		if f.ClaimID != a.ClaimID || f.RequestRef != a.RequestRef {
			a.Failures = append(a.Failures, "cross-Work-provider-fact")
		}
		if f.Operation == "model.invoke" {
			a.ModelCalls++
		}
		if f.Operation != "tool.invoke" {
			continue
		}
		if f.InvocationID == "" || contains(a.ToolInvocationIDs, f.InvocationID) {
			a.Failures = append(a.Failures, "missing-or-duplicate-tool-invocation-ID")
		}
		a.ToolInvocationIDs = append(a.ToolInvocationIDs, f.InvocationID)
		outcome := false
		for _, other := range v.Facts {
			if other.Kind == "ProviderOutcome" && other.Operation == "tool.invoke" && other.InvocationID == f.InvocationID && other.ClaimID == a.ClaimID && other.RequestRef == a.RequestRef && other.ProviderStatus == "Succeeded" {
				outcome = true
			}
		}
		if !outcome {
			a.Failures = append(a.Failures, "tool-provider-success-not-committed")
		}
		read, posts := 0, 0
		for _, r := range receipts {
			if r.Correlation != f.InvocationID {
				continue
			}
			if r.Event == "receipt" && r.RPCMethod == "tools/call" {
				posts++
			}
			if r.Event == "tool" && r.Tool == "read_file" && r.Outcome == "ok" {
				read++
				if !contains(s.files, r.File) {
					a.Failures = append(a.Failures, "receipt-read-outside-own-scope")
				} else if !contains(a.FilesRead, r.File) {
					a.FilesRead = append(a.FilesRead, r.File)
				}
			}
		}
		if read != 1 || posts != 1 {
			a.Failures = append(a.Failures, "missing-or-duplicate-correlated-MCP-receipt")
		}
	}
	if a.ModelCalls < 1 || a.ModelCalls > workerprotocol.MaxTurns {
		a.Failures = append(a.Failures, "model-call-bound-violated")
	}
	if len(a.FilesRead) != 2 {
		a.Failures = append(a.Failures, "both-evidence-files-not-read")
	}
	answer := strings.ToLower(a.Answer)
	for _, file := range s.files {
		if !strings.Contains(answer, strings.ToLower(filepath.Base(file))) {
			a.Failures = append(a.Failures, "answer-does-not-cite-each-file")
		}
	}
	for _, alternatives := range s.concepts {
		found := false
		for _, word := range alternatives {
			found = found || strings.Contains(answer, word)
		}
		if !found {
			a.Failures = append(a.Failures, "business-rubric-needs-review")
		}
	}
	return a
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
