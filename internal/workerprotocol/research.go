// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"errors"
	"net"
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
	return n.IP == "" || PublicNativeIP(net.ParseIP(n.IP))
}

// PublicNativeIP bounds the reported fixed-host diagnostic, not Pod authority.
func PublicNativeIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
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
	var v actionSchema
	if err = json.Unmarshal(schema, &v); err != nil {
		return nil, err
	}
	actions := []string{"tool", "native_https", "finish"}
	if finishing {
		actions = []string{"finish"}
	}
	v.Properties.Action = stringSchema{Type: "string", Enum: actions}
	// Native input is checked against NativeTarget after decoding, not as a grant.
	v.Properties.Input = stringSchema{Type: "string"}
	// Keep action/tool/input before answer and completion, like ActionSchema.
	// Alphabetical map serialization asks local constrained decoders to write the
	// answer before choosing a tool, encouraging contradictory call metadata.
	b, err := json.Marshal(researchVariant{Type: "object", Properties: researchProperties{v.Properties,
		stringSchema{Type: "string", Enum: []string{"", "completed", "partial", "incomplete"}},
		stringSchema{Type: "string", Enum: []string{"", "missing-capability", "tool-denied", "tool-failed", "network-unavailable", "insufficient-evidence"}},
		stringSchema{Type: "string"}},
		Required: []string{"action", "tool", "resource", "input", "answer", "completion", "reason", "target"}})
	if err == nil && len(b) > MaxSchemaBytes {
		return nil, ErrToolCatalog
	}
	return b, err
}

// AgentActionIssue reports bounded shape diagnostics, never model text.
func AgentActionIssue(text string, tools []Tool, protocol string) (string, string) {
	if protocol == "" {
		return ActionIssue(text, tools)
	}
	a, err := ParseAgentAction(text, tools, protocol)
	if err == nil {
		return "", ""
	}
	if a.Action == "tool" && (a.Answer != "" || a.Completion != "" || a.Reason != "" || a.Target != "") {
		return "agent-action-invalid", "A tool call must leave answer, completion, reason and target empty. Completion metadata is only for finish actions. The agent must retry."
	}
	if a.Action == "finish" && (a.Tool != "" || a.Resource != "" || a.Input != "") {
		return "agent-action-invalid", "A finish action must leave tool, resource and input empty. The agent must retry."
	}
	if a.Action == "finish" {
		return "agent-action-invalid", "A finish action requires an answer and valid completion metadata. The agent must retry."
	}
	if a.Action == "tool" {
		return "agent-action-invalid", "A tool call must select an available tool, its resource and a permitted nonempty input. The agent must retry."
	}
	return "agent-action-invalid", "The model response did not match the required Agent action JSON. The agent must retry."
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
