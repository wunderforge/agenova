// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package connectedclient sends canonical Work through the installed private
// API using a temporary loopback-only Kubernetes port-forward.
package connectedclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"gopkg.in/yaml.v3"
)

type Client struct {
	Context      string
	Namespace    string
	Executable   string
	PollInterval time.Duration
	// OpenTunnel is a test seam; production uses only the fixed private API port.
	OpenTunnel func(context.Context) (endpoint string, close func(), err error)
	HTTPClient *http.Client
}

const maxEvidenceBytes = 1 << 20

var forwardedPort = regexp.MustCompile(`^Forwarding from 127\.0\.0\.1:([0-9]+) -> 8081$`)

// Show fetches one canonical evidence view from the installed Work API after
// the process that submitted it has exited. History is still process-local.
func (c Client) Show(ref string) (evidence.View, error) {
	if !validRequestRef(ref) {
		return evidence.View{}, fmt.Errorf("provide one bounded request reference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return evidence.View{}, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, nil, ref)
	if err != nil {
		return evidence.View{}, err
	}
	var view evidence.View
	if err := json.Unmarshal(response, &view); err != nil || !validEvidenceView(view, ref) {
		return evidence.View{}, fmt.Errorf("installed Work service returned invalid evidence")
	}
	return view, nil
}

// List reads only the installed service's bounded current-session records.
func (c Client) List() ([]evidence.View, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return nil, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	var views []evidence.View
	if err := json.Unmarshal(response, &views); err != nil || views == nil || len(views) > 32 {
		return nil, fmt.Errorf("installed Work service returned invalid list")
	}
	for _, view := range views {
		if !validRequestRef(view.RequestRef) || !validEvidenceView(view, view.RequestRef) {
			return nil, fmt.Errorf("installed Work service returned invalid list")
		}
	}
	return views, nil
}

func (c Client) RunFile(path string) (evidence.View, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return evidence.View{}, fmt.Errorf("read ClaimRequest: %w", err)
	}
	request, validationErr := v0.ParseClaimRequestYAML(data)
	if validationErr != nil {
		return evidence.View{}, validationErr
	}
	if !validRequestRef(request.Metadata.Name) {
		return evidence.View{}, errors.New("ClaimRequest reference is invalid")
	}
	var value map[string]any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return evidence.View{}, fmt.Errorf("decode ClaimRequest: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return evidence.View{}, fmt.Errorf("encode ClaimRequest: %w", err)
	}
	if _, validationErr := v0.ParseClaimRequestJSON(canonical); validationErr != nil {
		return evidence.View{}, validationErr
	}
	waitBudget := time.Duration(*request.Spec.Runtime.Timeout)
	if waitBudget > 28*time.Minute {
		waitBudget = 28 * time.Minute
	}
	// Submission may spend nearly three minutes in tunnel startup and the
	// installed service's synchronous registration/runtime setup before the
	// worker's runtime budget starts. Keep a separate margin for that phase.
	ctx, cancel := context.WithTimeout(context.Background(), waitBudget+4*time.Minute)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return evidence.View{}, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, canonical, "")
	if err != nil {
		return evidence.View{}, err
	}
	view, err := decodeView(response, request.Metadata.Name)
	if err != nil {
		return evidence.View{}, err
	}
	if view.Outcome != nil {
		return view, nil
	}
	interval := c.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return view, fmt.Errorf("waiting for Work evidence: %w", ctx.Err())
		case <-time.After(interval):
		}
		response, err = c.call(ctx, endpoint, nil, request.Metadata.Name)
		if err != nil {
			return view, err
		}
		view, err = decodeView(response, request.Metadata.Name)
		if err != nil {
			return evidence.View{}, err
		}
		if view.Outcome != nil {
			return view, nil
		}
	}
}

func decodeView(data []byte, expectedRef string) (evidence.View, error) {
	var view evidence.View
	if err := json.Unmarshal(data, &view); err != nil {
		return evidence.View{}, fmt.Errorf("decode installed Work evidence: %w", err)
	}
	if !validEvidenceView(view, expectedRef) {
		return evidence.View{}, errors.New("installed Work service returned incomplete or mismatched evidence")
	}
	return view, nil
}

func validEvidenceView(view evidence.View, ref string) bool {
	if view.Version != "agenova.evidence/v0" || view.RequestRef != ref || view.Request == nil ||
		view.Request.Metadata.Name != ref || v0.ValidateClaimRequest(view.Request) != nil || view.Facts == nil {
		return false
	}
	if view.State != nil && (view.State.RequestRef != ref || v0.ValidateIssuedState(view.State) != nil) {
		return false
	}
	for _, fact := range view.Facts {
		if fact.ID == "" || fact.Sequence == 0 || fact.Timestamp.IsZero() || fact.Kind == "" || fact.RequestRef != ref {
			return false
		}
		if fact.ClaimID != "" && (view.State == nil || view.State.Claim == nil || fact.ClaimID != view.State.Claim.ID) {
			return false
		}
		if fact.Decision != nil && (fact.Decision.ID == "" || fact.Decision.PrincipalRef == "" || fact.Decision.Action == "" || fact.Decision.Result == "" || fact.Decision.PolicyRef.ID == "" || fact.Decision.PolicyRef.Version == "") {
			return false
		}
		if fact.Decision != nil && !validDecisionResult(fact.Decision.Result) {
			return false
		}
		if fact.Result != "" && !validDecisionResult(fact.Result) {
			return false
		}
		if fact.PolicyRef != nil && (fact.PolicyRef.ID == "" || fact.PolicyRef.Version == "") {
			return false
		}
		if fact.Authority != nil && (fact.Authority.ID == "" || fact.Authority.Runtime.ProfileRef == "" || time.Duration(fact.Authority.Runtime.Timeout) <= 0) {
			return false
		}
		if fact.BackendIdentity != nil && (fact.BackendIdentity.Backend == "" || fact.BackendIdentity.WorkerID == "") {
			return false
		}
	}
	if view.Outcome != nil {
		if view.State == nil || !validOutcomeState(view.Outcome.Status, view.State) {
			return false
		}
		if view.Outcome.Model != nil && (view.Outcome.Model.InvocationID == "" || view.Outcome.Model.Model == "" || view.Outcome.Model.InputTokens < 0 || view.Outcome.Model.OutputTokens < 0) {
			return false
		}
	}
	return true
}

func validOutcomeState(status string, state *v0.IssuedState) bool {
	if state.Decision.Result != v0.DecisionResultAllow {
		return state.Claim == nil && status == string(state.Decision.Result)
	}
	if state.Claim == nil {
		return false
	}
	switch state.Claim.Phase {
	case v0.ClaimPhaseSucceeded, v0.ClaimPhaseFailed, v0.ClaimPhaseExpired:
		return status == string(state.Claim.Phase) || (state.Claim.Phase == v0.ClaimPhaseExpired && status == "Cancelled")
	default:
		return false
	}
}

func validDecisionResult(result v0.DecisionResult) bool {
	switch result {
	case v0.DecisionResultAllow, v0.DecisionResultDeny, v0.DecisionResultApprovalRequired:
		return true
	default:
		return false
	}
}

func validRequestRef(ref string) bool {
	if strings.TrimSpace(ref) == "" || len(ref) > 256 || strings.ContainsAny(ref, "/\\") {
		return false
	}
	for _, ch := range ref {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func (c Client) call(ctx context.Context, endpoint string, input []byte, ref string) ([]byte, error) {
	method, path := http.MethodPost, "/api/requests"
	if input == nil {
		method = http.MethodGet
		if ref != "" {
			if !validRequestRef(ref) {
				return nil, errors.New("Work reference is invalid")
			}
			path = "/api/requests/" + url.PathEscape(ref) + "/evidence"
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+path, bytes.NewReader(input))
	if err != nil {
		return nil, errors.New("installed Work API request is invalid")
	}
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout:   4 * time.Minute,
			Transport: &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("installed Work API is unavailable; check Platform status and local tunnel")
	}
	defer response.Body.Close()
	limit := maxEvidenceBytes
	if input == nil && ref == "" {
		// A bounded current-session list may contain up to 32 individual views.
		limit = 8 << 20
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("installed Work API response is unavailable or too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Only stable, locally defined diagnostics may reach the operator. Never
		// echo arbitrary API/provider/kubectl error bodies into the CLI.
		var problem struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(data, &problem) == nil {
			if message, ok := safeAPIDiagnostic[problem.Code]; ok {
				return nil, fmt.Errorf("%s (HTTP %d)", message, response.StatusCode)
			}
		}
		return nil, fmt.Errorf("installed Work API rejected request (HTTP %d)", response.StatusCode)
	}
	return data, nil
}

var safeAPIDiagnostic = map[string]string{
	"active_policy_unavailable":   "Active PolicyBundle is unavailable; register or repair the active policy.",
	"active_policy_invalid":       "Active PolicyBundle is invalid; register a valid policy version.",
	"agent_template_unavailable":  "AgentTemplate is unavailable; register the requested template.",
	"assignment_unavailable":      "Assignment could not be resolved; check the registered template and active policy.",
	"authority_missing":           "Issued effective authority is missing; inspect policy and template configuration.",
	"model_profile_unavailable":   "Granted model profile is not installed; update the Platform model configuration.",
	"runtime_profile_unavailable": "Granted runtime profile is not installed; update the Platform runtime configuration.",
	"tool_unsupported":            "Granted tool is not supported by the installed Tool Gateway; narrow the template or install a compatible gateway.",
	"memory_unsupported":          "Granted memory scope is not supported by the installed Memory Interface; narrow the template or install a compatible interface.",
	"request_conflict":            "This Work reference already exists; choose a new request name.",
	"capacity_reached":            "The installed service has reached its current-session Work limit.",
	"not_found":                   "Work was not found in the installed service.",
}

func (c Client) openTunnel(ctx context.Context) (string, func(), error) {
	if c.Context == "" || c.Namespace == "" || c.Namespace == "default" {
		return "", nil, errors.New("installed Kubernetes target is unavailable")
	}
	var endpoint string
	var closeTunnel func()
	var err error
	if c.OpenTunnel != nil {
		endpoint, closeTunnel, err = c.OpenTunnel(ctx)
	} else {
		endpoint, closeTunnel, err = c.startPortForward(ctx)
	}
	if err != nil {
		return "", nil, err
	}
	if closeTunnel == nil {
		closeTunnel = func() {}
	}
	parsed, parseErr := url.Parse(endpoint)
	if parseErr != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || endpoint != "http://127.0.0.1:"+parsed.Port() {
		closeTunnel()
		return "", nil, errors.New("installed Work API tunnel returned a non-loopback endpoint")
	}
	return endpoint, closeTunnel, nil
}

func (c Client) startPortForward(ctx context.Context) (string, func(), error) {
	path := c.Executable
	if path == "" {
		path = "kubectl"
	}
	forwardCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(forwardCtx, path, c.portForwardArgs()...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start")
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start; check kubectl installation")
	}
	go io.Copy(io.Discard, stderr)
	portReady := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		announced := false
		for scanner.Scan() {
			if match := forwardedPort.FindStringSubmatch(strings.TrimSpace(scanner.Text())); match != nil && !announced {
				port, err := strconv.Atoi(match[1])
				if err == nil && port > 0 && port <= 65535 {
					portReady <- port
					announced = true
				}
			}
		}
		if !announced {
			portReady <- 0
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case port := <-portReady:
		if port > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", port), stop, nil
		}
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
	stop()
	return "", nil, errors.New("installed Work API tunnel is unavailable; check Platform status and Kubernetes port-forward RBAC")
}

func (c Client) portForwardArgs() []string {
	return []string{"--context", c.Context, "--namespace", c.Namespace, "port-forward", "deployment/agenova-control-plane", ":8081", "--address", "127.0.0.1"}
}
