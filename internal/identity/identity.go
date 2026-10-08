// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package identity verifies human identity at trusted transport boundaries.
// Domain requests never carry or select a VerifiedPrincipal.
package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

// Provider verifies externally issued credentials. Implementations must return
// only sanitized Error values; bearer tokens and raw provider responses are
// never caller-visible diagnostics.
type Provider interface {
	Verify(context.Context, string) (VerifiedPrincipal, error)
}

// VerifiedPrincipal is immutable trusted identity established outside a
// ClaimRequest. Attributes are bounded provider-neutral facts for future use;
// they are not granted authority.
type VerifiedPrincipal struct {
	Issuer                string
	Subject               string
	Team                  string
	AuthenticationContext string
	Attributes            []Attribute
}

type Attribute struct {
	Name   string
	Values []string
}

func (p VerifiedPrincipal) Principal() v0.Principal {
	return v0.Principal{Subject: p.Subject, Team: p.Team, AuthenticationContext: p.AuthenticationContext}
}

// OwnerKey is an opaque internal ownership key. Evidence keeps the existing
// public Principal projection and never exposes this value.
func (p VerifiedPrincipal) OwnerKey() string {
	sum := sha256.Sum256([]byte(p.Issuer + "\x00" + p.Subject))
	return hex.EncodeToString(sum[:])
}

func (p VerifiedPrincipal) Validate() error {
	if strings.TrimSpace(p.Issuer) == "" || strings.TrimSpace(p.Subject) == "" || strings.TrimSpace(p.Team) == "" || strings.TrimSpace(p.AuthenticationContext) == "" {
		return errors.New("verified principal is incomplete")
	}
	return nil
}

type ErrorCategory string

const (
	CategoryMissingCredential ErrorCategory = "missing_credential"
	CategoryMalformedToken    ErrorCategory = "malformed_token"
	CategoryInvalidSignature  ErrorCategory = "invalid_signature"
	CategoryInvalidIssuer     ErrorCategory = "invalid_issuer"
	CategoryInvalidAudience   ErrorCategory = "invalid_audience"
	CategoryExpiredToken      ErrorCategory = "expired_token"
	CategoryInactiveToken     ErrorCategory = "inactive_token"
	CategoryInvalidIdentity   ErrorCategory = "invalid_identity"
	CategoryKeyUnavailable    ErrorCategory = "key_unavailable"
)

// Error intentionally carries no wrapped provider error, token, raw claim, or
// key material. Operators diagnose details through bounded internal metrics.
type Error struct{ Category ErrorCategory }

func (e *Error) Error() string { return "identity verification failed: " + string(e.Category) }

func IsCategory(err error, category ErrorCategory) bool {
	var identityError *Error
	return errors.As(err, &identityError) && identityError.Category == category
}

func reject(category ErrorCategory) error { return &Error{Category: category} }
