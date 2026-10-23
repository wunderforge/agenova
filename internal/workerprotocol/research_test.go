// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"testing"
)

func TestResearchSchemaAndActionsAreOptIn(t *testing.T) {
	cases := []struct {
		action string
		valid  bool
	}{
		{`{"action":"finish","answer":"No available web tool.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`, true},
		{`{"action":"native_https","input":"https://jitsi.org/security/"}`, true},
		{`{"action":"native_https","input":"https://169.254.169.254/"}`, false},
		{`{"action":"native_https","input":"https://jitsi.org/security/","completion":"completed"}`, false},
		{`{"action":"finish","answer":"Blocked.","completion":"verified-firewall-block"}`, false},
	}
	for _, c := range cases {
		_, err := ParseAgentAction(c.action, nil, ResearchProtocol)
		if (err == nil) != c.valid {
			t.Fatalf("action %s err %v", c.action, err)
		}
		if c.valid {
			if _, err := ParseAction(c.action, nil); err == nil {
				t.Fatal("legacy accepted opt-in action")
			}
		}
	}
	for _, finishing := range []bool{false, true} {
		b, err := ResearchSchema(nil, finishing)
		if err != nil || !json.Valid(b) {
			t.Fatal(err)
		}
		var v map[string]any
		json.Unmarshal(b, &v)
		if len(v["required"].([]any)) != 8 {
			t.Fatal("schema metadata missing")
		}
	}
}

func TestNativeReportValidation(t *testing.T) {
	if !ValidNative(&NativeObservation{Phase: "tcp", Result: "timeout", Milliseconds: 5000, IP: "1.2.3.4"}) {
		t.Fatal("bounded report rejected")
	}
	for _, n := range []*NativeObservation{nil, {Phase: "firewall", Result: "blocked"}, {Phase: "tcp", Result: "timeout", Milliseconds: 20001}, {Phase: "tcp", Result: "timeout", IP: "x\nsecret"}} {
		if ValidNative(n) {
			t.Fatal("invalid report accepted")
		}
	}
}
