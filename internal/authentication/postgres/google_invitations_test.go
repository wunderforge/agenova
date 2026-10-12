// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
)

func TestGoogleInviteBindingAndReplay(t *testing.T) {
	ctx := context.Background()
	s := Store{invitationDB(t)}
	ownerInvite, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "other@gmail.com", Project: "project", Role: Admin}); err == nil {
		t.Fatal("second bootstrap accepted")
	}
	ownerID := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner-sub", Email: "owner@gmail.com", EmailAuthoritative: true}
	wrong := ownerID
	wrong.Email = "other@gmail.com"
	if _, err := s.AcceptGoogleInvitation(ctx, wrong, proof); err == nil {
		t.Fatal("wrong Google account accepted")
	}
	wrong = ownerID
	wrong.EmailAuthoritative = false
	if _, err := s.AcceptGoogleInvitation(ctx, wrong, proof); err == nil {
		t.Fatal("unverified email ownership accepted")
	}
	owner, err := s.AcceptGoogleInvitation(ctx, ownerID, proof)
	if err != nil {
		t.Fatal(err)
	}
	if owner.Subject != "user:"+ownerInvite.ID {
		t.Fatal("wrong principal binding")
	}
	if _, err := s.AcceptGoogleInvitation(ctx, ownerID, proof); err == nil {
		t.Fatal("invitation replay")
	}
	invite, proof, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "admin@gmail.com", Project: "project", Role: Admin, AccountRole: Admin})
	if err != nil {
		t.Fatal(err)
	}
	adminID := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "admin-sub", Email: "admin@gmail.com", EmailAuthoritative: true}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.AcceptGoogleInvitation(ctx, adminID, proof); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("concurrent acceptance count", successes.Load())
	}
	var account, role string
	if err := s.DB.QueryRow(`SELECT account_role FROM agenova_members WHERE subject='admin-sub'`).Scan(&account); err != nil || account != Admin {
		t.Fatal("Admin grant missing")
	}
	if err := s.DB.QueryRow(`SELECT role FROM agenova_project_grants WHERE principal=$1`, "user:"+invite.ID).Scan(&role); err != nil || role != Admin {
		t.Fatal("project grant missing")
	}
	raw, _, err := s.IssueApplicationSession(ctx, adminID.Issuer, adminID.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (ApplicationSessions{s}).Verify(ctx, raw); err != nil {
		t.Fatal("accepted account cannot login")
	}
	// A new email invite must not link an already-bound subject to a new role.
	_, anotherProof, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "alias@gmail.com", Project: "project", Role: MemberRole})
	if err != nil {
		t.Fatal(err)
	}
	adminID.Email = "alias@gmail.com"
	if _, err := s.AcceptGoogleInvitation(ctx, adminID, anotherProof); err == nil {
		t.Fatal("existing subject silently linked")
	}
}

func TestGoogleInvitationExpiredAndInviterRevoked(t *testing.T) {
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
	i, proof, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "member@gmail.com", Project: "project", Role: MemberRole})
	if err != nil {
		t.Fatal(err)
	}
	id := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "member", Email: "member@gmail.com", EmailAuthoritative: true}
	if _, err := s.DB.Exec(`UPDATE agenova_invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, i.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptGoogleInvitation(ctx, id, proof); err == nil {
		t.Fatal("expired invite accepted")
	}
	if _, err := s.DB.Exec(`UPDATE agenova_invitations SET expires_at=now()+interval '1 hour' WHERE id=$1`, i.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET active=false WHERE principal=$1`, owner.Subject); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptGoogleInvitation(ctx, id, proof); err == nil {
		t.Fatal("inactive inviter authority retained")
	}
}
