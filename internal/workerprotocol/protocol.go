// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package workerprotocol is the opt-in demo worker's bounded stdio protocol.
// It is a composition-edge protocol, not part of RuntimeBackend or authority.
package workerprotocol

import "context"

// MaxTools and MaxInputBytes bound the worker catalog; the host enforces the
// same limits when it builds a Task.
const MaxTools = 32
const MaxInputBytes = 4096

// Tool is one logical operation/resource pair a Work may call, already
// intersected with its effective authority. It carries no endpoint,
// transport, provider-native name or credential.
type Tool struct {
	Operation     string   `json:"operation"`
	Description   string   `json:"description"`
	ResourceScope string   `json:"resourceScope"`
	Parameter     string   `json:"parameter"`
	AllowedValues []string `json:"allowedValues"`
	// Synthetic labels the explicitly selected legacy mock fixture.
	Synthetic bool `json:"synthetic,omitempty"`
}

type Task struct {
	ClaimID      string `json:"claimId"`
	Objective    string `json:"objective"`
	ModelProfile string `json:"modelProfile"`
	Mode         string `json:"mode,omitempty"`
	Tools        []Tool `json:"tools,omitempty"`
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
