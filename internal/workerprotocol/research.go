// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"errors"
	"strings"
)

const ResearchProtocol = "research-v1"
const NativeTarget = "https://jitsi.org/security/"

// Completion is agent-reported goal completion, not a claim phase or grant.
type Completion struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Target string `json:"target,omitempty"`
}

// NativeObservation is a bounded worker report; it is not a CNI verdict.
type NativeObservation struct {
	Phase        string `json:"phase"`
	Result       string `json:"result"`
	IP           string `json:"ip,omitempty"`
	Milliseconds int64  `json:"milliseconds"`
	StatusCode   int    `json:"statusCode,omitempty"`
}

func ValidNative(n *NativeObservation) bool {
	if n == nil || n.Milliseconds < 0 || n.Milliseconds > 20000 || n.StatusCode < 0 || n.StatusCode > 599 {
		return false
	}
	switch n.Phase {
	case "dns", "tcp", "tls", "http":
	default:
		return false
	}
	switch n.Result {
	case "timeout", "error", "received":
	default:
		return false
	}
	return len(n.IP) <= 64 && !strings.ContainsAny(n.IP, "\r\n")
}

func ValidCompletion(c *Completion) bool {
	if c == nil {
		return true
	}
	switch c.Status {
	case "completed", "partial", "incomplete":
	default:
		return false
	}
	switch c.Reason {
	case "", "missing-capability", "tool-denied", "tool-failed", "network-unavailable", "insufficient-evidence":
	default:
		return false
	}
	return len(c.Target) <= 128 && !strings.ContainsAny(c.Target, "\r\n")
}

// ResearchSchema is opt-in. Ordinary worker schemas stay byte-compatible.
func ResearchSchema(tools []Tool, finishing bool) ([]byte, error) {
	schema, err := ActionSchema(tools)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err = json.Unmarshal(schema, &v); err != nil {
		return nil, err
	}
	p := v["properties"].(map[string]any)
	actions := []string{"tool", "native_https", "finish"}
	if finishing {
		actions = []string{"finish"}
	}
	p["action"] = map[string]any{"type": "string", "enum": actions}
	p["completion"] = map[string]any{"type": "string", "enum": []string{"", "completed", "partial", "incomplete"}}
	p["reason"] = map[string]any{"type": "string", "enum": []string{"", "missing-capability", "tool-denied", "tool-failed", "network-unavailable", "insufficient-evidence"}}
	p["target"] = map[string]any{"type": "string"}
	// Native input is checked against NativeTarget after decoding, not as a grant.
	p["input"] = map[string]any{"type": "string"}
	v["required"] = []string{"action", "tool", "resource", "input", "answer", "completion", "reason", "target"}
	b, err := json.Marshal(v)
	if err == nil && len(b) > MaxSchemaBytes {
		return nil, ErrToolCatalog
	}
	return b, err
}

func ParseAgentAction(text string, tools []Tool, protocol string) (Action, error) {
	if protocol == "" {
		return ParseAction(text, tools)
	}
	if protocol != ResearchProtocol {
		return Action{}, errors.New("unsupported agent protocol")
	}
	// Use the same bounded JSON decoder and strict field checks as ordinary actions.
	a, err := decodeAction(text)
	if err != nil {
		return a, err
	}
	if !ValidCompletion(a.CompletionValue()) {
		return a, errors.New("invalid completion metadata")
	}
	switch a.Action {
	case "native_https":
		if a.Input == NativeTarget && a.Tool == "" && a.Resource == "" && a.Answer == "" && a.Completion == "" && a.Reason == "" && a.Target == "" {
			return a, nil
		}
	case "tool":
		if a.Answer == "" && a.Completion == "" && a.Reason == "" && a.Target == "" {
			if _, ok := FindTool(tools, a.Tool, a.Resource, a.Input); ok {
				return a, nil
			}
		}
	case "finish":
		if strings.TrimSpace(a.Answer) != "" && a.Tool == "" && a.Resource == "" && a.Input == "" && a.Completion != "" {
			return a, nil
		}
	}
	return a, errors.New("invalid research action")
}

func (a Action) CompletionValue() *Completion {
	if a.Completion == "" {
		return nil
	}
	return &Completion{Status: a.Completion, Reason: a.Reason, Target: a.Target}
}
