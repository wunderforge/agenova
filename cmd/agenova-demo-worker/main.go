// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const lineLimit = 128 * 1024

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		os.Stderr.WriteString("demo worker task failed\n")
		os.Exit(1)
	}
}

func run(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), lineLimit)
	var task workerprotocol.Task
	if err := readLine(scanner, &task); err != nil {
		return err
	}
	if task.ClaimID == "" || task.ModelProfile == "" || strings.TrimSpace(task.Objective) == "" {
		return errors.New("invalid task")
	}
	if task.Mode == workerprotocol.ReAct {
		return reactLoop(scanner, output, task)
	}
	if task.Mode != "" {
		return errors.New("unsupported agent mode")
	}
	if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: task.Objective}}); err != nil {
		return err
	}
	var reply workerprotocol.Reply
	if err := readLine(scanner, &reply); err != nil {
		return err
	}
	if !reply.Allowed || reply.Error != "" || strings.TrimSpace(reply.Text) == "" {
		return errors.New("model request failed")
	}
	return writeLine(output, workerprotocol.Message{Result: reply.Text})
}

func reactLoop(scanner *bufio.Scanner, output io.Writer, task workerprotocol.Task) error {
	if workerprotocol.ValidateTools(task.Tools) != nil {
		return errors.New("invalid tool catalog")
	}
	transcript := ""
	observations := 0
	readInputs := []string{}
	seen := map[string]bool{}
	for turn := 1; turn <= workerprotocol.MaxTurns; turn++ {
		progress := fmt.Sprintf("Current model turn: %d of %d. Successful observations: %d. Already read inputs: %q.\n", turn, workerprotocol.MaxTurns, observations, readInputs)
		promptTask := task
		if len(readInputs) == workerprotocol.ToolInputs(task.Tools) || turn == workerprotocol.MaxTurns {
			promptTask.Tools = nil
		}
		prompt := workerprotocol.LoopPrompt(promptTask, progress+transcript)
		if len(prompt) > 60<<10 {
			return errors.New("transcript limit reached")
		}
		if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: prompt}}); err != nil {
			return err
		}
		var reply workerprotocol.Reply
		if err := readLine(scanner, &reply); err != nil {
			return err
		}
		if !reply.Allowed || reply.Error != "" || strings.TrimSpace(reply.Text) == "" {
			return errors.New("model request failed")
		}
		action, err := workerprotocol.ParseAction(reply.Text, task.Tools)
		if err != nil {
			_, issue := workerprotocol.ActionIssue(reply.Text, task.Tools)
			transcript += "\nObservation: " + issue + " Return only the allowed JSON schema."
			continue
		}
		if action.Action == "finish" {
			if turn < 2 || (len(task.Tools) > 0 && observations == 0) {
				transcript += "\nObservation: premature finish. Complete the required read/review before finishing."
				continue
			}
			return writeLine(output, workerprotocol.Message{Result: action.Answer})
		}
		if len(promptTask.Tools) == 0 {
			transcript += "\nObservation: no tool is available. Review the objective then finish."
			continue
		}
		key := action.Tool + "\x00" + action.Resource + "\x00" + action.Input
		if seen[key] {
			transcript += "\nObservation: input already read successfully; choose new evidence or finish."
			continue
		}
		if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: action.Tool, ResourceScope: action.Resource, Input: action.Input}}); err != nil {
			return err
		}
		var observation workerprotocol.Reply
		if err := readLine(scanner, &observation); err != nil {
			return err
		}
		if observation.Allowed && observation.Error == "" && observation.Text != "" {
			observations++
			seen[key] = true
			readInputs = append(readInputs, action.Input)
		}
		data, _ := json.Marshal(struct {
			Tool        string               `json:"tool"`
			Resource    string               `json:"resource"`
			Input       string               `json:"input"`
			Observation workerprotocol.Reply `json:"observation"`
		}{action.Tool, action.Resource, action.Input, observation})
		transcript += "\n" + string(data)
	}
	return workerprotocol.ErrTurnLimit
}

func readLine(scanner *bufio.Scanner, value any) error {
	if !scanner.Scan() {
		return errors.New("missing or oversized input")
	}
	decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid input")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing input")
	}
	return nil
}

func writeLine(output io.Writer, value any) error {
	line, err := json.Marshal(value)
	if err != nil || len(line)+1 >= lineLimit {
		return errors.New("oversized output")
	}
	_, err = output.Write(append(line, '\n'))
	return err
}
