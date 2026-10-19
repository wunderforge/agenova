// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"errors"
	"fmt"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/registration"
	"os"
	"strings"
	"testing"
)

func TestProjectGovernanceIsolationRevocationAndAtomicActivation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, n := range []string{"owner", "governor", "member"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: n, Principal: n, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; INSERT INTO agenova_projects VALUES('team','one'),('team','two')`); err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team", AuthenticationContext: "oidc"}
	}
	doc := func(project, version string) []byte {
		return []byte(fmt.Sprintf("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: scoped\n  version: '%s'\nspec:\n  rules:\n    - team: team\n      project: %s\n      action: claim.create\n      templateRef: engineer\n", version, project))
	}
	apply := func(actor, project, version string) error {
		_, err := s.ApplyGovernance(ctx, p(actor), project, "policy", doc(project, version), nil)
		return err
	}
	for _, n := range []string{"owner", "governor", "member"} {
		if apply(n, "one", "1") == nil {
			t.Fatal("implicit governance access", n)
		}
	}
	c := invitations.MemberChange{Principal: "governor", Action: "grant-role", Role: GovernanceAdmin, ScopeKind: "project", ScopeRef: "one"}
	if err := s.ChangeMember(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if apply("governor", "two", "1") == nil {
		t.Fatal("foreign project write")
	}
	if _, err := s.ApplyGovernance(ctx, p("governor"), "one", "policy", doc("two", "1"), nil); err == nil {
		t.Fatal("foreign project rules")
	}
	result, err := s.ApplyGovernance(ctx, p("governor"), "one", "policy", doc("one", "1"), nil)
	if err != nil || !result.Changed || !result.Active {
		t.Fatal(result, err)
	}
	result, err = s.ApplyGovernance(ctx, p("governor"), "one", "policy", doc("one", "1"), nil)
	if err != nil || result.Changed {
		t.Fatal("identical reapply", result, err)
	}
	conflict := []byte("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: scoped\n  version: '1'\nspec:\n  rules: []\n")
	if _, err = s.ApplyGovernance(ctx, p("governor"), "one", "policy", conflict, nil); !errors.Is(err, registration.ErrConflict) {
		t.Fatal("immutable identity", err)
	}
	registry := ProjectRegistry{Store: s, Team: "team", Project: "one"}
	current, err := registry.ActivePolicy()
	if err != nil || current.Version != "1" || len(current.Rules) != 1 {
		t.Fatal(current, err)
	}
	if _, err = (ProjectRegistry{Store: s, Team: "team", Project: "two"}).ActivePolicy(); err == nil {
		t.Fatal("foreign policy disclosed")
	}
	if _, err = s.DB.Exec(`CREATE FUNCTION reject_governance() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation LIKE 'governance.%' THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_governance BEFORE INSERT ON agenova_identity_audit FOR EACH ROW EXECUTE FUNCTION reject_governance()`); err != nil {
		t.Fatal(err)
	}
	if apply("governor", "one", "2") == nil {
		t.Fatal("audit failure accepted")
	}
	current, err = registry.ActivePolicy()
	if err != nil || current.Version != "1" {
		t.Fatal("activation not rolled back", current, err)
	}
	var count int
	if err = s.DB.QueryRow(`SELECT count(*) FROM agenova_project_policies WHERE version='2'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed version persisted", count, err)
	}
	if _, err = s.DB.Exec(`DROP TRIGGER reject_governance ON agenova_identity_audit`); err != nil {
		t.Fatal(err)
	}
	c.Action = "revoke-role"
	if err = s.ChangeMember(ctx, p("owner"), c); err != nil {
		t.Fatal(err)
	}
	if apply("governor", "one", "2") == nil {
		t.Fatal("revoked role wrote policy")
	}
}

func TestGovernanceTemplateCompatibilityAndConfigurationVisibility(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, n := range []string{"owner", "governor", "member"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: n, Principal: n, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; INSERT INTO agenova_projects VALUES('team','one'),('team','two'); INSERT INTO agenova_project_grants VALUES('member','team','two','viewer')`); err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team", AuthenticationContext: "oidc"}
	}
	grant := invitations.MemberChange{Principal: "governor", Action: "grant-role", Role: GovernanceAdmin, ScopeKind: "project", ScopeRef: "one"}
	if err := s.ChangeMember(ctx, p("owner"), grant); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"owner": "one,two", "governor": "one", "member": "two"} {
		projects, err := s.ConfigurationProjects(ctx, p(name))
		if err != nil || strings.Join(projects, ",") != want {
			t.Fatal(name, projects, err)
		}
	}
	doc, err := os.ReadFile("../../../deploy/reference/demo/engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyGovernance(ctx, p("governor"), "one", "template", doc, nil); err == nil {
		t.Fatal("missing compatibility check allowed")
	}
	if _, err = s.ApplyGovernance(ctx, p("governor"), "one", "template", doc, func(*v0.AgentTemplate) error { return errors.New("incompatible runtime") }); err == nil {
		t.Fatal("incompatible template registered")
	}
	validate := func(*v0.AgentTemplate) error { return nil }
	result, err := s.ApplyGovernance(ctx, p("governor"), "one", "template", doc, validate)
	if err != nil || !result.Changed {
		t.Fatal(result, err)
	}
	result, err = s.ApplyGovernance(ctx, p("governor"), "one", "template", doc, validate)
	if err != nil || result.Changed {
		t.Fatal(result, err)
	}
	changed := []byte(strings.Replace(string(doc), "git.read", "git.write", 1))
	if _, err = s.ApplyGovernance(ctx, p("governor"), "one", "template", changed, validate); !errors.Is(err, registration.ErrConflict) {
		t.Fatal("mutable template", err)
	}
	registry := ProjectRegistry{Store: s, Team: "team", Project: "one"}
	template, err := registry.Lookup("engineer")
	if err != nil || template.Metadata.Name != "engineer" {
		t.Fatal(template, err)
	}
	if _, err = (ProjectRegistry{Store: s, Team: "team", Project: "two"}).Lookup("engineer"); err == nil {
		t.Fatal("foreign template disclosed")
	}
	names, err := registry.TemplateNames([]string{"base"})
	if err != nil || strings.Join(names, ",") != "base,engineer" {
		t.Fatal(names, err)
	}
}
