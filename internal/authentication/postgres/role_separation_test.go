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
)

func TestIndependentInvitationRolesAndScopedAdministration(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	_, proof, err := s.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "one", Role: Admin})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "owner", Email: "owner@gmail.com", EmailAuthoritative: true}, proof)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO agenova_projects VALUES('team','two')`); err != nil {
		t.Fatal(err)
	}
	accept := func(name, project, role, account string) v0.Principal {
		t.Helper()
		i, proof, err := s.ReserveGoogleInvitation(ctx, owner, invitations.Request{Email: name + "@gmail.com", Project: project, Role: role, AccountRole: account})
		if err != nil {
			t.Fatal(err)
		}
		want := account
		if want == "" {
			want = MemberRole
		}
		if i.AccountRole != want {
			t.Fatal("response account role", i.AccountRole)
		}
		p, err := s.AcceptGoogleInvitation(ctx, googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: name, Email: name + "@gmail.com", EmailAuthoritative: true}, proof)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	pa := accept("projectadmin", "one", Admin, "")
	other := accept("other", "two", MemberRole, "")
	var account string
	if err = s.DB.QueryRow(`SELECT account_role FROM agenova_members WHERE principal=$1`, pa.Subject).Scan(&account); err != nil || account != MemberRole {
		t.Fatal("implicit account elevation", account, err)
	}
	if s.ChangeMember(ctx, pa, invitations.MemberChange{Principal: other.Subject, Action: "account-role", Role: Admin}) == nil {
		t.Fatal("project admin changed account role")
	}
	for _, r := range []invitations.Request{{Email: "a@gmail.com", Project: "two", Role: MemberRole}, {Email: "b@gmail.com", Project: "one", Role: MemberRole, AccountRole: Admin}, {Email: "c@gmail.com", Project: "one", Role: Admin}} {
		if _, _, err = s.ReserveGoogleInvitation(ctx, pa, r); err == nil {
			t.Fatal("project admin escaped scope", r)
		}
	}
	if _, _, err = s.ReserveGoogleInvitation(ctx, pa, invitations.Request{Email: "allowed@gmail.com", Project: "one", Role: Viewer}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListMembers(ctx, pa)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range rows {
		if m.Principal == other.Subject {
			t.Fatal("other project member disclosed")
		}
		for _, g := range m.Projects {
			if g.Project != "one" {
				t.Fatal("foreign project grant disclosed")
			}
		}
	}
	invites, err := s.ListInvitations(ctx, pa)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range invites {
		if i.Project != "one" {
			t.Fatal("foreign invitation disclosed")
		}
	}
	aa := accept("accountadmin", "one", Viewer, Admin)
	if s.Authorize(ctx, aa, SubmitWork, "one") == nil {
		t.Fatal("account admin bypassed viewer role")
	}
	if s.Authorize(ctx, pa, SubmitWork, "one") != nil {
		t.Fatal("project admin cannot run work")
	}
}

func TestOwnershipTransferGuardsConcurrencyAndAudit(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	for _, n := range []string{"owner", "first", "second", "member"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "issuer", Subject: n, Principal: n, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='owner'; UPDATE agenova_members SET account_role='admin' WHERE principal IN ('first','second')`); err != nil {
		t.Fatal(err)
	}
	p := func(n string) v0.Principal {
		return v0.Principal{Subject: n, Team: "team", AuthenticationContext: "oidc"}
	}
	transfer := func(actor, target, confirm string) error {
		return s.ChangeMember(ctx, p(actor), invitations.MemberChange{Principal: target, Action: "transfer-owner", Confirmation: confirm})
	}
	for _, c := range [][3]string{{"first", "second", "second"}, {"owner", "member", "member"}, {"owner", "first", ""}, {"owner", "owner", "owner"}} {
		if transfer(c[0], c[1], c[2]) == nil {
			t.Fatal("invalid ownership transfer", c)
		}
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET active=false WHERE principal='first'`); err != nil {
		t.Fatal(err)
	}
	if transfer("owner", "first", "first") == nil {
		t.Fatal("inactive recipient accepted")
	}
	if _, err := s.DB.Exec(`UPDATE agenova_members SET active=true WHERE principal='first'`); err != nil {
		t.Fatal(err)
	}
	// Audit failure must roll back both role writes.
	if _, err := s.DB.Exec(`CREATE FUNCTION reject_transfer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation='ownership.transfer' THEN RAISE EXCEPTION 'fixture'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_transfer BEFORE INSERT ON agenova_identity_audit FOR EACH ROW EXECUTE FUNCTION reject_transfer()`); err != nil {
		t.Fatal(err)
	}
	if transfer("owner", "first", "first") == nil {
		t.Fatal("audit failure accepted")
	}
	var role string
	if err := s.DB.QueryRow(`SELECT account_role FROM agenova_members WHERE principal='owner'`).Scan(&role); err != nil || role != Owner {
		t.Fatal("ownership lost on rollback", err)
	}
	if _, err := s.DB.Exec(`DROP TRIGGER reject_transfer ON agenova_identity_audit`); err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, target := range []string{"first", "second"} {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			if transfer("owner", target, target) == nil {
				wins.Add(1)
			}
		}(target)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("concurrent ownership winners", wins.Load())
	}
	var n int
	if err := s.DB.QueryRow(`SELECT count(*) FROM agenova_members WHERE account_role='owner' AND active`).Scan(&n); err != nil || n != 1 {
		t.Fatal("last owner invariant", n, err)
	}
	if err := s.DB.QueryRow(`SELECT principal FROM agenova_members WHERE account_role='owner'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	for _, change := range []invitations.MemberChange{{Principal: role, Action: "disable"}, {Principal: role, Action: "account-role", Role: MemberRole}} {
		if s.ChangeMember(ctx, p(role), change) == nil {
			t.Fatal("last owner removed")
		}
	}
	if err := s.DB.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE operation='ownership.transfer'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("transfer audit", n, err)
	}
}
