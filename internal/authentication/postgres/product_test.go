// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"errors"
	"fmt"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/registration"
	"sync"
	"testing"
)

func TestProductMetadataPolicyCASAndProjectIsolation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, n := range []string{"owner", "admin", "member"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "synthetic", Subject: n, Principal: n, Team: "team-a", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner';UPDATE agenova_members SET account_role='admin' WHERE principal='admin';INSERT INTO agenova_projects VALUES('team-a','payments'),('team-a','billing');INSERT INTO agenova_project_grants VALUES('admin','team-a','payments','admin'),('member','team-a','payments','member')`)
	if err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team-a", AuthenticationContext: "oidc"}
	}
	if err = s.UpdateOrganization(ctx, p("member"), "forged", ""); !errors.Is(err, authentication.ErrForbidden) {
		t.Fatal("member changed metadata", err)
	}
	if err = s.UpdateOrganization(ctx, p("owner"), "Agenova demo team", "Display only"); err != nil {
		t.Fatal(err)
	}
	org, err := s.Organization(ctx, p("member"))
	if err != nil || org.ID != "team-a" || org.DisplayName != "Agenova demo team" {
		t.Fatal(org, err)
	}
	if err = s.UpdateOrganization(ctx, p("owner"), "", ""); !errors.Is(err, invitations.ErrInvalid) {
		t.Fatal("blank display name", err)
	}
	for _, project := range []string{"billing", "payments"} {
		if err = s.GrantProject(ctx, p("owner"), "member", project, MemberRole); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.GrantProject(ctx, p("owner"), "member", "billing", ""); err != nil {
		t.Fatal(err)
	}
	if s.Authorize(ctx, p("member"), SubmitWork, "payments") != nil || s.Authorize(ctx, p("member"), SubmitWork, "billing") == nil {
		t.Fatal("independent project edits failed")
	}
	doc := func(version string) []byte {
		return []byte(fmt.Sprintf(`{"apiVersion":"agenova.io/v1alpha1","kind":"PolicyBundle","metadata":{"name":"demo","version":"%s"},"spec":{"rules":[{"team":"team-a","action":"claim.create","project":"payments","templateRef":"engineer"}]}}`, version))
	}
	if _, err = s.ApplyGovernance(ctx, p("admin"), "payments", "policy", doc("1"), nil, &v0.PolicyReference{}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, version := range []string{"2", "3"} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			_, e := s.ApplyGovernance(ctx, p("admin"), "payments", "policy", doc(v), nil, &v0.PolicyReference{ID: "demo", Version: "1"})
			results <- e
		}(version)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, registration.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("lost-update protection", success, conflicts)
	}
	history, err := s.PolicyHistoryFor(ctx, p("member"), "payments")
	if err != nil || len(history.Versions) != 2 || len(history.Events) != 2 {
		t.Fatal(history, err)
	}
	active := *history.Active
	if _, err = s.ApplyGovernance(ctx, p("admin"), "payments", "policy", doc("1"), nil, &active); err != nil {
		t.Fatal("rollback", err)
	}
	history, err = s.PolicyHistoryFor(ctx, p("member"), "payments")
	if err != nil || history.Active.Version != "1" || len(history.Events) != 3 {
		t.Fatal(history, err)
	}
	if _, err = s.PolicyHistoryFor(ctx, p("member"), "billing"); !errors.Is(err, authentication.ErrForbidden) {
		t.Fatal("foreign project history", err)
	}
	// Project IDs may contain a slash. Structured history must not infer scope
	// from the legacy team/project/name audit string's prefix.
	if _, err = s.DB.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments/other')`); err != nil {
		t.Fatal(err)
	}
	foreign := []byte(`{"apiVersion":"agenova.io/v1alpha1","kind":"PolicyBundle","metadata":{"name":"foreign","version":"private-version"},"spec":{"rules":[]}}`)
	if _, err = s.ApplyGovernance(ctx, p("owner"), "payments/other", "policy", foreign, nil); err != nil {
		t.Fatal(err)
	}
	history, err = s.PolicyHistoryFor(ctx, p("member"), "payments")
	if err != nil || len(history.Events) != 3 {
		t.Fatal("foreign activation leaked through a project-name prefix", history, err)
	}
	if _, err = s.DB.Exec(`UPDATE agenova_members SET active=false WHERE principal='member'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Organization(ctx, p("member")); err == nil {
		t.Fatal("disabled organization read")
	}
	if _, err = s.PolicyHistoryFor(ctx, p("member"), "payments"); err == nil {
		t.Fatal("disabled history read")
	}
}

func TestPolicyHistoryFailsExplicitlyWhenVersionLimitIsExceeded(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	if err := s.PutMember(ctx, "fixture", Member{Issuer: "synthetic", Subject: "owner", Principal: "owner", Team: "team", Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; INSERT INTO agenova_projects VALUES('team','research'); INSERT INTO agenova_project_policies SELECT 'team','research','bounded',lpad(n::text,3,'0'),jsonb_build_object('ID','bounded','Version',lpad(n::text,3,'0'),'Rules','[]'::jsonb) FROM generate_series(1,101) AS n; INSERT INTO agenova_project_policy_active VALUES('team','research','bounded','101')`); err != nil {
		t.Fatal(err)
	}
	p := v0.Principal{Subject: "owner", Team: "team", AuthenticationContext: "oidc"}
	if _, err := s.PolicyHistoryFor(ctx, p, "research"); !errors.Is(err, invitations.ErrUnavailable) {
		t.Fatal("truncated history looked complete", err)
	}
	if _, err := s.DB.Exec(`DELETE FROM agenova_project_policy_active; DELETE FROM agenova_project_policies WHERE version='101'`); err != nil {
		t.Fatal(err)
	}
	h, err := s.PolicyHistoryFor(ctx, p, "research")
	if err != nil || len(h.Versions) != 100 {
		t.Fatal("bounded complete history unavailable", len(h.Versions), err)
	}
}
