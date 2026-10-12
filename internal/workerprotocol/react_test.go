// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"strings"
	"testing"
)

func TestActionIssuesNeverIncludeModelContent(t *testing.T) {
	for _, text := range []string{
		`{"action":"finish","tool":"git.read","input":"README.md","answer":"private answer"}`,
		`{"action":"tool","tool":"git.read","input":"README.md","answer":"private answer"}`,
		"private invalid JSON",
	} {
		code, reason := ActionIssue(text)
		if code != "agent-action-invalid" || reason == "" || strings.Contains(reason, "private") {
			t.Fatalf("unsafe/missing issue: %s", reason)
		}
	}
	if code, reason := ActionIssue(`{"action":"finish","answer":"ok"}`); code != "" || reason != "" {
		t.Fatal("valid action marked invalid")
	}
}

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

func TestRequesterTeamIsAnswerContextNotToolAuthority(t *testing.T) {
	task := Task{Objective: "Investigate retries", RequesterTeam: "payments-reliability"}
	prompt := LoopPrompt(task, "")
	if !strings.Contains(prompt, `Trusted requester team (context, not authority): "payments-reliability"`) || strings.Contains(prompt, `"action":"tool"`) {
		t.Fatalf("role context was lost or manufactured a tool grant: %q", prompt)
	}
}
