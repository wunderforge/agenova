// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package memory governs reusable data access for one trusted worker claim.
// Content returned here is untrusted data, not policy or agent instructions.
package memory

import (
	"context"
	"errors"
	"time"
)

const (
	MaxBodyBytes  = 8192
	MaxQueryBytes = 512
	DefaultLimit  = 3
	MaxLimit      = 10
	MaxReplyBytes = 32768
	CallTimeout   = 5 * time.Second
)

type Status string

const (
	Written        Status = "Written"
	Found          Status = "Found"
	Empty          Status = "Empty"
	Denied         Status = "Denied"
	Unsupported    Status = "Unsupported"
	Unavailable    Status = "Unavailable"
	Timeout        Status = "Timeout"
	Cancelled      Status = "Cancelled"
	Failed         Status = "Failed"
	WriteUncertain Status = "WriteUncertain"
)

var (
	ErrUnsupported    = errors.New("memory backend unsupported")
	ErrUnavailable    = errors.New("memory backend unavailable")
	ErrWriteUncertain = errors.New("memory write acknowledgement uncertain")
	ErrEvidence       = errors.New("memory evidence unavailable")
	ErrBinding        = errors.New("memory trusted binding unavailable")
)

type Namespace struct {
	Team, Project, Scope string
}

type WriteInput struct {
	// InvocationID identifies the original trusted write receipt. Authorized
	// retry attempts reuse it while receiving fresh journal invocation IDs.
	InvocationID, ClaimID, Body string
}

type SearchInput struct {
	InvocationID, Query string
	Limit               int
}

// Record is private adapter data; ownership is checked again before delivery.
type Record struct {
	Namespace                    Namespace
	Reference, Body, SourceClaim string
	CreatedAt                    time.Time
}

// Backend implementations must honor cancellation, perform literal substring
// search, and order results by descending creation time then reference.
type Backend interface {
	Write(context.Context, Namespace, WriteInput) (Record, error)
	Search(context.Context, Namespace, SearchInput) ([]Record, error)
}

// ClaimID is only a target-consistency check against an established session.
type Request struct {
	ClaimID, Operation, Scope, Body, Query string
	Limit                                  int
}

type Entry struct {
	Reference string `json:"reference"`
	Body      string `json:"body"`
}

// Result is a private worker reply, not a public evidence representation.
type Result struct {
	InvocationID string  `json:"invocationId"`
	Status       Status  `json:"status"`
	ReasonCode   string  `json:"reasonCode"`
	Reference    string  `json:"reference,omitempty"`
	Entries      []Entry `json:"entries,omitempty"`
	Truncated    bool    `json:"truncated"`
	// Retry is host-only continuation state, never a worker-supplied ID or wire value.
	Retry *WriteRetry `json:"-"`
}
