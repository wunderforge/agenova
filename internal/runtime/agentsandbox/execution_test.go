// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

func executionTask() workerprotocol.Task {
	return workerprotocol.Task{ClaimID: "run-1", Objective: "Explain an invoice retry", ModelProfile: "coding-standard"}
}

func protocolLines(values ...any) string {
	var out strings.Builder
	for _, value := range values {
		line, _ := json.Marshal(value)
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.String()
}

func modelMessage(task workerprotocol.Task) workerprotocol.Message {
	return workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: task.Objective}}
}

func TestExecutionExchangeTaskDependentResult(t *testing.T) {
	task := executionTask()
	var output bytes.Buffer
	calls := 0
	result, err := exchangeWorker(context.Background(), strings.NewReader(protocolLines(modelMessage(task), workerprotocol.Message{Result: "Use bounded retries."})), &output, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		calls++
		if op.Prompt != task.Objective || op.Profile != task.ModelProfile || op.ClaimID != task.ClaimID {
			t.Fatalf("wrong task operation: %+v", op)
		}
		return workerprotocol.Reply{Allowed: true, Text: "Use bounded retries."}, nil
	})
	if err != nil || result != "Use bounded retries." || calls != 1 {
		t.Fatalf("result=%q err=%v calls=%d", result, err, calls)
	}
	decoder := json.NewDecoder(&output)
	var sent workerprotocol.Task
	var reply workerprotocol.Reply
	if decoder.Decode(&sent) != nil || sent != task || decoder.Decode(&reply) != nil || reply.Text != result {
		t.Fatal("task and governed response were not sent through stdio")
	}
}

func TestExecutionExchangeRejectsBeforeCallback(t *testing.T) {
	task := executionTask()
	foreign := modelMessage(task)
	foreign.Operation.ClaimID = "foreign-running-claim"
	wrongPrompt := modelMessage(task)
	wrongPrompt.Operation.Prompt = "different objective"
	for name, input := range map[string]string{
		"foreign":             protocolLines(foreign),
		"wrong prompt":        protocolLines(wrongPrompt),
		"malformed":           "{bad}\n",
		"unknown field":       "{\"result\":\"fake\",\"secret\":\"value\"}\n",
		"trailing JSON":       "{} {}\n",
		"oversized":           strings.Repeat("x", executionLineLimit) + "\n",
		"result before model": protocolLines(workerprotocol.Message{Result: "fake"}),
		"both":                protocolLines(workerprotocol.Message{Operation: modelMessage(task).Operation, Result: "fake"}),
		"missing":             "",
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_, err := exchangeWorker(context.Background(), strings.NewReader(input), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
				calls++
				return workerprotocol.Reply{}, nil
			})
			if err == nil || calls != 0 {
				t.Fatalf("err=%v callbacks=%d", err, calls)
			}
		})
	}
}

func TestExecutionExchangeNoLateOperationsOrFabricatedResult(t *testing.T) {
	task := executionTask()
	for name, input := range map[string]string{
		"duplicate result":  protocolLines(modelMessage(task), workerprotocol.Message{Result: "real"}, workerprotocol.Message{Result: "real"}),
		"late operation":    protocolLines(modelMessage(task), workerprotocol.Message{Result: "real"}, modelMessage(task)),
		"fabricated result": protocolLines(modelMessage(task), workerprotocol.Message{Result: "different"}),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_, err := exchangeWorker(context.Background(), strings.NewReader(input), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
				calls++
				return workerprotocol.Reply{Allowed: true, Text: "real"}, nil
			})
			if err == nil || calls != 1 {
				t.Fatalf("err=%v callbacks=%d", err, calls)
			}
		})
	}
}

func TestExecutionExchangeCancellationAndDenial(t *testing.T) {
	task := executionTask()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := exchangeWorker(ctx, strings.NewReader(protocolLines(modelMessage(task), modelMessage(task))), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
		calls++
		cancel()
		return workerprotocol.Reply{Allowed: true, Text: "late"}, nil
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v callbacks=%d", err, calls)
	}
	for _, reply := range []workerprotocol.Reply{{Allowed: false}, {Allowed: true}, {Allowed: true, Text: "real", Error: "failed"}} {
		_, err = exchangeWorker(context.Background(), strings.NewReader(protocolLines(modelMessage(task), workerprotocol.Message{Result: "fake"})), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) { return reply, nil })
		if err == nil {
			t.Fatal("denied, empty or failed model response succeeded")
		}
	}
}

func TestExecutionOperationLimitAndInputBound(t *testing.T) {
	task := executionTask()
	tool := workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: "git.read"}}
	calls := 0
	_, err := exchangeWorker(context.Background(), strings.NewReader(strings.Repeat(protocolLines(tool), executionOperationLimit+1)), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
		calls++
		return workerprotocol.Reply{Allowed: true}, nil
	})
	if err == nil || calls != executionOperationLimit {
		t.Fatalf("err=%v callbacks=%d", err, calls)
	}
	task.Objective = strings.Repeat("x", executionLineLimit)
	if _, err := exchangeWorker(context.Background(), strings.NewReader(""), io.Discard, task, nil); err == nil {
		t.Fatal("oversized task accepted")
	}
}

func TestExecutionPinsKubernetesArguments(t *testing.T) {
	runner := newKubectlRunner("kind-explicit", "owned-namespace")
	got, err := runner.executionArgs("bound-pod")
	want := []string{"--context", "kind-explicit", "--namespace", "owned-namespace", "exec", "-i", "pod/bound-pod", "-c", "agent", "--", "/agenova-demo-worker"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v err=%v", got, err)
	}
	for _, worker := range []string{"../foreign", "-flag", "", "a b"} {
		if _, err := runner.executionArgs(worker); err == nil {
			t.Fatalf("unsafe pod %q accepted", worker)
		}
	}
	runner.context = ""
	if _, err := runner.executionArgs("bound-pod"); err == nil {
		t.Fatal("ambient context accepted")
	}
}

func TestReActTransportKeepsFinalAndActionEvidence(t *testing.T) {
	task := executionTask()
	task.Mode = workerprotocol.ReAct
	task.ResourceScope = "repo:acme/payments"
	model := modelMessage(task)
	model.Operation.Prompt = workerprotocol.LoopPrompt(task, "")
	tool := workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: "git.read", ResourceScope: task.ResourceScope, Input: "README.md"}}
	for _, final := range []string{"Verified deadline fix.", "forged"} {
		calls := 0
		result, err := exchangeWorker(context.Background(), strings.NewReader(protocolLines(model, tool, model, workerprotocol.Message{Result: final})), io.Discard, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			calls++
			if calls == 1 {
				return workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"git.read","input":"README.md"}`}, nil
			}
			if op.Kind == "tool" {
				return workerprotocol.Reply{Allowed: true, Text: "mock deadline log"}, nil
			}
			return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Verified deadline fix."}`}, nil
		})
		if final == "forged" {
			if err == nil {
				t.Fatal("forged result accepted")
			}
		} else if err != nil || result != final || calls != 3 {
			t.Fatalf("result=%q err=%v calls=%d", result, err, calls)
		}
	}
	calls := 0
	if _, err := exchangeWorker(context.Background(), strings.NewReader(protocolLines(tool)), io.Discard, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
		calls++
		return workerprotocol.Reply{}, nil
	}); err == nil || calls != 0 {
		t.Fatal("tool not selected by model reached callback")
	}
}

type executionFakeControl struct {
	*fakeWorkerControl
	input string
	calls int
}

func (f *executionFakeControl) executeSession(ctx context.Context, worker string, exchange func(io.Reader, io.Writer) (string, error)) (string, error) {
	f.calls++
	if worker != f.worker {
		return "", errors.New("foreign worker")
	}
	return exchange(strings.NewReader(f.input), io.Discard)
}

func TestExecutionRequiresStartedAndObservedBinding(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	task := executionTask()
	control := &executionFakeControl{fakeWorkerControl: f, input: protocolLines(modelMessage(task), workerprotocol.Message{Result: "real"})}
	a.control = control
	callback := func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
		return workerprotocol.Reply{Allowed: true, Text: "real"}, nil
	}
	if _, err := a.Execute(context.Background(), alloc.Identity, task, callback); err == nil || control.calls != 0 {
		t.Fatal("execution allowed before start")
	}
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	wrong := task
	wrong.ClaimID = "foreign"
	if _, err := a.Execute(context.Background(), alloc.Identity, wrong, callback); err == nil || control.calls != 0 {
		t.Fatal("foreign task opened exec")
	}
	result, err := a.Execute(context.Background(), alloc.Identity, task, callback)
	if err != nil || result != "real" || control.calls != 1 {
		t.Fatalf("result=%q err=%v calls=%d", result, err, control.calls)
	}
	k.claims[resourceName("claim", f.claim)].Status.Sandbox.Name = "rebound"
	if _, err := a.Execute(context.Background(), alloc.Identity, task, callback); err == nil || control.calls != 1 {
		t.Fatal("stale identity opened exec")
	}
}

func TestExecutionCommandCancellationUnblocksPipes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecutionHelperProcess$")
	cmd.Env = append(os.Environ(), "AGENOVA_SYNTHETIC_EXECUTION_HELPER=wait")
	start := time.Now()
	_, err := streamExecutionCommand(ctx, cmd, func(reader io.Reader, writer io.Writer) (string, error) {
		_, err := io.Copy(io.Discard, reader)
		return "", err
	})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("cancellation did not stop blocked session: %v elapsed=%v", err, time.Since(start))
	}
}

func TestExecutionCommandFailureSanitizesStderr(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecutionHelperProcess$")
	cmd.Env = append(os.Environ(), "AGENOVA_SYNTHETIC_EXECUTION_HELPER=fail")
	_, err := streamExecutionCommand(ctx, cmd, func(reader io.Reader, writer io.Writer) (string, error) {
		io.Copy(io.Discard, reader)
		return "result", nil
	})
	if err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestExecutionHelperProcess(t *testing.T) {
	switch os.Getenv("AGENOVA_SYNTHETIC_EXECUTION_HELPER") {
	case "wait":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "fail":
		os.Stderr.WriteString("sensitive credential-plugin output")
		os.Exit(3)
	}
}
