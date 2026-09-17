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
	ctx, cancel := context.WithTimeout(context.Background(), waitBudget+2*time.Minute)
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
	if view.Version != "agenova.evidence/v0" || view.RequestRef != expectedRef {
		return evidence.View{}, errors.New("installed Work service returned mismatched evidence")
	}
	return view, nil
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
		if !validRequestRef(ref) {
			return nil, errors.New("Work reference is invalid")
		}
		method, path = http.MethodGet, "/api/requests/"+url.PathEscape(ref)+"/evidence"
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
			Timeout:   90 * time.Second,
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
	data, err := io.ReadAll(io.LimitReader(response.Body, maxEvidenceBytes+1))
	if err != nil || len(data) > maxEvidenceBytes {
		return nil, errors.New("installed Work API response is unavailable or too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Never print raw API, provider or kubectl output in a CLI error.
		return nil, fmt.Errorf("installed Work API rejected request (HTTP %d)", response.StatusCode)
	}
	return data, nil
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
