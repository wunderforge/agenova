// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package authentication binds externally verified identity to current server-owned
// membership. Authentication does not issue claims or confer infrastructure rights.
package authentication

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

var ErrUnauthenticated = errors.New("authentication required")
var ErrForbidden = errors.New("active membership required")

// Identity is private authentication metadata, never an evidence/Work payload.
// TokenID and SessionID identify revocation records, not bearer credentials.
type Identity struct {
	Issuer, Subject, TokenID, SessionID string
	ExpiresAt                           time.Time
}

type Verifier interface {
	Verify(context.Context, string) (Identity, error)
}

// Memberships must check active membership AND token/session revocation on every
// call. Failure/unavailability denies access. No implementation may derive team,
// role, or subject from caller-authored request fields or mutable email.
// The returned principal is a public projection, not an authorization decision.
type Memberships interface {
	Resolve(context.Context, Identity) (v0.Principal, error)
}

type Boundary struct {
	verifier    Verifier
	memberships Memberships
}

func New(verifier Verifier, memberships Memberships) (*Boundary, error) {
	if verifier == nil || memberships == nil {
		return nil, errors.New("identity verifier and current membership source are required")
	}
	return &Boundary{verifier, memberships}, nil
}

func (b *Boundary) Authenticate(r *http.Request) (v0.Principal, error) {
	if b == nil || r == nil {
		return v0.Principal{}, ErrUnauthenticated
	}
	values := r.Header.Values("Authorization")
	if len(values) != 1 || len(values[0]) > 16<<10 {
		return v0.Principal{}, ErrUnauthenticated
	}
	scheme, token, ok := strings.Cut(values[0], " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return v0.Principal{}, ErrUnauthenticated
	}
	id, err := b.verifier.Verify(r.Context(), token)
	if err != nil || id.Issuer == "" || id.Subject == "" || id.TokenID == "" || id.SessionID == "" || !id.ExpiresAt.After(time.Now()) {
		return v0.Principal{}, ErrUnauthenticated
	}
	p, err := b.memberships.Resolve(r.Context(), id)
	if err != nil || strings.TrimSpace(p.Subject) == "" || strings.TrimSpace(p.Team) == "" || p.AuthenticationContext != "oidc" {
		return v0.Principal{}, ErrForbidden
	}
	return p, nil
}
