// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"strings"
	"testing"
)

func TestActionBoundary(t *testing.T) {
	for _, text := range []string{`{}`, `{"action":"tool","tool":"shell.exec","input":"ls"}`, `{"action":"finish","answer":""}`, `{"action":"finish","answer":"ok","thought":"private"}`, `{"action":"finish","answer":"ok"} {}`, `{"action":"tool","tool":"git.read","input":"README.md","answer":"fake"}`} {
		if _, err := ParseAction(text); err == nil {
			t.Fatalf("invalid action accepted: %s", text)
		}
	}
	for _, text := range []string{`{"action":"tool","tool":"git.read","input":"README.md"}`, `{"action":"finish","answer":"Use a shared deadline."}`} {
		if _, err := ParseAction(text); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoopPromptKeepsObjectiveAndRemovesToolActionWhenUnavailable(t *testing.T) {
	task := Task{Objective: "Inspect synthetic retries.", ResourceScope: "repo:acme/payments"}
	withTools := LoopPrompt(task, "Successful observations: 0")
	if !strings.HasPrefix(withTools, PromptPrefix(task)) || !strings.Contains(withTools, `"action":"tool"`) {
		t.Fatal("available action or rooted objective missing")
	}
	task.ResourceScope = ""
	finishOnly := LoopPrompt(task, "Successful observations: 3")
	if !strings.HasPrefix(finishOnly, PromptPrefix(task)) || strings.Contains(finishOnly, `"action":"tool"`) || !strings.Contains(finishOnly, `"action":"finish"`) {
		t.Fatal("finish-only prompt still advertises a tool action")
	}
}
