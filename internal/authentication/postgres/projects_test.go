// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"errors"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

func TestOrganizationAdministratorsAndAuditedProjectCreation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, name := range []string{"owner", "admin", "member", "project-admin"} {
		if err := s.PutMember(ctx, "fixture", Member{Issuer: "synthetic", Subject: name, Principal: name, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; UPDATE agenova_members SET account_role='admin' WHERE principal='admin'; INSERT INTO agenova_projects VALUES('team','payments'); INSERT INTO agenova_project_grants VALUES('project-admin','team','payments','admin')`); err != nil {
		t.Fatal(err)
	}
	p := func(name string) v0.Principal {
		return v0.Principal{Subject: name, Team: "team", AuthenticationContext: "oidc"}
	}
	for _, name := range []string{"member", "project-admin", "missing"} {
		if _, err := s.CreateProject(ctx, p(name), "research"); !errors.Is(err, authentication.ErrForbidden) {
			t.Fatal("unauthorized project creation", name, err)
		}
	}
	for _, value := range []string{"", " research", "research/other", "Research", "a\n"} {
		if _, err := s.CreateProject(ctx, p("admin"), value); !errors.Is(err, invitations.ErrInvalid) {
			t.Fatal("invalid project accepted", value, err)
		}
	}
	created, err := s.CreateProject(ctx, p("admin"), "research")
	if err != nil || !created {
		t.Fatal("admin project creation", created, err)
	}
	created, err = s.CreateProject(ctx, p("owner"), "research")
	if err != nil || created {
		t.Fatal("project reapply not idempotent", created, err)
	}
	var audits, grants int
	if s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE operation='project.create' AND principal='team/research' AND actor='admin'`).Scan(&audits) != nil || audits != 1 || s.DB.QueryRow(`SELECT count(*) FROM agenova_project_grants WHERE project='research'`).Scan(&grants) != nil || grants != 0 {
		t.Fatal("project creation audit/grants", audits, grants)
	}
	for _, name := range []string{"owner", "admin"} {
		for _, action := range []string{SubmitWork, ReadOwnWork, ReadProjectWork, ManageMembers} {
			if s.Authorize(ctx, p(name), action, "research") != nil {
				t.Fatal("new project unavailable", name, action)
			}
		}
		projects, err := s.ConfigurationProjects(ctx, p(name))
		if err != nil || len(projects) != 2 {
			t.Fatal("configuration discovery", projects, err)
		}
		if s.AuthorizeScoped(ctx, p(name), "template.register", "project", "research") != nil {
			t.Fatal("registration unavailable", name)
		}
		foreign := p(name)
		foreign.Team = "foreign"
		if s.Authorize(ctx, foreign, SubmitWork, "research") == nil || s.Authorize(ctx, p(name), SubmitWork, "missing") == nil || s.Authorize(ctx, p(name), "unknown", "research") == nil {
			t.Fatal("team/project/action escape", name)
		}
	}
	for _, name := range []string{"member", "project-admin"} {
		if s.Authorize(ctx, p(name), SubmitWork, "research") == nil || s.AuthorizeScoped(ctx, p(name), "template.register", "project", "research") == nil {
			t.Fatal("ordinary account escaped project", name)
		}
	}
	// Audit failure must leave no orphan project.
	if _, err := s.DB.Exec(`CREATE FUNCTION reject_project_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation='project.create' THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_project_audit BEFORE INSERT ON agenova_identity_audit FOR EACH ROW EXECUTE FUNCTION reject_project_audit()`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, p("admin"), "rollback"); err == nil {
		t.Fatal("audit failure ignored")
	}
	var exists bool
	if s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM agenova_projects WHERE project='rollback')`).Scan(&exists) != nil || exists {
		t.Fatal("failed creation persisted")
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='member' WHERE principal='admin'`); err != nil {
		t.Fatal(err)
	}
	if s.Authorize(ctx, p("admin"), SubmitWork, "research") == nil || s.AuthorizeScoped(ctx, p("admin"), "policy.register", "project", "research") == nil {
		t.Fatal("demotion did not revoke access")
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='admin',active=false WHERE principal='admin'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, p("admin"), "disabled"); !errors.Is(err, authentication.ErrForbidden) {
		t.Fatal("disabled admin accepted", err)
	}
}
