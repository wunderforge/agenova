// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const executionLineLimit = 128 * 1024
const executionOperationLimit = 8

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
	if r.context == "" || r.namespace == "" || !isDNSLabel(workerID) || len(workerID) > 63 {
		return nil, errors.New("worker execution requires explicit context, namespace and valid pod")
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := writeExecutionLine(writer, task); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), executionLineLimit)
	var result, modelText string
	operations := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if result != "" {
			return "", errors.New("worker sent records after final result")
		}
		var message workerprotocol.Message
		if err := decodeExecutionLine(scanner.Bytes(), &message); err != nil {
			return "", err
		}
		if (message.Operation == nil) == (message.Result == "") {
			return "", errors.New("worker record requires exactly one operation or result")
		}
		if message.Operation == nil {
			if modelText == "" || message.Result != modelText {
				return "", errors.New("worker result lacks matching governed model response")
			}
			result = message.Result
			continue
		}
		op := *message.Operation
		operations++
		if operations > executionOperationLimit || op.ClaimID != task.ClaimID {
			return "", errors.New("worker operation limit or claim binding rejected")
		}
		if (op.Kind != "model" && op.Kind != "tool") || (op.Kind == "model" && (op.Profile != task.ModelProfile || op.Prompt != task.Objective)) {
			return "", errors.New("worker operation shape rejected")
		}
		reply, err := handle(ctx, op)
		if err != nil {
			return "", errors.New("worker governed operation failed")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if op.Kind == "model" {
			if !reply.Allowed || strings.TrimSpace(reply.Text) == "" || reply.Error != "" {
				return "", errors.New("worker model operation did not produce an allowed response")
			}
			modelText = reply.Text
		}
		if err := writeExecutionLine(writer, reply); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if scanner.Err() != nil {
		return "", errors.New("worker output malformed or exceeds limit")
	}
	if result == "" {
		return "", errors.New("worker exited without final result")
	}
	return result, nil
}

func writeExecutionLine(writer io.Writer, value any) error {
	line, err := json.Marshal(value)
	if err != nil || len(line)+1 >= executionLineLimit {
		return errors.New("worker input exceeds protocol limit")
	}
	if _, err := writer.Write(append(line, '\n')); err != nil {
		return errors.New("worker protocol input failed")
	}
	return nil
}

func decodeExecutionLine(line []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("worker output is invalid protocol JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("worker output contains trailing JSON")
	}
	return nil
}
