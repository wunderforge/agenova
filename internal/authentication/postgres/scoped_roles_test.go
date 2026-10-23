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
	for _, name := range []string{"member"} {
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

func TestTeamAdministratorBundleAndRevocation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, name := range []string{"team-admin", "account-only", "project-only", "viewer", "owner"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: name, Principal: name, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`INSERT INTO agenova_projects VALUES('team','one'),('team','two'); UPDATE agenova_members SET account_role='admin' WHERE principal IN ('team-admin','account-only'); UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; INSERT INTO agenova_project_grants VALUES('team-admin','team','one','admin'),('project-only','team','one','admin'),('viewer','team','one','viewer')`); err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team", AuthenticationContext: "oidc"}
	}
	for _, name := range []string{"project-only", "viewer"} {
		if s.AuthorizeInstallation(ctx, p(name), "platform.configure", "installed", "installed") == nil {
			t.Fatal("deployment escalation", name)
		}
		if s.AuthorizeScoped(ctx, p(name), "policy.register", "project", "one") == nil {
			t.Fatal("governance escalation", name)
		}
	}
	for _, name := range []string{"owner", "account-only", "team-admin"} {
		if s.AuthorizeInstallation(ctx, p(name), "platform.configure", "installed", "installed") != nil || s.AuthorizeScoped(ctx, p(name), "policy.register", "project", "two") != nil {
			t.Fatal("organization administrator lacks bundled access", name)
		}
	}
	for _, action := range []string{"platform.configure", "platform.diagnose"} {
		if err := s.AuthorizeInstallation(ctx, p("team-admin"), action, "installed", "installed"); err != nil {
			t.Fatal(err)
		}
	}
	if s.AuthorizeInstallation(ctx, p("team-admin"), "platform.configure", "other", "installed") == nil {
		t.Fatal("foreign installation")
	}
	if s.AuthorizeInstallation(ctx, p("team-admin"), "aws.admin", "installed", "installed") == nil {
		t.Fatal("unknown action")
	}
	if s.AuthorizeScoped(ctx, p("team-admin"), "policy.register", "project", "missing") == nil {
		t.Fatal("nonexistent project")
	}
	if s.AuthorizeScoped(ctx, p("team-admin"), "policy.register", "account", "team") == nil {
		t.Fatal("account governance escalation")
	}
	doc := []byte("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: team-policy\n  version: '1'\nspec:\n  rules:\n    - team: team\n      project: one\n      action: claim.create\n      templateRef: engineer\n")
	if _, err := s.ApplyGovernance(ctx, p("team-admin"), "one", "policy", doc, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyGovernance(ctx, p("team-admin"), "two", "policy", doc, nil); err == nil {
		t.Fatal("cross-project write")
	}
	for _, mutation := range []string{`UPDATE agenova_members SET account_role='member' WHERE principal='team-admin'`, `UPDATE agenova_members SET account_role='admin',active=false WHERE principal='team-admin'`} {
		if _, err := s.DB.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		if s.AuthorizeInstallation(ctx, p("team-admin"), "platform.configure", "installed", "installed") == nil {
			t.Fatal("revoked deployment")
		}
		if _, err := s.ApplyGovernance(ctx, p("team-admin"), "one", "policy", doc, nil); err == nil {
			t.Fatal("revoked governance")
		}
	}
}

func TestGrafanaRolesUseLiveMembership(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, name := range []string{"owner", "admin", "operator", "member"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: name, Principal: name, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; UPDATE agenova_members SET account_role='admin' WHERE principal='admin'; INSERT INTO agenova_projects VALUES('team','project'); INSERT INTO agenova_project_grants VALUES('admin','team','project','admin'); INSERT INTO agenova_scoped_roles VALUES('operator','team','platform-operator','installation','installed')`); err != nil {
		t.Fatal(err)
	}
	p := func(name string) v0.Principal {
		return v0.Principal{Subject: name, Team: "team", AuthenticationContext: "oidc"}
	}
	for name, want := range map[string]string{"owner": "Editor", "admin": "Editor", "operator": "Viewer"} {
		if role, err := s.GrafanaRole(ctx, p(name), "installed"); err != nil || role != want {
			t.Fatal(name, role, err)
		}
	}
	for _, name := range []string{"member", "missing"} {
		if _, err := s.GrafanaRole(ctx, p(name), "installed"); err == nil {
			t.Fatal("unauthorized", name)
		}
	}
	if _, err := s.GrafanaRole(ctx, p("operator"), "other"); err == nil {
		t.Fatal("installation escape")
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET active=false WHERE principal='admin'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GrafanaRole(ctx, p("admin"), "installed"); err == nil {
		t.Fatal("revoked admin")
	}
}
