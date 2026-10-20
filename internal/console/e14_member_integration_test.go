// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/policy"
)

type e14Verifier struct{}

func (e14Verifier) Verify(_ context.Context, token string) (authentication.Identity, error) {
	if token != "alice-token" && token != "bob-token" {
		return authentication.Identity{}, errors.New("invalid token")
	}
	return authentication.Identity{
		Issuer: "https://company.example", Subject: strings.TrimSuffix(token, "-token"),
		TokenID: token, SessionID: token, ExpiresAt: time.Now().Add(time.Minute),
	}, nil
}

type e14Memberships struct{}

func (e14Memberships) Resolve(_ context.Context, id authentication.Identity) (v0.Principal, error) {
	if id.Subject != "alice" && id.Subject != "bob" {
		return v0.Principal{}, authentication.ErrForbidden
	}
	return v0.Principal{Subject: id.Subject, Team: "team-a", AuthenticationContext: "oidc"}, nil
}

// E14 reuses the production member boundary. It must not select the reference
// principal or accept a caller-authored team when two members use one service.
func TestE14TwoMembersShareOnePolicyService(t *testing.T) {
	loader := &policy.Loader{}
	aliceTimeout, bobTimeout := 10*time.Minute, 20*time.Minute
	if err := loader.Load(policy.PolicyBundle{ID: "e14-two-member-caps", Version: "1", Rules: []policy.Rule{
		{Subject: "alice", Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer", Constraints: &policy.AuthorityConstraints{Tools: &policy.StringSetConstraint{Values: []string{"git.read"}}, MaxTimeout: &aliceTimeout}},
		{Subject: "bob", Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer", Constraints: &policy.AuthorityConstraints{Tools: &policy.StringSetConstraint{Values: []string{"git.read", "shell.exec"}}, MaxTimeout: &bobTimeout}},
	}}); err != nil {
		t.Fatal(err)
	}
	backend, provider := &verticalBackend{}, &httpProvider{}
	service, err := NewServiceWithOptions(backend, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{
		Setup: memberTestSetup, Configure: memberTestConfigure, Access: memberAccess{},
		PrepareFor: func(data []byte, principal v0.Principal) (app.PreparedAssignment, error) {
			return app.PrepareAssignment(data, principalSource{principal}, loader, registeredTemplate{})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	boundary, err := authentication.New(e14Verifier{}, e14Memberships{})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AuthenticatedHandler(service, boundary)
	if err != nil {
		t.Fatal(err)
	}

	var firstSpec json.RawMessage
	for _, user := range []string{"alice", "bob"} {
		headers := map[string]string{"Authorization": "Bearer " + user + "-token", "X-Team": "other-team"}
		request := httpInput(user)
		var document struct {
			Spec json.RawMessage `json:"spec"`
		}
		if err := json.Unmarshal(request, &document); err != nil {
			t.Fatal(err)
		}
		if firstSpec == nil {
			firstSpec = document.Spec
		} else if !bytes.Equal(firstSpec, document.Spec) {
			t.Fatal("members submitted different Work specs")
		}
		response := httpCall(handler, http.MethodPost, "/api/requests", request, headers)
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s submit: %d %s", user, response.Code, response.Body)
		}
		view := awaitVertical(t, service, user)
		if view.State.Principal.Subject != user || view.State.Principal.Team != "team-a" {
			t.Fatalf("%s trusted principal changed: %+v", user, view.State.Principal)
		}
		wantTimeout := aliceTimeout
		if user == "bob" {
			wantTimeout = bobTimeout
		}
		if view.State.PolicyRef.ID != "e14-two-member-caps" || view.State.PolicyRef.Version != "1" || view.State.EffectiveAuthority == nil || !slices.Equal(view.State.EffectiveAuthority.Tools, []string{"git.read"}) || time.Duration(view.State.EffectiveAuthority.Runtime.Timeout) != wantTimeout {
			t.Fatalf("%s did not receive its Policy-bounded authority: policy=%+v authority=%+v", user, view.State.PolicyRef, view.State.EffectiveAuthority)
		}
		other := "alice"
		if user == "alice" {
			other = "bob"
		}
		own := httpCall(handler, http.MethodGet, "/api/requests/"+user+"/evidence", nil, headers)
		if own.Code != http.StatusOK {
			t.Fatalf("%s own evidence: %d", user, own.Code)
		}
		foreign := httpCall(handler, http.MethodGet, "/api/requests/"+other+"/evidence", nil, headers)
		if foreign.Code != http.StatusNotFound {
			t.Fatalf("%s foreign evidence: %d", user, foreign.Code)
		}
	}
	beforeProvider, beforeBackend := provider.calls.Load(), backend.calls.Load()
	for _, token := range []string{"", "forged"} {
		response := httpCall(handler, http.MethodPost, "/api/requests", httpInput("denied"), map[string]string{"Authorization": "Bearer " + token, "X-Team": "team-a"})
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid identity accepted: %d", response.Code)
		}
	}
	if provider.calls.Load() != beforeProvider || backend.calls.Load() != beforeBackend {
		t.Fatal("invalid identity reached worker or provider")
	}
}
