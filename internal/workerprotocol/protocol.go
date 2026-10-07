// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package workerprotocol is the opt-in demo worker's bounded stdio protocol.
// It is a composition-edge protocol, not part of RuntimeBackend or authority.
package workerprotocol

import "context"

type Task struct {
	ClaimID       string   `json:"claimId"`
	Objective     string   `json:"objective"`
	ModelProfile  string   `json:"modelProfile"`
	Mode          string   `json:"mode,omitempty"`
	ResourceScope string   `json:"resourceScope,omitempty"`
	RequesterTeam string   `json:"requesterTeam,omitempty"`
	AllowedTools  []string `json:"allowedTools,omitempty"`
	// CandidateTools advertises operations the example agent may request, not
	// authority. The Tool Gateway alone decides whether each call is granted.
	CandidateTools []string          `json:"candidateTools,omitempty"`
	ToolScopes     map[string]string `json:"toolScopes,omitempty"`
	// CompletionTool is a task deliverable, never an authority grant. The
	// example agent must observe the requested attempt or success before finish.
	CompletionTool string `json:"completionTool,omitempty"`
	CompletionMode string `json:"completionMode,omitempty"`
}

type Operation struct {
	ClaimID       string `json:"claimId"`
	Kind          string `json:"kind"`
	Profile       string `json:"profile,omitempty"`
	Tool          string `json:"tool,omitempty"`
	ResourceScope string `json:"resourceScope,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
	Input         string `json:"input,omitempty"`
}

type Reply struct {
	Allowed bool   `json:"allowed"`
	Text    string `json:"text,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Message travels from worker to host: exactly one operation or final result.
type Message struct {
	Operation *Operation `json:"operation,omitempty"`
	Result    string     `json:"result,omitempty"`
}

type Handler func(context.Context, Operation) (Reply, error)
