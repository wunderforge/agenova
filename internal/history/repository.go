// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package history defines durable evidence storage and the trusted read boundary.
// Neither persisted snapshots nor a viewer's claimed identity confer authority.
package history

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/policy"
	"time"
)

var ErrUnavailable = errors.New("evidence storage unavailable")
var ErrNotFound = errors.New("evidence not found")
var ErrConflict = errors.New("request already exists")
var ErrCursor = errors.New("invalid history cursor")
var ErrDenied = errors.New("trusted viewer required")

type Admission struct {
	Principal v0.Principal        `json:"principal"`
	Policy    policy.PolicyBundle `json:"policy"`
	Template  *v0.AgentTemplate   `json:"template,omitempty"`
	Revision  string              `json:"revision,omitempty"`
}
type Entry struct {
	View      evidence.View `json:"view"`
	Admission Admission     `json:"admission"`
	Created   time.Time     `json:"created"`
	Updated   time.Time     `json:"updated"`
	Order     uint64        `json:"order"`
}

// Authorizer is supplied by the trusted application composition (E14). It must
// not infer operator access from request parameters, headers, or loopback reachability.
type Authorizer interface {
	CanRead(context.Context, v0.Principal, Admission) bool
}
type AuthorizerFunc func(context.Context, v0.Principal, Admission) bool

func (f AuthorizerFunc) CanRead(ctx context.Context, p v0.Principal, a Admission) bool {
	return f(ctx, p, a)
}

type Summary struct {
	Completion *evidence.Completion `json:"completion,omitempty"`
	RequestRef string               `json:"requestRef"`
	ClaimID    string               `json:"claimId,omitempty"`
	TaskType   string               `json:"taskType"`
	Status     string               `json:"status"`
	Created    time.Time            `json:"created"`
	Updated    time.Time            `json:"updated"`
	TraceID    string               `json:"traceId,omitempty"`
	Diagnosis  *evidence.Diagnosis  `json:"diagnosis,omitempty"`
}
type Page struct {
	Items      []Summary `json:"items"`
	NextCursor string    `json:"nextCursor,omitempty"`
}
type Repository interface {
	Create(evidence.View, Admission) error
	Append(facts.Fact, *v0.IssuedState) error
	Finish(string, *v0.IssuedState, *evidence.Outcome) error
	Get(string) (Entry, error)
	History(context.Context, v0.Principal, Authorizer, int, string) (Page, error)
	LastSequence() (uint64, error)
	Recover() error
	Close() error
}

func Read(ctx context.Context, repository Repository, viewer v0.Principal, auth Authorizer, ref string) (evidence.View, error) {
	if viewer.Subject == "" || auth == nil {
		return evidence.View{}, ErrDenied
	}
	entry, err := repository.Get(ref)
	if err != nil {
		return evidence.View{}, err
	}
	if !auth.CanRead(ctx, viewer, entry.Admission) {
		return evidence.View{}, ErrNotFound
	}
	return entry.View, nil
}
