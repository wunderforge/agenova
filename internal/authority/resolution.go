// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"crypto/sha256"
	"encoding/json"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
)

// Resolution is an internal proof that #28 resolved authority for one full
// ClaimRequest. Its fields are private: a public EffectiveAuthority, even if
// equal in shape, is not sufficient input for system claim issuance.
type Resolution struct {
	requestDigest [sha256.Size]byte
	authority     *v1alpha1.EffectiveAuthority
}

// ResolveForIssuance performs the usual authority intersection and binds its
// output to the canonical full request, including task and requested access.
func ResolveForIssuance(request *v1alpha1.ClaimRequest, template *v1alpha1.AgentTemplate, admission authorization.Admission) (*Resolution, *v1alpha1.ValidationError) {
	resolved, err := Resolve(request, template, admission)
	if err != nil {
		return nil, err
	}
	digest, digestErr := fullRequestDigest(request)
	if digestErr != nil {
		return nil, invalid("spec.task.input", "validated request could not be encoded for authority binding")
	}
	return &Resolution{requestDigest: digest, authority: resolved}, nil
}

// AuthorityFor returns an independent authority snapshot only when the
// supplied full request is exactly the one resolved. A zero or mismatched
// resolution cannot be used for issuance.
func (r *Resolution) AuthorityFor(request *v1alpha1.ClaimRequest) (*v1alpha1.EffectiveAuthority, bool) {
	if r == nil || r.authority == nil || v1alpha1.ValidateClaimRequest(request) != nil {
		return nil, false
	}
	digest, err := fullRequestDigest(request)
	if err != nil || digest != r.requestDigest {
		return nil, false
	}
	copy := *r.authority
	copy.Tools = append([]string(nil), r.authority.Tools...)
	copy.ResourceScopes = append([]string(nil), r.authority.ResourceScopes...)
	copy.MemoryScopes = append([]string(nil), r.authority.MemoryScopes...)
	return &copy, true
}

func fullRequestDigest(request *v1alpha1.ClaimRequest) ([sha256.Size]byte, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
