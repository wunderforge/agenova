// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
)

// ReferencePrincipalPreset selects a deterministic identity for local demos.
// It is operator/test setup, not a production authentication mechanism.
type ReferencePrincipalPreset string

const (
	ReferencePrincipalTeamA ReferencePrincipalPreset = "team-a"
	ReferencePrincipalTeamB ReferencePrincipalPreset = "team-b"
)

// ReferencePrincipalSource is the explicit local authentication boundary. It
// never receives ClaimRequest data, so request contents cannot replace the
// trusted principal selected by composition.
type ReferencePrincipalSource struct {
	principal v1alpha1.Principal
}

// NewReferencePrincipalSource constructs one fail-closed local identity
// source. Missing and unknown presets never fall back to a privileged user.
func NewReferencePrincipalSource(preset ReferencePrincipalPreset) (ReferencePrincipalSource, error) {
	var principal v1alpha1.Principal
	switch preset {
	case ReferencePrincipalTeamA:
		principal = v1alpha1.Principal{
			Subject:               "user:team-a-engineer",
			Team:                  "team-a",
			AuthenticationContext: "reference:local",
		}
	case ReferencePrincipalTeamB:
		principal = v1alpha1.Principal{
			Subject:               "user:team-b-engineer",
			Team:                  "team-b",
			AuthenticationContext: "reference:local",
		}
	default:
		return ReferencePrincipalSource{}, fmt.Errorf("unknown local principal preset %q", preset)
	}
	return ReferencePrincipalSource{principal: principal}, nil
}

// Principal returns a copy so callers cannot mutate the configured identity.
func (s ReferencePrincipalSource) Principal() v1alpha1.Principal {
	return s.principal
}
