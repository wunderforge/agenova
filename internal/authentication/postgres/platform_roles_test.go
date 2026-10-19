// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
	"testing"
)

func TestThreeRolesTransitionsAndRevocation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	_, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "payments", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	_, proof, err = s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "member@gmail.com", Project: "payments", Role: MemberRole})
	if err != nil {
		t.Fatal(err)
	}
	member, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "member", Email: "member@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	change := func(actor v0.Principal, role string) error {
		return s.ChangeMember(ctx, actor, invitations.MemberChange{Action: "platform-role", Principal: member.Subject, Role: role, Project: "payments", ScopeRef: "installed"})
	}
	if change(member, PlatformAdmin) == nil {
		t.Fatal("member self escalation")
	}
	if change(owner, "bogus") == nil {
		t.Fatal("unknown role")
	}
	foreign := owner
	foreign.Team = "foreign"
	if change(foreign, PlatformAdmin) == nil {
		t.Fatal("cross team")
	}
	if err = change(owner, PlatformAdmin); err != nil {
		t.Fatal(err)
	}
	if s.Authorize(ctx, member, SubmitWork, "payments") != nil || s.AuthorizeScoped(ctx, member, "policy.register", "project", "payments") != nil || s.AuthorizeInstallation(ctx, member, "platform.configure", "installed", "installed") != nil {
		t.Fatal("admin capabilities missing")
	}
	if s.AuthorizeInstallation(ctx, member, "platform.configure", "other", "installed") == nil {
		t.Fatal("wrong installation")
	}
	// A Platform Admin can appoint another admin and later change their role.
	otherID := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "other", Email: "other@gmail.com", EmailAuthoritative: true}
	if _, err = s.RegisterGoogleAccess(ctx, otherID, "team"); err != nil {
		t.Fatal(err)
	}
	requests, err := s.ListAccessRequests(ctx, member)
	if err != nil || len(requests) != 1 {
		t.Fatalf("requests: %v %v", requests, err)
	}
	if err = s.ReviewAccessRequest(ctx, member, invitations.AccessReview{ID: requests[0].ID, Action: "approve", PlatformRole: PlatformAdmin, Installation: "installed", Project: "payments"}); err != nil {
		t.Fatal(err)
	}
	var other string
	if err = s.DB.QueryRow(`SELECT principal FROM agenova_members WHERE subject='other'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeMember(ctx, member, invitations.MemberChange{Action: "platform-role", Principal: other, Role: PlatformOperator, Project: "payments", ScopeRef: "installed"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeMember(ctx, member, invitations.MemberChange{Action: "platform-role", Principal: owner.Subject, Role: MemberRole, Project: "payments", ScopeRef: "installed"}); err == nil {
		t.Fatal("admin changed Owner")
	}
	if change(member, MemberRole) == nil {
		t.Fatal("admin changed own role")
	}
	if err = change(owner, PlatformOperator); err != nil {
		t.Fatal(err)
	}
	if s.AuthorizeInstallation(ctx, member, "platform.configure", "installed", "installed") != nil {
		t.Fatal("operator deployment denied")
	}
	if s.Authorize(ctx, member, SubmitWork, "payments") == nil || s.Authorize(ctx, member, ReadProjectWork, "payments") == nil || s.AuthorizeScoped(ctx, member, "policy.register", "project", "payments") == nil {
		t.Fatal("operator business/governance leak")
	}
	if _, err = s.ListMembers(ctx, member); err == nil {
		t.Fatal("operator member administration")
	}
	if err = change(owner, MemberRole); err != nil {
		t.Fatal(err)
	}
	if s.Authorize(ctx, member, SubmitWork, "payments") != nil {
		t.Fatal("member submit denied")
	}
	if s.AuthorizeInstallation(ctx, member, "platform.configure", "installed", "installed") == nil || s.AuthorizeScoped(ctx, member, "policy.register", "project", "payments") == nil || s.Authorize(ctx, member, ReadProjectWork, "payments") == nil {
		t.Fatal("demotion retained authority")
	}
	if err = s.ChangeMember(ctx, owner, invitations.MemberChange{Action: "disable", Principal: member.Subject}); err != nil {
		t.Fatal(err)
	}
	if change(owner, PlatformAdmin) == nil {
		t.Fatal("disabled member revived")
	}
	var count int
	if s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE principal=$1 AND operation LIKE 'platform-role.%'`, member.Subject).Scan(&count) != nil || count != 3 {
		t.Fatalf("audit count %d", count)
	}
}

func TestOperatorApprovalAndAuditRollback(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	_, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "payments", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	id := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "operator", Email: "operator@gmail.com", EmailAuthoritative: true}
	if _, err = s.RegisterGoogleAccess(ctx, id, "team"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListAccessRequests(ctx, owner)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	review := invitations.AccessReview{ID: rows[0].ID, Action: "approve", PlatformRole: PlatformOperator, Project: "payments", Installation: "installed"}
	if err = s.ReviewAccessRequest(ctx, owner, review); err != nil {
		t.Fatal(err)
	}
	operator := v0.Principal{Subject: "user:request:" + rows[0].ID, Team: "team", AuthenticationContext: "oidc"}
	if s.AuthorizeInstallation(ctx, operator, "platform.configure", "installed", "installed") != nil || s.Authorize(ctx, operator, SubmitWork, "payments") == nil {
		t.Fatal("operator approval grants incorrect capabilities")
	}
	if _, err = s.DB.Exec(`CREATE FUNCTION reject_role_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation LIKE 'platform-role.%' THEN RAISE EXCEPTION 'test audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_role_audit BEFORE INSERT ON agenova_identity_audit FOR EACH ROW EXECUTE FUNCTION reject_role_audit()`); err != nil {
		t.Fatal(err)
	}
	if s.ChangeMember(ctx, owner, invitations.MemberChange{Action: "platform-role", Principal: operator.Subject, Role: MemberRole, Project: "payments", ScopeRef: "installed"}) == nil {
		t.Fatal("audit failure committed")
	}
	if s.AuthorizeInstallation(ctx, operator, "platform.configure", "installed", "installed") != nil || s.Authorize(ctx, operator, SubmitWork, "payments") == nil {
		t.Fatal("role migration did not roll back")
	}
}
