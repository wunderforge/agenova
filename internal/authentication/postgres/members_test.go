// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
)

func TestMemberAdministrationAndLoginInvalidation(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	_, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	principals := map[string]v0.Principal{"owner": owner}
	for _, name := range []string{"admin", "member", "viewer"} {
		_, proof, err = s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: name + "@gmail.com", Project: "project", Role: name})
		if err != nil {
			t.Fatal(err)
		}
		p, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: name, Email: name + "@gmail.com", EmailAuthoritative: true}, proof)
		if err != nil {
			t.Fatal(err)
		}
		principals[name] = p
	}
	admin, member := principals["admin"], principals["member"]
	raw, _, err := s.IssueApplicationSession(ctx, googleoidc.Issuer, "member", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	change := func(actor v0.Principal, target, action, role string) error {
		return s.ChangeMember(ctx, actor, invitations.MemberChange{Principal: principals[target].Subject, Action: action, Role: role})
	}
	for _, name := range []string{"member", "viewer"} {
		if _, err := s.ListMembers(ctx, principals[name]); err == nil {
			t.Fatal("nonadministrator listed members")
		}
		if err := change(principals[name], "member", "account-role", Admin); err == nil {
			t.Fatal("self elevation")
		}
	}
	rows, err := s.ListMembers(ctx, admin)
	if err != nil || len(rows) != 4 {
		t.Fatal("admin listing", err)
	}
	for _, row := range rows {
		if row.Email == "" || len(row.Projects) != 1 {
			t.Fatal("missing member description")
		}
	}
	if err := change(admin, "member", "account-role", Admin); err == nil {
		t.Fatal("admin appointed admin")
	}
	if err := change(owner, "owner", "disable", ""); err == nil {
		t.Fatal("owner disabled")
	}
	if err := change(owner, "owner", "account-role", MemberRole); err == nil {
		t.Fatal("owner demoted")
	}
	if err := change(admin, "admin", "disable", ""); err == nil {
		t.Fatal("admin disabled privileged account")
	}
	if err := change(admin, "member", "disable", ""); err != nil {
		t.Fatal(err)
	}
	verifier := ApplicationSessions{s}
	if _, err := verifier.Verify(ctx, raw); err == nil {
		t.Fatal("disabled session valid")
	}
	if err := change(admin, "member", "enable", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(ctx, raw); err == nil {
		t.Fatal("re-enable revived old session")
	}
	if _, _, err := s.IssueApplicationSession(ctx, googleoidc.Issuer, "member", time.Hour); err != nil {
		t.Fatal("fresh login denied")
	}
	if err := change(owner, "member", "account-role", Admin); err != nil {
		t.Fatal(err)
	}
	if err := change(admin, "member", "disable", ""); err == nil {
		t.Fatal("admin disabled new admin")
	}
	if err := change(owner, "member", "account-role", MemberRole); err != nil {
		t.Fatal(err)
	}
	forged := owner
	forged.Team = "other"
	if _, err := s.ListMembers(ctx, forged); err == nil {
		t.Fatal("cross-team listing")
	}
	if err := s.ChangeMember(ctx, forged, invitations.MemberChange{Principal: member.Subject, Action: "disable"}); err == nil {
		t.Fatal("cross-team change")
	}
	if err := s.ChangeMember(ctx, owner, invitations.MemberChange{Principal: member.Subject, Action: "project-role", Project: "project", Role: Viewer}); err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(ctx, member, SubmitWork, "project"); err == nil {
		t.Fatal("viewer retained submit")
	}

	if _, err := s.DB.Exec(`INSERT INTO agenova_projects VALUES('team','private')`); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeMember(ctx, admin, invitations.MemberChange{Principal: member.Subject, Action: "project-role", Project: "private", Role: MemberRole}); err == nil {
		t.Fatal("admin granted outside its project")
	}
	if _, _, err := s.ReserveGoogleInvitation(ctx, admin, invitations.Request{Email: "foreign@gmail.com", Project: "private", Role: MemberRole}); err == nil {
		t.Fatal("admin invited outside its project")
	}
	if err := s.GrantProject(ctx, owner, member.Subject, "private", Viewer); err != nil {
		t.Fatal(err)
	}
	if err := change(admin, "member", "disable", ""); err == nil {
		t.Fatal("admin disabled membership spanning another project")
	}
	if err := s.GrantProject(ctx, owner, member.Subject, "private", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantProject(ctx, owner, member.Subject, "project", Admin); err != nil {
		t.Fatal(err)
	}
	if err := change(admin, "member", "disable", ""); err == nil {
		t.Fatal("admin disabled project administrator")
	}

	_, pendingProof, err := s.ReserveGoogleInvitation(ctx, admin, invitations.Request{Email: "pending@gmail.com", Project: "project", Role: MemberRole})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantProject(ctx, owner, admin.Subject, "project", Viewer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "pending", Email: "pending@gmail.com", EmailAuthoritative: true}, pendingProof); err == nil {
		t.Fatal("removed project administrator's invite accepted")
	}
	var count int
	if err := s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE operation LIKE 'member.%'`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("audit: %d %v", count, err)
	}
}
