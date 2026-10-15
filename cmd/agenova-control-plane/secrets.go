// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

// errTokenSecretUnavailable is the only error the reader returns, so neither a
// Secret value nor kubectl output can reach an error, a fact or a log.
var errTokenSecretUnavailable = errors.New("provisional token Secret is unavailable")

// maxSecretObjectBytes bounds the kubectl output kept in memory.
const maxSecretObjectBytes = 1 << 20

// kubectlSecretReader resolves the mcp-http provisional token reference (E16
// Slice 4) for the installed control plane: one bounded `kubectl get secret`
// in the install namespace per call, through the in-cluster kubeconfig and a
// Role that grants get on the referenced Secret names only. It is not a
// general credential resolver; #155 replaces it.
type kubectlSecretReader struct {
	namespace string
	kubectl   string
}

func (r kubectlSecretReader) ReadSecretKey(ctx context.Context, name, key string) ([]byte, error) {
	if r.namespace == "" || r.namespace == "default" || name == "" || key == "" {
		return nil, errTokenSecretUnavailable
	}
	path := r.kubectl
	if path == "" {
		path = "kubectl"
	}
	// KUBECONFIG is read when the command starts, so a reader built before
	// setInClusterKubeconfig still uses the in-cluster configuration.
	cmd := exec.CommandContext(ctx, path, "get", "secret", name, "--namespace", r.namespace, "--request-timeout=5s", "-o", "json")
	cmd.WaitDelay = time.Second
	stdout := &boundedBuffer{limit: maxSecretObjectBytes}
	cmd.Stdout = stdout // stderr is discarded: warnings would corrupt the JSON
	if err := cmd.Run(); err != nil || stdout.overflow {
		return nil, errTokenSecretUnavailable
	}
	var secret struct {
		Kind string            `json:"kind"`
		Data map[string]string `json:"data"`
	}
	if json.Unmarshal(stdout.data, &secret) != nil || secret.Kind != "Secret" {
		return nil, errTokenSecretUnavailable
	}
	encoded, ok := secret.Data[key]
	if !ok {
		return nil, errTokenSecretUnavailable
	}
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errTokenSecretUnavailable
	}
	return value, nil
}

// boundedBuffer keeps at most limit bytes and records an overflow instead of
// growing, so an unexpected object cannot exhaust memory.
type boundedBuffer struct {
	data     []byte
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > b.limit {
		b.overflow = true
		return len(p), nil
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
