// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"bytes"
	"strings"
	"testing"
)

func TestResearchSchemaKeepsDecisionBeforeAnswer(t *testing.T) {
	b, err := ResearchSchema([]Tool{{Operation: "web.read", Description: "Public page", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}}, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := -1
	for _, key := range []string{"action", "tool", "resource", "input", "answer", "completion", "reason", "target"} {
		next := bytes.Index(b, []byte(`"`+key+`":`))
		if next <= previous {
			t.Fatalf("decision field order lost: %s", b)
		}
		previous = next
	}
}
func TestResearchIssueDoesNotConfuseCompletionWithToolAuthority(t *testing.T) {
	code, reason := AgentActionIssue(`{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.com/","completion":"incomplete","target":"private-marker"}`, nil, ResearchProtocol)
	if code != "agent-action-invalid" || !strings.Contains(reason, "Completion metadata") || strings.Contains(reason, "private-marker") {
		t.Fatal(code, reason)
	}
	_, reason = AgentActionIssue(`{"action":"finish","answer":"Cannot verify without a tool.","completion":"incomplete","reason":"missing-capability"}`, nil, ResearchProtocol)
	if reason != "" {
		t.Fatal(reason)
	}
}
