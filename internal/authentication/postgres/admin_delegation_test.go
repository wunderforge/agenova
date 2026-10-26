// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/invitations"
)

func TestOrganizationAdminDelegationWithoutProjectGrants(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, name := range []string{"owner", "admin", "member", "project-admin", "foreign"} {
		team := "team"
		if name == "foreign" {
			team = "other"
		}
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: name, Principal: name, Team: team, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; UPDATE agenova_members SET account_role='admin' WHERE principal='admin'; INSERT INTO agenova_projects VALUES('team','one'),('team','two'); INSERT INTO agenova_project_grants VALUES('project-admin','team','one','admin'),('member','team','two','viewer')`); err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team", AuthenticationContext: "oidc"}
	}
	changes := []invitations.MemberChange{
		{Action: "platform-role", Principal: "member", Role: PlatformAdmin, Project: "two", ScopeRef: "installed"},
		{Action: "platform-role", Principal: "member", Role: PlatformOperator, Project: "two", ScopeRef: "installed"},
		{Action: "account-role", Principal: "member", Role: Admin},
		{Action: "project-role", Principal: "member", Role: Admin, Project: "two"},
		{Action: "project-role", Principal: "member", Role: MemberRole, Project: "two"},
		{Action: "grant-role", Principal: "member", Role: GovernanceAdmin, ScopeKind: "project", ScopeRef: "two"},
		{Action: "revoke-role", Principal: "member", Role: GovernanceAdmin, ScopeKind: "project", ScopeRef: "two"},
		{Action: "disable", Principal: "member"},
		{Action: "enable", Principal: "member"},
	}
	for _, c := range changes {
		for _, actor := range []string{"project-admin", "foreign"} {
			if s.ChangeMember(ctx, p(actor), c) == nil {
				t.Fatalf("unauthorized delegation: %s %s", actor, c.Action)
			}
		}
		for _, target := range []string{"owner", "admin", "foreign"} {
			if target == "admin" && c.Action == "project-role" {
				continue // Independent project grants do not change account authority.
			}
			bad := c
			bad.Principal = target
			if s.ChangeMember(ctx, p("admin"), bad) == nil {
				t.Fatalf("protected target %s: %s", target, c.Action)
			}
		}
		if err := s.ChangeMember(ctx, p("admin"), c); err != nil {
			t.Fatalf("Admin %s: %v", c.Action, err)
		}
	}
	// Invitation authority is independent of explicit project grants too.
	if _, _, err := s.ReserveGoogleInvitation(ctx, p("admin"), invitations.Request{Email: "new@example.com", AccountRole: Admin, Project: "two", Role: Admin}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE actor='admin' AND principal='member'`).Scan(&count); err != nil || count != len(changes) {
		t.Fatalf("audit %d: %v", count, err)
	}
	for _, mutation := range []string{`UPDATE agenova_members SET active=false WHERE principal='admin'`, `UPDATE agenova_members SET active=true,account_role='member' WHERE principal='admin'`} {
		if _, err := s.DB.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		if s.ChangeMember(ctx, p("admin"), changes[0]) == nil {
			t.Fatal("revoked administrator retained delegation")
		}
	}
}
