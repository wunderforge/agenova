// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
