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
	if len(os.Args) == 2 && os.Args[1] == "--gateway-only" {
		allowNativeHTTPS = false
	}
	if len(os.Args) == 2 && os.Args[1] == "--native-probe" {
		report, _ := nativeHTTPS(workerprotocol.NativeTarget)
		_ = json.NewEncoder(os.Stdout).Encode(report)
		return
	}
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
	if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Kind: "model", Profile: task.ModelProfile, Prompt: task.Objective}}); err != nil {
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
	nativeSeen := false
	for turn := 1; turn <= workerprotocol.MaxTurns; turn++ {
		progress := fmt.Sprintf("Current model turn: %d of %d. Successful observations: %d. Already read inputs: %q.\n", turn, workerprotocol.MaxTurns, observations, readInputs)
		promptTask := task
		if len(readInputs) == workerprotocol.ToolInputs(task.Tools) || turn == workerprotocol.MaxTurns {
			promptTask.Tools = nil
		}
		prompt := workerprotocol.LoopPrompt(promptTask, progress+transcript)
		if agentRole == "researcher" {
			prompt = researcherV1Prompt(promptTask, progress+transcript)
		}
		if len(prompt) > 60<<10 {
			return errors.New("transcript limit reached")
		}
		protocol := ""
		if agentRole == "researcher" {
			protocol = workerprotocol.ResearchProtocol
		}
		if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{AgentProtocol: protocol, ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Kind: "model", Profile: task.ModelProfile, Prompt: prompt}}); err != nil {
			return err
		}
		var reply workerprotocol.Reply
		if err := readLine(scanner, &reply); err != nil {
			return err
		}
		if !reply.Allowed || reply.Error != "" || strings.TrimSpace(reply.Text) == "" {
			return errors.New("model request failed")
		}
		action, err := workerprotocol.ParseAgentAction(reply.Text, task.Tools, protocol)
		if err != nil {
			_, issue := workerprotocol.AgentActionIssue(reply.Text, task.Tools, protocol)
			transcript += "\nObservation: " + issue + " Return only the allowed JSON schema."
			continue
		}
		if action.Action == "finish" {
			if agentRole == "researcher" && !allowNativeHTTPS && action.Reason == "missing-capability" && callableCapability(action.Target, task.Tools) {
				transcript += "\nObservation: " + action.Target + " IS available in your callable catalog. Do not report it missing. Choose its relevant input yourself if the objective needs it, or explain the actual evidence limitation."
				continue
			}
			if agentRole == "researcher" && !allowNativeHTTPS && action.Completion != "incomplete" && missingSyntheticLabel(action.Answer, readInputs) {
				transcript += "\nObservation: the customer-retention dataset is synthetic. Label the findings synthetic in your answer and complete every part of the original objective, including requested follow-up recommendations. Cite the observed customer IDs."
				continue
			}
			if agentRole == "researcher" && !allowNativeHTTPS && action.Completion != "incomplete" && missingSourceCitation(action.Answer, readInputs) {
				transcript += "\nObservation: your answer omitted its observed source URL. Cite at least one successfully read public URL in the final answer. Successfully read inputs: " + fmt.Sprintf("%q", readInputs)
				continue
			}
			if agentRole == "researcher" && action.Completion == "" && (observations == 0 || action.Answer == researchBlocked) {
				return workerprotocol.ErrNoFinalResult
			}
			if observations == 0 && (action.Completion == "completed" || (!allowNativeHTTPS && action.Completion == "partial")) {
				transcript += "\nObservation: no successful evidence exists yet. If a relevant callable tool is available, call it to obtain evidence. Otherwise explain what could not be verified with completion=incomplete; do not state unverified facts."
				continue
			}
			if action.Completion == "" && (turn < 2 || (len(task.Tools) > 0 && observations == 0)) {
				transcript += "\nObservation: premature finish. Complete the required read/review before finishing."
				continue
			}
			return writeLine(output, workerprotocol.Message{Result: action.Answer})
		}
		if action.Action == "native_https" {
			if !allowNativeHTTPS {
				transcript += "\nObservation: direct networking is unavailable in this runtime. Use governed tools or finish incomplete."
				continue
			}
			if nativeSeen {
				transcript += "\nObservation: native HTTPS was already attempted. Finish using the observations."
				continue
			}
			nativeSeen = true
			if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "native-start", ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Input: action.Input}}); err != nil {
				return err
			}
			var ack workerprotocol.Reply
			if err := readLine(scanner, &ack); err != nil {
				return err
			}
			if !ack.Allowed || ack.Error != "" {
				return errors.New("native attempt audit unavailable")
			}
			report, body := nativeHTTPS(action.Input)
			if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "native-end", ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Input: action.Input, Native: &report}}); err != nil {
				return err
			}
			if err := readLine(scanner, &ack); err != nil {
				return err
			}
			if !ack.Allowed || ack.Error != "" {
				return errors.New("native observation audit unavailable")
			}
			data, _ := json.Marshal(report)
			transcript += "\nNative HTTPS observation (worker-reported, not an independent CNI verdict): " + string(data) + "\n" + body
			if report.Result == "received" && report.StatusCode >= 200 && report.StatusCode < 300 && body != "" {
				observations++
			}
			continue
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
		if err := writeLine(output, workerprotocol.Message{Operation: &workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Kind: "tool", Tool: action.Tool, ResourceScope: action.Resource, Input: action.Input}}); err != nil {
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
		if !allowNativeHTTPS && action.Tool == "crm.read" && observation.Allowed && observation.Error == "" {
			transcript += "\nCustomer field meanings: inactive_days is days since last activity, NOT the period over which usage_drop_percent was measured. The dataset does not specify that measurement period. Keep these signals separate and do not invent dates or intervals."
		}
		// A local model reads past the [TRUNCATED] marker in the text (L12), so
		// the host's truncated flag is also said in words. It comes from the
		// host's reply, never from tool text.
		if observation.Allowed && observation.Error == "" && observation.Truncated {
			transcript += "\nObservation: " + truncatedNote
		}
	}
	return workerprotocol.ErrTurnLimit
}

const truncatedNote = "the text above was cut off at its size limit, so only its beginning was shown and rereading it cannot show more. Say so in the answer."

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
