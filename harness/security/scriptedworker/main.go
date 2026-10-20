// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command scriptedworker is a test-only E15 worker for #198. It replaces the
// reference worker entrypoint and sends a fixed operation sequence chosen by
// the scenario named in the task objective, so each refusal path can be
// exercised on the installed reference environment. It is not a product
// deliverable and never runs outside an explicitly selected test cluster.
//
// Objective format: "e15-scenario=<name> [target=<claim-id>] [wait=<duration>]"
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
	"time"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const lineLimit = 128 * 1024

// step is one worker record: an operation, a raw line, or a pause.
type step struct {
	op   *workerprotocol.Operation
	raw  string
	wait time.Duration
}

func main() {
	if err := run(os.Stdin, os.Stdout, time.Sleep); err != nil {
		os.Stderr.WriteString("scripted worker stopped: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run(input io.Reader, output io.Writer, sleep func(time.Duration)) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), lineLimit)
	var task workerprotocol.Task
	if err := readLine(scanner, &task); err != nil {
		return err
	}
	steps, err := scenario(task)
	if err != nil {
		return err
	}
	var lastText string
	for _, s := range steps {
		switch {
		case s.wait > 0:
			sleep(s.wait)
			continue
		case s.raw != "":
			if _, err := io.WriteString(output, s.raw+"\n"); err != nil {
				return err
			}
			continue
		}
		line, err := json.Marshal(workerprotocol.Message{Operation: s.op})
		if err != nil {
			return err
		}
		if _, err := output.Write(append(line, '\n')); err != nil {
			return err
		}
		var reply workerprotocol.Reply
		// A refused operation closes the session, so no reply is the expected
		// end of a negative scenario.
		if err := readLine(scanner, &reply); err != nil {
			return err
		}
		lastText = reply.Text
	}
	if strings.Contains(task.Objective, "e15-scenario=control") {
		a, err := workerprotocol.ParseAction(lastText, task.Tools)
		if err != nil || a.Action != "finish" {
			return errors.New("control requires model-selected finish")
		}
		b, _ := json.Marshal(workerprotocol.Message{Result: a.Answer})
		_, err = output.Write(append(b, '\n'))
		return err
	}
	return nil
}

func scenario(task workerprotocol.Task) ([]step, error) {
	args := map[string]string{}
	for _, field := range strings.Fields(task.Objective) {
		if key, value, ok := strings.Cut(field, "="); ok {
			args[key] = value
		}
	}
	// Every operation carries the host-issued token for its Gateway, so each
	// scenario changes only the field it names.
	allowed := func() step {
		return step{op: &workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")}}
	}
	with := func(edit func(*workerprotocol.Operation)) step {
		s := allowed()
		edit(s.op)
		return s
	}
	tool := func(name, scope string) step {
		return step{op: &workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Kind: "tool", Tool: name, ResourceScope: scope, Input: "README.md"}}
	}
	grantedScope := ""
	if len(task.Tools) > 0 {
		grantedScope = task.Tools[0].ResourceScope
	}
	switch name := args["e15-scenario"]; name {
	case "control":
		return []step{allowed(), allowed()}, nil
	case "cross-claim":
		if args["target"] == "" || args["target"] == task.ClaimID {
			return nil, errors.New("cross-claim needs a target claim other than the bound claim")
		}
		return []step{allowed(), with(func(op *workerprotocol.Operation) { op.ClaimID = args["target"] })}, nil
	case "profile":
		return []step{allowed(), with(func(op *workerprotocol.Operation) { op.Profile = "e15-ungranted-profile" })}, nil
	case "tool":
		return []step{allowed(), tool("git.write", grantedScope)}, nil
	case "scope":
		return []step{allowed(), tool("git.read", "repo:e15/ungranted")}, nil
	case "malformed":
		return []step{allowed(), {raw: "{not-json"}}, nil
	case "unsupported":
		return []step{allowed(), with(func(op *workerprotocol.Operation) { op.Kind = "memory" })}, nil
	case "turn-limit":
		steps := make([]step, workerprotocol.MaxTurns+1)
		for i := range steps {
			steps[i] = allowed()
		}
		return steps, nil
	case "after-expiry":
		wait, err := time.ParseDuration(args["wait"])
		if err != nil || wait <= 0 {
			return nil, errors.New("after-expiry needs a positive wait")
		}
		return []step{allowed(), {wait: wait}, allowed()}, nil
	case "after-failure":
		return []step{allowed(), {raw: `{"result":"e15-unmatched-result"}`}, allowed()}, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q", name)
	}
}

func readLine(scanner *bufio.Scanner, value any) error {
	if !scanner.Scan() {
		return errors.New("session closed")
	}
	decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid input")
	}
	return nil
}
