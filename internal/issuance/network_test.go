// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package issuance

import (
	"reflect"
	"testing"

	"github.com/wunderforge/agenova/internal/policy"
)

func TestIssueNetworkAuthorityIsBoundAndDefensivelyCopied(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
	template.Spec.CapabilityCeiling.NetworkProfiles = []string{"public-web"}
	bundle.Rules[0].Constraints = &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	state, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.EffectiveAuthority.NetworkProfiles, []string{"public-web"}) {
		t.Fatal("network authority lost on issuance")
	}
	state.EffectiveAuthority.NetworkProfiles[0] = "mutated"
	second, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil || !reflect.DeepEqual(second.EffectiveAuthority.NetworkProfiles, []string{"public-web"}) {
		t.Fatalf("issued network aliases resolution: %+v / %v", second, err)
	}
	offline := cloneRequest(t, request)
	offline.Spec.RequestedAccess.NetworkProfiles = nil
	state, err = Issue(offline, fixture.Principal, decision, resolved, admission)
	assertRejected(t, state, err, "resolution")
	offlineResolution := resolve(t, offline, template, admission)
	state, err = Issue(offline, fixture.Principal, decision, offlineResolution, admission)
	if err != nil || len(state.EffectiveAuthority.NetworkProfiles) != 0 || state.Claim.ID == second.Claim.ID {
		t.Fatalf("offline identity/grant wrong: %+v / %v", state, err)
	}
}
