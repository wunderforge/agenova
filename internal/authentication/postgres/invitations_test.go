// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

func invitationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	suffix, err := inviteID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "invite_" + suffix
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	if err = Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

type inviteDirectory struct {
	calls   atomic.Int32
	fail    bool
	subject string
}

func (d *inviteDirectory) Create(context.Context, string) (string, error) {
	d.calls.Add(1)
	return d.subject, nil
}
func (d *inviteDirectory) Send(context.Context, string) error {
	d.calls.Add(1)
	if d.fail {
		return errors.New("unconfirmed")
	}
	return nil
}
func TestInvitationAuthorityActivationAndReplay(t *testing.T) {
	db := invitationDB(t)
	ctx := context.Background()
	s := Store{db}
	for _, role := range []string{Owner, Admin, MemberRole} {
		if err := s.PutMember(ctx, "operator", Member{Issuer: "issuer", Subject: role, Principal: role, Team: "team", Active: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE agenova_members SET account_role=$1 WHERE principal=$1`, role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team','payments')`); err != nil {
		t.Fatal(err)
	}
	principal := func(role string) v0.Principal {
		return v0.Principal{Subject: role, Team: "team", AuthenticationContext: "oidc"}
	}
	if err := s.GrantProject(ctx, principal(Owner), Admin, "payments", Admin); err != nil {
		t.Fatal(err)
	}
	directory := &inviteDirectory{subject: "new-subject"}
	service := invitations.Service{Issuer: "issuer", Directory: directory, Store: s}
	request := invitations.Request{Email: "invited@example.com", Project: "payments", Role: Admin}
	for _, p := range []v0.Principal{principal(MemberRole), {Subject: Owner, Team: "foreign", AuthenticationContext: "oidc"}} {
		if _, err := service.Invite(ctx, p, request); err == nil {
			t.Fatal("unauthorized invitation allowed")
		}
	}
	if directory.calls.Load() != 0 {
		t.Fatal("denial called directory")
	}
	invite, err := service.Invite(ctx, principal(Owner), request)
	if err != nil {
		t.Fatal(err)
	}
	if invite.Status != "sent" || directory.calls.Load() != 2 {
		t.Fatal("invite delivery state")
	}
	if _, err = service.Invite(ctx, principal(Owner), request); err == nil {
		t.Fatal("duplicate invitation accepted")
	}
	if directory.calls.Load() != 2 {
		t.Fatal("duplicate sent another email")
	}
	var active bool
	if err = db.QueryRow(`SELECT active FROM agenova_members WHERE subject='new-subject'`).Scan(&active); err != nil || active {
		t.Fatal("pending invite activated before login")
	}
	identity := authentication.Identity{Issuer: "issuer", Subject: "new-subject", TokenID: "jti", SessionID: "grant", AuthenticatedAt: databaseAuthTime(t, db), ExpiresAt: time.Now().Add(time.Hour)}
	resolver := InvitedMemberships{s}
	p, err := resolver.Resolve(ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != invite.Principal || s.Authorize(ctx, p, SubmitWork, "payments") != nil {
		t.Fatal("invited admin cannot submit")
	}
	if _, err = resolver.Resolve(ctx, identity); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agenova_identity_audit WHERE operation='invitation.accepted'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("invitation acceptance replayed")
	}
	if err = s.PutMember(ctx, Owner, Member{Issuer: identity.Issuer, Subject: identity.Subject, Principal: p.Subject, Team: p.Team, Active: false}); err != nil {
		t.Fatal(err)
	}
	if _, err = resolver.Resolve(ctx, identity); err == nil {
		t.Fatal("invitation replay re-enabled disabled member")
	}
	// Mail uncertainty grants no authority even if a valid token is later obtained.
	directory.subject = "uncertain-subject"
	directory.fail = true
	request.Email = "uncertain@example.com"
	request.Role = MemberRole
	if _, err = service.Invite(ctx, principal(Admin), request); err == nil {
		t.Fatal("uncertain delivery reported success")
	}
	identity.Subject = directory.subject
	if _, err = resolver.Resolve(ctx, identity); err == nil {
		t.Fatal("uncertain invitation activated")
	}
	// Expiration is enforced by the DB even with cryptographically valid identity.
	directory.subject = "expired-subject"
	directory.fail = false
	request.Email = "expired@example.com"
	expired, err := service.Invite(ctx, principal(Admin), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agenova_invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	identity.Subject = directory.subject
	if _, err = resolver.Resolve(ctx, identity); err == nil {
		t.Fatal("expired invitation activated")
	}
	if _, err = s.ListInvitations(ctx, principal(MemberRole)); err == nil {
		t.Fatal("member read invitation emails")
	}
	if list, err := s.ListInvitations(ctx, principal(Owner)); err != nil || len(list) != 3 {
		t.Fatalf("invite list: %d %v", len(list), err)
	}
}

func TestOwnerBootstrapIsOneTimeAndSubjectBound(t *testing.T) {
	db := invitationDB(t)
	ctx := context.Background()
	s := Store{db}
	directory := &inviteDirectory{subject: "owner-sub"}
	service := invitations.Service{Issuer: "issuer", Directory: directory, Store: s}
	r := invitations.Request{Email: "owner@example.com", Project: "payments", Role: Admin}
	invite, err := service.BootstrapOwner(ctx, "team", r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.BootstrapOwner(ctx, "team", r); err == nil {
		t.Fatal("second Owner bootstrap accepted")
	}
	if directory.calls.Load() != 2 {
		t.Fatal("bootstrap duplicate reached external service")
	}
	var active bool
	var role string
	if err = db.QueryRow(`SELECT active,account_role FROM agenova_members WHERE principal=$1`, invite.Principal).Scan(&active, &role); err != nil || active || role != Owner {
		t.Fatal("Owner activated prematurely")
	}
	id := authentication.Identity{Issuer: "issuer", Subject: "wrong", SessionID: "grant", TokenID: "jti", AuthenticatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	resolver := InvitedMemberships{s}
	if _, err = resolver.Resolve(ctx, id); err == nil {
		t.Fatal("wrong subject accepted Owner invitation")
	}
	id.Subject = "owner-sub"
	p, err := resolver.Resolve(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PutMember(ctx, "operator", Member{Issuer: id.Issuer, Subject: id.Subject, Principal: p.Subject, Team: p.Team, Active: false}); err == nil {
		t.Fatal("Owner disabled by generic membership update")
	}
	directory.subject = "admin-sub"
	r.Email = "admin@example.com"
	if _, err = service.Invite(ctx, p, r); err != nil {
		t.Fatal("Owner could not invite Admin:", err)
	}
}
