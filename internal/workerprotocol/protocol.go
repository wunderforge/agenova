// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package workerprotocol is the opt-in demo worker's bounded stdio protocol.
// It is a composition-edge protocol, not part of RuntimeBackend or authority.
package workerprotocol

import "context"

type Task struct {
	ClaimID       string `json:"claimId"`
	Objective     string `json:"objective"`
	ModelProfile  string `json:"modelProfile"`
	Mode          string `json:"mode,omitempty"`
	ResourceScope string `json:"resourceScope,omitempty"`
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
	Untrusted bool   `json:"untrusted,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Allowed   bool   `json:"allowed"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Message travels from worker to host: exactly one operation or final result.
type Message struct {
	Operation *Operation `json:"operation,omitempty"`
	Result    string     `json:"result,omitempty"`
}

type Handler func(context.Context, Operation) (Reply, error)
