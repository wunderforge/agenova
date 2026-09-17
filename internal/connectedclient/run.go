// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package connectedclient sends canonical Work through an installed reference
// service using Kubernetes exec authorization; it does not host a worker.
package connectedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"gopkg.in/yaml.v3"
)

type Client struct {
	Context      string
	Namespace    string
	Executable   string
	PollInterval time.Duration
	Invoke       func(context.Context, []byte, ...string) ([]byte, error)
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
	response, err := c.call(ctx, canonical, "submit")
	if err != nil {
		return evidence.View{}, err
	}
	var view evidence.View
	if err := json.Unmarshal(response, &view); err != nil {
		return evidence.View{}, fmt.Errorf("decode installed Work submission: %w", err)
	}
	if view.RequestRef != request.Metadata.Name {
		return evidence.View{}, fmt.Errorf("installed Work service returned a mismatched request")
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
		response, err = c.call(ctx, nil, "evidence", request.Metadata.Name)
		if err != nil {
			return view, err
		}
		if err := json.Unmarshal(response, &view); err != nil {
			return evidence.View{}, fmt.Errorf("decode installed Work evidence: %w", err)
		}
		if view.RequestRef != request.Metadata.Name {
			return evidence.View{}, fmt.Errorf("installed Work service returned a mismatched request")
		}
		if view.Outcome != nil {
			return view, nil
		}
	}
}

func (c Client) call(ctx context.Context, input []byte, command ...string) ([]byte, error) {
	if c.Context == "" || c.Namespace == "" || c.Namespace == "default" {
		return nil, errors.New("installed Kubernetes target is unavailable")
	}
	args := []string{"--context", c.Context, "--namespace", c.Namespace, "exec", "deployment/agenova-control-plane"}
	if input != nil {
		args = append(args, "-i")
	}
	args = append(args, "--", "/agenova-control-plane")
	args = append(args, command...)
	if c.Invoke != nil {
		return c.Invoke(ctx, input, args...)
	}
	path := c.Executable
	if path == "" {
		path = "kubectl"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		// Do not echo arbitrary provider/cluster stderr (it can include secrets).
		if strings.Contains(stderr.String(), "Forbidden") {
			return nil, fmt.Errorf("Kubernetes identity cannot execute in the installed Work service")
		}
		return nil, fmt.Errorf("installed Work service call failed; inspect Platform status and namespace access")
	}
	return stdout.Bytes(), nil
}
