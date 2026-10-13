// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccessRegistrationApprovalAndBoundaries(t *testing.T) {
	ctx := context.Background()
	s := Store{invitationDB(t)}
	_, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	id := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "new", Email: "new@gmail.com", EmailAuthoritative: true}
	for range 3 {
		if status, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil || status != "pending" {
			t.Fatal(status, err)
		}
	}
	if _, _, err := s.IssueApplicationSession(ctx, id.Issuer, id.Subject, time.Hour); err == nil {
		t.Fatal("pending account received credentials")
	}
	requests, err := s.ListAccessRequests(ctx, owner)
	if err != nil || len(requests) != 1 {
		t.Fatal(requests, err)
	}
	bad := id
	bad.EmailAuthoritative = false
	if _, err := s.RegisterGoogleAccess(ctx, bad, "team"); err == nil {
		t.Fatal("unverified accepted")
	}
	if _, err := s.RegisterGoogleAccess(ctx, id, "other"); err == nil {
		t.Fatal("cross-team accepted")
	}
	ordinary := v0.Principal{Subject: "not-admin", Team: "team", AuthenticationContext: "oidc"}
	if _, err := s.ListAccessRequests(ctx, ordinary); err == nil {
		t.Fatal("ordinary request disclosure")
	}
	change := invitations.AccessReview{ID: requests[0].ID, Action: "approve", AccountRole: Admin, Project: "project", Role: Admin}
	if err := s.ReviewAccessRequest(ctx, ordinary, change); err == nil {
		t.Fatal("self approval")
	}
	invalid := change
	invalid.Project = "unknown"
	if err := s.ReviewAccessRequest(ctx, owner, invalid); err == nil {
		t.Fatal("unknown project")
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.ReviewAccessRequest(ctx, owner, change) == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("review count", success.Load())
	}
	if status, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil || status != "active" {
		t.Fatal(status, err)
	}
	if _, _, err := s.IssueApplicationSession(ctx, id.Issuer, id.Subject, time.Hour); err != nil {
		t.Fatal(err)
	}
	members, err := s.ListMembers(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range members {
		if m.Email == id.Email {
			found = m.Active && m.AccountRole == Admin && len(m.Projects) == 1 && m.Projects[0].Role == Admin
		}
	}
	if !found {
		t.Fatal("registered member or grants missing")
	}
	principal := "user:request:" + change.ID
	if err := s.ChangeMember(ctx, owner, invitations.MemberChange{Principal: principal, Action: "disable"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterGoogleAccess(ctx, id, "team"); err == nil {
		t.Fatal("disabled identity reregistered")
	}
	id.Subject = "declined"
	id.Email = "declined@gmail.com"
	if _, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil {
		t.Fatal(err)
	}
	requests, _ = s.ListAccessRequests(ctx, owner)
	if len(requests) != 1 {
		t.Fatal(requests)
	}
	if err := s.ReviewAccessRequest(ctx, owner, invitations.AccessReview{ID: requests[0].ID, Action: "reject"}); err != nil {
		t.Fatal(err)
	}
	if status, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil || status != "rejected" {
		t.Fatal("rejection reopened", status, err)
	}
}
func TestAccessApprovalAdminScopeAndAuditAtomicity(t *testing.T) {
	ctx := context.Background()
	s := Store{invitationDB(t)}
	_, proof, _ := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: Admin})
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	_, proof, err = s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "admin@gmail.com", Project: "project", Role: Admin, AccountRole: Admin})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "admin", Email: "admin@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	id := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "pending", Email: "pending@gmail.com", EmailAuthoritative: true}
	if _, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListAccessRequests(ctx, owner)
	c := invitations.AccessReview{ID: rows[0].ID, Action: "approve", AccountRole: Admin, Project: "project", Role: MemberRole}
	if s.ReviewAccessRequest(ctx, admin, c) == nil {
		t.Fatal("admin appointed account admin")
	}
	c.AccountRole = MemberRole
	c.Role = Admin
	if s.ReviewAccessRequest(ctx, admin, c) == nil {
		t.Fatal("admin appointed project admin")
	}
	c.Role = MemberRole
	c.Project = "other"
	if _, err := s.DB.Exec(`INSERT INTO agenova_projects(team,project) VALUES('team','other')`); err != nil {
		t.Fatal(err)
	}
	if s.ReviewAccessRequest(ctx, admin, c) == nil {
		t.Fatal("cross project approval")
	}
	c.Project = "project"
	if _, err := s.DB.Exec(`CREATE FUNCTION reject_access_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation LIKE 'access.approved%' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_access_audit BEFORE INSERT ON agenova_identity_audit FOR EACH ROW EXECUTE FUNCTION reject_access_audit()`); err != nil {
		t.Fatal(err)
	}
	if s.ReviewAccessRequest(ctx, admin, c) == nil {
		t.Fatal("audit failure accepted")
	}
	if status, err := s.RegisterGoogleAccess(ctx, id, "team"); err != nil || status != "pending" {
		t.Fatal("partial approval", status, err)
	}
	if _, _, err := s.IssueApplicationSession(ctx, id.Issuer, id.Subject, time.Hour); err == nil {
		t.Fatal("partial membership")
	}
	if _, err := s.DB.Exec(`DROP TRIGGER reject_access_audit ON agenova_identity_audit`); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewAccessRequest(ctx, admin, c); err != nil {
		t.Fatal(err)
	}
}
