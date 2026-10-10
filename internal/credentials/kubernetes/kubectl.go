// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
)

type commandFunc func(context.Context, ...string) ([]byte, error)

// NewKubectl uses the host's explicitly selected Kubernetes identity. It never
// reads an implicit default context or accepts a worker-selected namespace.
func NewKubectl(kubeContext string) (Getter, error) {
	return kubectlGetter(kubeContext, execKubectl)
}

func kubectlGetter(kubeContext string, command commandFunc) (Getter, error) {
	if strings.TrimSpace(kubeContext) != kubeContext || kubeContext == "" || len(kubeContext) > 256 || strings.ContainsAny(kubeContext, "\x00\r\n") || command == nil {
		return nil, credentials.ErrConfiguration
	}
	return func(ctx context.Context, namespace, name string) ([]byte, error) {
		if ctx == nil || !dnsLabel.MatchString(namespace) || !validName(name) {
			return nil, credentials.ErrRejected
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, credentials.CallTimeout)
		defer cancel()
		raw, err := command(ctx, "--context", kubeContext, "--namespace", namespace, "--request-timeout=5s", "get", "secret", name, "-o", "json")
		if ctx.Err() != nil {
			clear(raw)
			return nil, ctx.Err()
		}
		if err != nil || len(raw) == 0 || len(raw) > MaxJSONBytes {
			clear(raw)
			return nil, credentials.ErrUnavailable
		}
		return raw, nil
	}, nil
}

type boundedBuffer struct{ data bytes.Buffer }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > MaxJSONBytes-b.data.Len() {
		return 0, credentials.ErrUnavailable
	}
	return b.data.Write(data)
}

func execKubectl(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.WaitDelay = time.Second
	var stdout boundedBuffer
	defer func() { clear(stdout.data.Bytes()) }()
	cmd.Stdout, cmd.Stderr = &stdout, io.Discard
	if cmd.Run() != nil {
		return nil, credentials.ErrUnavailable
	}
	return bytes.Clone(stdout.data.Bytes()), nil
}
