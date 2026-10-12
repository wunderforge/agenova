// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/invitations"
	"testing"
)

func TestScopedDutiesAreExplicitAndRevocable(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, name := range []string{"owner", "admin", "member", "outsider"} {
		team := "team"
		if name == "outsider" {
			team = "other"
		}
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: name, Principal: name, Team: team, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; UPDATE agenova_members SET account_role='admin' WHERE principal='admin'; INSERT INTO agenova_projects VALUES('team','project'),('team','other-project'); INSERT INTO agenova_project_grants VALUES('admin','team','project','admin'),('member','team','project','member')`); err != nil {
		t.Fatal(err)
	}
	p := func(name string) v0.Principal {
		return v0.Principal{Subject: name, Team: "team", AuthenticationContext: "oidc"}
	}
	c := invitations.MemberChange{Principal: "member", Action: "grant-role", Role: GovernanceAdmin, ScopeKind: "project", ScopeRef: "project"}
	for _, name := range []string{"owner", "admin", "member"} {
		if s.AuthorizeScoped(ctx, p(name), "policy.register", "project", "project") == nil {
			t.Fatal("implicit governance access", name)
		}
	}
	for _, name := range []string{"admin", "member", "outsider"} {
		if s.ChangeMember(ctx, p(name), c) == nil {
			t.Fatal("delegation escalation", name)
		}
	}
	if err := s.ChangeMember(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if err := s.AuthorizeScoped(ctx, p("member"), "policy.register", "project", "project"); err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct{ action, kind, ref string }{{"policy.register", "project", "other-project"}, {"policy.register", "account", "team"}, {"platform.configure", "installation", "sydney"}, {"claim.issue", "project", "project"}, {"aws.admin", "project", "project"}} {
		if s.AuthorizeScoped(ctx, p("member"), x.action, x.kind, x.ref) == nil {
			t.Fatal("scope/action escape", x)
		}
	}
	if s.ChangeMember(ctx, p("admin"), invitations.MemberChange{Principal: "member", Action: "disable"}) == nil {
		t.Fatal("project admin modified privileged member")
	}
	rows, err := s.ListMembers(ctx, p("owner"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Principal == "member" {
			found = len(r.Roles) == 1
		}
	}
	if !found {
		t.Fatal("grant missing from API view")
	}
	c.Action = "revoke-role"
	if err = s.ChangeScopedRole(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if s.AuthorizeScoped(ctx, p("member"), "policy.register", "project", "project") == nil {
		t.Fatal("revoked grant allowed")
	}
	c.Action = "grant-role"
	c.ScopeKind = "account"
	c.ScopeRef = "other"
	if s.ChangeScopedRole(ctx, p("owner"), c) == nil {
		t.Fatal("foreign account grant")
	}
	c.ScopeRef = "team"
	if err = s.ChangeScopedRole(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if err = s.AuthorizeScoped(ctx, p("member"), "template.register", "project", "other-project"); err != nil {
		t.Fatal(err)
	}
	c.Role = PlatformOperator
	c.ScopeKind = "installation"
	c.ScopeRef = "sydney"
	if err = s.ChangeScopedRole(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if err = s.AuthorizeScoped(ctx, p("member"), "platform.configure", "installation", "sydney"); err != nil {
		t.Fatal(err)
	}
	if s.AuthorizeScoped(ctx, p("member"), "platform.configure", "installation", "other") == nil {
		t.Fatal("installation escape")
	}
	if err = s.ChangeMember(ctx, p("owner"), invitations.MemberChange{Principal: "member", Action: "disable"}); err != nil {
		t.Fatal(err)
	}
	if s.AuthorizeScoped(ctx, p("member"), "platform.configure", "installation", "sydney") == nil {
		t.Fatal("disabled member allowed")
	}
	var count int
	if err = s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE operation LIKE 'grant-role:%' OR operation LIKE 'revoke-role:%'`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("audit count %d: %v", count, err)
	}
}
