// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"testing"

	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
)

func TestGoogleInvitationCancellationReplacement(t *testing.T) {
	ctx := context.Background()
	s := Store{invitationDB(t)}
	ownerInvite, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	invite, old, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "member@gmail.com", Project: "project", Role: MemberRole})
	if err != nil {
		t.Fatal(err)
	}
	id := googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "member", Email: "member@gmail.com", EmailAuthoritative: true}
	if _, _, err = s.ManageGoogleInvitation(ctx, owner, ownerInvite.ID, "reissue"); err == nil {
		t.Fatal("owner invitation changed")
	}
	forged := owner
	forged.Team = "foreign"
	if _, _, err = s.ManageGoogleInvitation(ctx, forged, invite.ID, "cancel"); err == nil {
		t.Fatal("cross-team cancel")
	}
	if _, _, err = s.ManageGoogleInvitation(ctx, owner, invite.ID, "cancel"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptGoogleInvitation(ctx, id, old); err == nil {
		t.Fatal("cancelled proof accepted")
	}
	_, fresh, err := s.ManageGoogleInvitation(ctx, owner, invite.ID, "reissue")
	if err != nil || fresh == old {
		t.Fatal("replacement", err)
	}
	if _, err = s.AcceptGoogleInvitation(ctx, id, old); err == nil {
		t.Fatal("replaced proof accepted")
	}
	if _, err = s.DB.Exec(`UPDATE agenova_invitations SET expires_at=now()-interval '1 minute' WHERE id=$1`, invite.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListInvitations(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ID == invite.ID {
			found = row.Status == "expired" && row.LinkManaged && row.AcceptanceURL == ""
		}
	}
	if !found {
		t.Fatal("expired status/proof leak")
	}
	if _, err = s.AcceptGoogleInvitation(ctx, id, fresh); err == nil {
		t.Fatal("expired proof accepted")
	}
	_, fresh, err = s.ManageGoogleInvitation(ctx, owner, invite.ID, "reissue")
	if err != nil {
		t.Fatal(err)
	}
	member, err := s.AcceptGoogleInvitation(ctx, id, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.ManageGoogleInvitation(ctx, owner, invite.ID, "reissue"); err == nil {
		t.Fatal("accepted invitation reissued")
	}
	other, _, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: "another@gmail.com", Project: "project", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.ManageGoogleInvitation(ctx, member, other.ID, "cancel"); err == nil {
		t.Fatal("member managed invitation")
	}
	for n := 0; n < 8; n++ {
		if _, _, err = s.ManageGoogleInvitation(ctx, owner, other.ID, "reissue"); err != nil {
			t.Fatal(n, err)
		}
	}
	if _, _, err = s.ManageGoogleInvitation(ctx, owner, other.ID, "reissue"); err == nil {
		t.Fatal("reissue cap missing")
	}
}
