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
	transcript := ""
	observations := 0
	readFiles := []string{}
	seen := map[string]bool{}
	proposalRecorded := false
	deniedAttempted := false
	completionAttempted := false
	completionSucceeded := false
	for turn := 1; turn <= workerprotocol.MaxTurns; turn++ {
		progress := fmt.Sprintf("Current model turn: %d of %d. Successful observations: %d. Already read files: %q. Action completed: %t. Denied attempt observed: %t.\n", turn, workerprotocol.MaxTurns, observations, readFiles, proposalRecorded, deniedAttempted)
		promptTask := task
		if proposalRecorded || deniedAttempted || turn == workerprotocol.MaxTurns {
			promptTask.ResourceScope = ""
		} else if len(task.CandidateTools) > 0 && hasTool(task.CandidateTools, "git.read") && (len(readFiles) == 0 || (task.CompletionTool == "github.pr.create" && hasTool(task.AllowedTools, "github.pr.create") && !containsFile(readFiles, "src/retry.go"))) {
			promptTask.CandidateTools = []string{"git.read"}
			promptTask.AllowedTools = []string{"git.read"}
		} else if len(readFiles) == 3 && len(task.CandidateTools) == 0 {
			promptTask.AllowedTools = nil
			for _, tool := range task.AllowedTools {
				if tool != "git.read" {
					promptTask.AllowedTools = append(promptTask.AllowedTools, tool)
				}
			}
			promptTask.CandidateTools = nil
			for _, tool := range task.CandidateTools {
				if tool != "git.read" {
					promptTask.CandidateTools = append(promptTask.CandidateTools, tool)
				}
			}
			if len(promptTask.AllowedTools) == 0 && len(promptTask.CandidateTools) == 0 {
				promptTask.ResourceScope = ""
			}
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
		action, err := workerprotocol.ParseAction(reply.Text)
		if err != nil {
			_, issue := workerprotocol.ActionIssue(reply.Text)
			transcript += "\nObservation: " + issue + " Return only the allowed JSON schema."
			continue
		}
		if action.Action == "finish" {
			if turn < 2 || (task.ResourceScope != "" && observations == 0 && !deniedAttempted) {
				transcript += "\nObservation: premature finish. Complete the required read/review before finishing."
				continue
			}
			if task.CompletionTool != "" && ((task.CompletionMode == "attempt" && !completionAttempted) || (task.CompletionMode == "success" && !completionSucceeded)) {
				transcript += "\nObservation: the task deliverable has not been observed. A plan is not completion. Attempt the governed operation and use the actual result; if it fails, revise and retry."
				continue
			}
			return writeLine(output, workerprotocol.Message{Result: action.Answer})
		}
		if len(task.CandidateTools) > 0 && action.Tool != "git.read" && len(readFiles) == 0 {
			transcript += "\nObservation: read at least one relevant incident artifact before an external operation."
			continue
		}
		if action.Tool == "github.pr.create" && hasTool(task.AllowedTools, "github.pr.create") && !containsFile(readFiles, "src/retry.go") {
			transcript += "\nObservation: read src/retry.go before authoring a PR; read src/retry_test.go too if you need its assertions."
			continue
		}
		actionScope := task.ToolScopes[action.Tool]
		if actionScope == "" {
			actionScope = task.ResourceScope
		}
		if actionScope == "" {
			transcript += "\nObservation: no tool is available. Review the objective then finish."
			continue
		}
		key := action.Tool + ":" + action.Input
		if seen[key] {
			if action.Tool == "git.read" {
				transcript += "\nObservation: file already read successfully; choose new evidence or finish."
			} else {
				transcript += "\nObservation: this exact tool request was already attempted; choose new evidence or finish."
			}
			continue
		}
		if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: action.Tool, ResourceScope: actionScope, Input: action.Input}}); err != nil {
			return err
		}
		var observation workerprotocol.Reply
		if err := readLine(scanner, &observation); err != nil {
			return err
		}
		seen[key] = true
		if action.Tool == task.CompletionTool {
			completionAttempted = true
			if observation.Allowed && observation.Error == "" && observation.Text != "" {
				completionSucceeded = true
			}
		}
		if observation.Allowed && observation.Error == "" && observation.Text != "" {
			observations++
			if action.Tool == "git.read" {
				readFiles = append(readFiles, action.Input)
			} else {
				proposalRecorded = true
			}
		} else if !observation.Allowed && observation.Error == "tool access denied" {
			deniedAttempted = true
		}
		data, _ := json.Marshal(struct {
			Tool        string               `json:"tool"`
			File        string               `json:"file"`
			Observation workerprotocol.Reply `json:"observation"`
		}{action.Tool, action.Input, observation})
		transcript += "\n" + string(data)
	}
	return workerprotocol.ErrTurnLimit
}

func hasTool(tools []string, target string) bool {
	for _, name := range tools {
		if name == target {
			return true
		}
	}
	return false
}

func containsFile(files []string, target string) bool {
	for _, name := range files {
		if name == target {
			return true
		}
	}
	return false
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
