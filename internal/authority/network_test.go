// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"reflect"
	"testing"

	"github.com/wunderforge/agenova/internal/policy"
)

func TestNetworkAuthorityRequiresExplicitThreeWayConsent(t *testing.T) {
	for _, test := range []struct {
		name             string
		request, ceiling []string
		caps             *policy.AuthorityConstraints
		allowed          bool
	}{
		{"legacy offline", nil, nil, nil, true},
		{"offline with reusable policy and ceiling", nil, []string{"public-web"}, &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}, true},
		{"explicitly empty request", []string{}, []string{"public-web"}, &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}, true},
		{"missing template ceiling", []string{"public-web"}, nil, &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}, false},
		{"missing policy constraints", []string{"public-web"}, []string{"public-web"}, nil, false},
		{"missing network policy cap", []string{"public-web"}, []string{"public-web"}, &policy.AuthorityConstraints{}, false},
		{"empty network policy cap", []string{"public-web"}, []string{"public-web"}, &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{}}}, false},
		{"explicit request ceiling and policy", []string{"public-web"}, []string{"public-web"}, &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, template, _ := fixtures(t)
			request.Spec.RequestedAccess.NetworkProfiles = test.request
			template.Spec.CapabilityCeiling.NetworkProfiles = test.ceiling
			grant, err := Resolve(request, template, admitWithConstraints(t, request, test.caps))
			if !test.allowed {
				assertFailure(t, grant, err, "spec.requestedAccess.networkProfiles")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(grant.NetworkProfiles) != len(test.request) || len(test.request) > 0 && !reflect.DeepEqual(grant.NetworkProfiles, test.request) {
				t.Fatalf("grant differs from request: %q vs %q", grant.NetworkProfiles, test.request)
			}
		})
	}
}

func TestNetworkAuthoritySnapshotBindingAndIsolation(t *testing.T) {
	request, template, _ := fixtures(t)
	request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
	template.Spec.CapabilityCeiling.NetworkProfiles = []string{"public-web"}
	caps := &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{"public-web"}}}
	resolution, err := ResolveForIssuance(request, template, admitWithConstraints(t, request, caps))
	if err != nil {
		t.Fatal(err)
	}
	grant, ok := resolution.AuthorityFor(request)
	if !ok {
		t.Fatal("valid network request lost binding")
	}
	grant.NetworkProfiles[0] = "mutated"
	template.Spec.CapabilityCeiling.NetworkProfiles[0] = "mutated"
	caps.NetworkProfiles.Values[0] = "mutated"
	independent, ok := resolution.AuthorityFor(request)
	if !ok || !reflect.DeepEqual(independent.NetworkProfiles, []string{"public-web"}) {
		t.Fatalf("network snapshot aliases inputs/results: %+v", independent)
	}
	request.Spec.RequestedAccess.NetworkProfiles = nil
	if grant, ok := resolution.AuthorityFor(request); ok || grant != nil {
		t.Fatal("network grant reused for a different full request")
	}
	request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
	if changes, ok := resolution.ChangesFor(request); !ok || len(changes) != 0 {
		t.Fatalf("unchanged explicit grant has spurious narrowing: %+v / %v", changes, ok)
	}
}
