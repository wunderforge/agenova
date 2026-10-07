// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"time"
	"unicode/utf8"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
)

// Resolution is an internal proof that #28 resolved authority for one full
// ClaimRequest. Its fields are private: a public EffectiveAuthority, even if
// equal in shape, is not sufficient input for system claim issuance.
type Resolution struct {
	requestDigest [sha256.Size]byte
	authority     *v1alpha1.EffectiveAuthority
	changes       []Change
}

// Change is producer-recorded resolution provenance, not another permission
// engine. The current resolver narrows against the admitted template ceiling.
type Change struct {
	Field      string `json:"field"`
	Requested  string `json:"requested"`
	Effective  string `json:"effective"`
	ReasonCode string `json:"reasonCode"`
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
		return nil, invalid("$", "request could not be losslessly encoded for authority binding")
	}
	changes := []Change{}
	for _, dimension := range []struct {
		field                string
		requested, effective []string
	}{
		{"tools", request.Spec.RequestedAccess.Tools, resolved.Tools},
		{"resourceScopes", request.Spec.RequestedAccess.ResourceScopes, resolved.ResourceScopes},
		{"memoryScopes", request.Spec.RequestedAccess.MemoryScopes, resolved.MemoryScopes},
	} {
		for _, value := range dimension.requested {
			if !contains(dimension.effective, value) {
				reason := "outside-template-ceiling"
				if dimension.field == "tools" && contains(template.Spec.CapabilityCeiling.Tools, value) {
					if policyTools, limited := admission.ToolCeiling(); limited && !contains(policyTools, value) {
						reason = "outside-policy-tool-ceiling"
					}
				}
				changes = append(changes, Change{Field: dimension.field, Requested: value, ReasonCode: reason})
			}
		}
	}
	if *request.Spec.Runtime.Timeout != resolved.Runtime.Timeout {
		changes = append(changes, Change{Field: "runtime.timeout", Requested: time.Duration(*request.Spec.Runtime.Timeout).String(), Effective: time.Duration(resolved.Runtime.Timeout).String(), ReasonCode: "template-timeout-cap"})
	}
	return &Resolution{requestDigest: digest, authority: resolved, changes: changes}, nil
}

func (r *Resolution) ChangesFor(request *v1alpha1.ClaimRequest) ([]Change, bool) {
	if _, ok := r.AuthorityFor(request); !ok {
		return nil, false
	}
	return append([]Change{}, r.changes...), true
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
	if !validUTF8Value(reflect.ValueOf(request)) {
		return [sha256.Size]byte{}, errors.New("request contains invalid UTF-8")
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

// encoding/json replaces invalid UTF-8 bytes with U+FFFD. Reject them before
// hashing so different in-memory requests cannot share one authority proof.
// Both callers validate ClaimRequest first; its task-input validator rejects
// cyclic containers. Do not add a second, lower nesting limit here.
func validUTF8Value(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return true
		}
		return validUTF8Value(value.Elem())
	case reflect.String:
		return utf8.ValidString(value.String())
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !validUTF8Value(value.Field(i)) {
				return false
			}
		}
	case reflect.Map:
		for iter := value.MapRange(); iter.Next(); {
			if !validUTF8Value(iter.Key()) || !validUTF8Value(iter.Value()) {
				return false
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if !validUTF8Value(value.Index(i)) {
				return false
			}
		}
	}
	return true
}
