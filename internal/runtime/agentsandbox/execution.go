// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const executionLineLimit = 128 * 1024
const executionOperationLimit = workerprotocol.MaxOperations

type workerSession interface {
	executeSession(context.Context, string, func(io.Reader, io.Writer) (string, error)) (string, error)
}

// Execute is an opt-in composition edge, not a RuntimeBackend operation.
// Kubernetes authenticates the pinned exec session; worker-supplied claim IDs
// are only nominations and must match the system-recorded session binding.
func (a *ControlledAdapter) Execute(ctx context.Context, id v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	if ctx == nil || handle == nil || strings.TrimSpace(task.Objective) == "" || task.ModelProfile == "" {
		return "", errors.New("invalid worker execution input")
	}
	entry, err := a.entryFor(id)
	if err != nil {
		return "", errors.New("worker execution identity is not registered")
	}
	if task.ClaimID != entry.claimID {
		return "", errors.New("worker execution claim binding mismatch")
	}
	checkRunning := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		a.mu.Lock()
		phase := a.phases[id.WorkerID]
		a.mu.Unlock()
		if phase != controlRunning {
			return errors.New("worker execution requires acknowledged running control")
		}
		return nil
	}
	if err := checkRunning(); err != nil {
		return "", err
	}
	obs, err := a.Observe(id)
	if err != nil || obs.Released || !obs.Ready || obs.ClaimID != task.ClaimID || obs.Identity != id {
		return "", errors.New("worker execution binding or readiness unconfirmed")
	}
	session, ok := a.control.(workerSession)
	if !ok {
		return "", errors.New("worker execution transport unsupported")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result, err := session.executeSession(ctx, id.WorkerID, func(reader io.Reader, writer io.Writer) (string, error) {
		return exchangeWorker(ctx, reader, writer, task, func(ctx context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			if err := checkRunning(); err != nil {
				return workerprotocol.Reply{}, err
			}
			return handle(ctx, op)
		})
	})
	if err != nil {
		return "", err
	}
	if err := checkRunning(); err != nil {
		return "", err
	}
	return result, nil
}

func (r *kubectlRunner) executionArgs(workerID string) ([]string, error) {
	if (r.context == "" && os.Getenv("KUBECONFIG") == "") || r.namespace == "" || !isDNSLabel(workerID) || len(workerID) > 63 {
		return nil, errors.New("worker execution requires configured Kubernetes access, namespace and valid pod")
	}
	return r.baseArgs("exec", "-i", "pod/"+workerID, "-c", "agent", "--", "/agenova-demo-worker"), nil
}

func (r *kubectlRunner) executeSession(ctx context.Context, workerID string, exchange func(io.Reader, io.Writer) (string, error)) (string, error) {
	args, err := r.executionArgs(workerID)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	return streamExecutionCommand(ctx, cmd, exchange)
}

func streamExecutionCommand(ctx context.Context, cmd *exec.Cmd, exchange func(io.Reader, io.Writer) (string, error)) (string, error) {
	callerContext := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd.WaitDelay = killWaitDelay
	cmd.Stderr = io.Discard // Authentication/plugin errors must not leak output.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", errors.New("worker execution input unavailable")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return "", errors.New("worker execution output unavailable")
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return "", errors.New("worker execution process could not start")
	}
	// Closing both pipes also unblocks exchange when a credential-plugin child
	// inherits stdout after kubectl is killed. WaitDelay alone cannot help until
	// exchange returns and Wait begins.
	closed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			stdin.Close()
			stdout.Close()
		case <-closed:
		}
	}()
	defer close(closed)
	result, exchangeErr := exchange(stdout, stdin)
	stdin.Close()
	if exchangeErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if exchangeErr != nil {
		if callerContext.Err() != nil {
			return "", callerContext.Err()
		}
		return "", exchangeErr
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if waitErr != nil {
		return "", errors.New("worker execution process failed")
	}
	return result, nil
}

func exchangeWorker(ctx context.Context, reader io.Reader, writer io.Writer, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	return workerprotocol.Exchange(ctx, reader, writer, task, handle)
}
