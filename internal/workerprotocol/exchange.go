// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const executionLineLimit = 128 * 1024
const executionOperationLimit = MaxOperations

// Exchange enforces the bounded Worker conversation independently of its transport.
func Exchange(ctx context.Context, reader io.Reader, writer io.Writer, task Task, handle Handler) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := ValidateTools(task.Tools); err != nil {
		return "", err
	}
	if err := writeExecutionLine(writer, task); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), executionLineLimit)
	var result, modelText string
	operations := 0
	modelTurns, observations := 0, 0
	protocol := ""
	nativeStarted, nativeFinished := false, false
	if task.Mode != "" && task.Mode != ReAct {
		return "", errors.New("unsupported worker mode")
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if result != "" {
			return "", errors.New("worker sent records after final result")
		}
		var message Message
		if err := decodeExecutionLine(scanner.Bytes(), &message); err != nil {
			return "", err
		}
		if (message.Operation == nil) == (message.Result == "") {
			return "", errors.New("worker record requires exactly one operation or result")
		}
		if message.Operation == nil {
			expected := modelText
			if task.Mode == ReAct {
				a, err := ParseAgentAction(modelText, task.Tools, protocol)
				legacyPremature := a.Completion == "" && (modelTurns < 2 || (len(task.Tools) > 0 && observations == 0))
				if err != nil || a.Action != "finish" || legacyPremature || (a.Completion == "completed" && observations == 0) || (nativeStarted && !nativeFinished) {
					return "", ErrInvalidFinalResult
				}
				expected = a.Answer
			}
			if modelText == "" || message.Result != expected {
				return "", ErrInvalidFinalResult
			}
			result = message.Result
			continue
		}
		op := *message.Operation
		if op.Kind == "model" {
			if modelTurns > 0 && op.AgentProtocol != protocol {
				return "", errors.New("agent protocol changed during execution")
			}
			if op.AgentProtocol != "" && op.AgentProtocol != ResearchProtocol {
				return "", errors.New("unsupported agent protocol")
			}
			protocol = op.AgentProtocol
		} else if op.AgentProtocol != "" {
			return "", errors.New("unexpected agent protocol field")
		}
		operations++
		expectedToken := task.ModelToken
		if op.Kind == "tool" {
			expectedToken = task.ToolToken
		}
		limit := executionOperationLimit
		if protocol == ResearchProtocol {
			limit += 2
		}
		if operations > limit || op.ClaimID != task.ClaimID || op.WorkloadToken != expectedToken {
			return "", errors.New("worker operation limit or claim binding rejected")
		}
		promptOK := op.Prompt == task.Objective
		if task.Mode == ReAct {
			promptOK = strings.HasPrefix(op.Prompt, PromptPrefix(task)) && len(op.Prompt) <= 60<<10
		}
		native := op.Kind == "native-start" || op.Kind == "native-end"
		if (op.Kind != "model" && op.Kind != "tool" && !native) || (op.Kind == "model" && (op.Profile != task.ModelProfile || !promptOK)) || (!native && op.Native != nil) {
			return "", errors.New("worker operation shape rejected")
		}
		if task.Mode == ReAct && op.Kind == "model" && modelTurns >= MaxTurns {
			return "", ErrTurnLimit
		}
		if task.Mode == ReAct && op.Kind == "tool" {
			// The call must repeat the model-selected action exactly, and that
			// action must name one entry of the host-issued Work catalog.
			a, err := ParseAgentAction(modelText, task.Tools, protocol)
			if err != nil || a.Action != "tool" || a.Tool != op.Tool || a.Resource != op.ResourceScope || a.Input != op.Input {
				return "", errors.New("tool lacks matching model-selected action")
			}
			if op.Profile != "" || op.Prompt != "" {
				return "", errors.New("worker tool shape rejected")
			}
		}
		if native {
			a, err := ParseAgentAction(modelText, task.Tools, protocol)
			if err != nil || a.Action != "native_https" || op.Input != a.Input || op.Profile != "" || op.Prompt != "" || op.Tool != "" || op.ResourceScope != "" {
				return "", errors.New("native report lacks matching model action")
			}
			if op.Kind == "native-start" {
				if nativeStarted || op.Native != nil {
					return "", errors.New("duplicate native attempt")
				}
				nativeStarted = true
			} else {
				if !nativeStarted || nativeFinished || !ValidNative(op.Native) {
					return "", errors.New("invalid native observation")
				}
				nativeFinished = true
			}
		}
		reply, err := handle(ctx, op)
		if err != nil {
			return "", errors.New("worker governed operation failed")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if op.Kind == "model" {
			modelTurns++
			if task.Mode == ReAct && modelTurns > MaxTurns {
				return "", ErrTurnLimit
			}
			if !reply.Allowed || strings.TrimSpace(reply.Text) == "" || reply.Error != "" {
				return "", errors.New("worker model operation did not produce an allowed response")
			}
			modelText = reply.Text
		}
		if op.Kind == "tool" && reply.Allowed && reply.Error == "" && reply.Text != "" {
			observations++
		}
		if op.Kind == "native-end" {
			if !reply.Allowed || reply.Error != "" {
				return "", errors.New("native evidence was not recorded")
			}
			if op.Native.Result == "received" && op.Native.StatusCode >= 200 && op.Native.StatusCode < 300 {
				observations++
			}
			modelText = ""
		}
		if task.Mode == ReAct && op.Kind == "tool" {
			modelText = ""
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
		if task.Mode == ReAct && modelTurns >= MaxTurns {
			return "", ErrTurnLimit
		}
		return "", ErrNoFinalResult
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
