// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

// Explicit disposable PostgreSQL only. The test never drops an existing schema.
func TestPostgresMembershipAndGrantRevocation(t *testing.T) {
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("set AGENOVA_TEST_POSTGRES to a disposable database for real PostgreSQL evidence")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	s := Store{db}
	suffix := time.Now().UTC().Format("150405.000000000")
	m := Member{Issuer: "https://issuer.example/" + suffix, Subject: "alice", Principal: "member-" + suffix, Team: "payments", Active: true}
	id := authentication.Identity{Issuer: m.Issuer, Subject: m.Subject, SessionID: "grant-1", TokenID: "token-1", ExpiresAt: time.Now().Add(time.Hour), AuthenticatedAt: time.Now().Add(-time.Minute)}
	if _, err := s.Resolve(ctx, id); err == nil {
		t.Fatal("uninvited user allowed")
	}
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	p, err := s.Resolve(ctx, id)
	if err != nil || p.Subject != m.Principal || p.Team != m.Team {
		t.Fatalf("mapping: %+v %v", p, err)
	}
	other := m
	other.Team = "other"
	if err := s.PutMember(ctx, "operator", other); err == nil {
		t.Fatal("identity was reassigned")
	}
	m.Active = false
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, id); err == nil {
		t.Fatal("disabled membership allowed")
	}
	m.Active = true
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, id); err == nil {
		t.Fatal("re-enable revived the old login")
	}
	id.AuthenticatedAt = databaseAuthTime(t, db)
	if _, err := s.Resolve(ctx, id); err != nil {
		t.Fatal("fresh login denied after re-enable")
	}
	if err := s.Revoke(ctx, id); err != nil {
		t.Fatal(err)
	}
	// Another process/connection must observe persisted tombstones, including a
	// refreshed access token with a new jti but the same origin_jti.
	db2, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	restarted := Store{db2}
	id.TokenID = "refreshed-token"
	if _, err := restarted.Resolve(ctx, id); err == nil {
		t.Fatal("refresh revived revoked grant")
	}
	id.SessionID = "new-login"
	if _, err := restarted.Resolve(ctx, id); err != nil {
		t.Fatal("new login incorrectly revoked")
	}
	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Resolve(ctx, id); err == nil {
		t.Fatal("database outage allowed membership")
	}
	var audits int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_identity_audit WHERE principal=$1", m.Principal).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("audit atomicity: %d %v", audits, err)
	}
}

// Synthetic identities use the DB clock after commit, avoiding host/VM skew.
func databaseAuthTime(t *testing.T, db *sql.DB) time.Time {
	t.Helper()
	var now time.Time
	if err := db.QueryRowContext(context.Background(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal("cannot obtain synthetic identity fixture timestamp")
	}
	return now
}

func TestProjectRolesCannotEscalateOrCrossTeams(t *testing.T) {
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	s := Store{db}
	team := "roles-" + time.Now().UTC().Format("150405.000000000")
	for _, name := range []string{"owner", "admin", "member", "viewer"} {
		if err := s.PutMember(ctx, "bootstrap", Member{Issuer: "https://roles.example", Subject: team + name, Principal: team + name, Team: team, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	// Operator bootstrap, intentionally not exposed as a member-facing API.
	if _, err := db.ExecContext(ctx, `UPDATE agenova_members SET account_role='owner' WHERE principal=$1`, team+"owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agenova_projects(team,project) VALUES($1,'payments')`, team); err != nil {
		t.Fatal(err)
	}
	principal := func(name string) v0.Principal {
		return v0.Principal{Subject: team + name, Team: team, AuthenticationContext: "oidc"}
	}
	for _, name := range []string{"admin", "member", "viewer"} {
		if err := s.GrantProject(ctx, principal("owner"), team+name, "payments", name); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, action string
		allow        bool
	}{{"owner", ManageMembers, true}, {"admin", SubmitWork, true}, {"admin", ReadProjectWork, true}, {"member", SubmitWork, true}, {"member", ReadOwnWork, true}, {"member", ReadProjectWork, false}, {"member", ManageMembers, false}, {"viewer", SubmitWork, false}, {"viewer", ReadProjectWork, true}, {"owner", "platform.apply", false}} {
		err := s.Authorize(ctx, principal(tc.name), tc.action, "payments")
		if (err == nil) != tc.allow {
			t.Fatalf("%s %s: %v", tc.name, tc.action, err)
		}
	}
	if err := s.GrantProject(ctx, principal("admin"), team+"member", "payments", Admin); err == nil {
		t.Fatal("admin promoted another admin")
	}
	if err := s.GrantProject(ctx, principal("member"), team+"viewer", "payments", MemberRole); err == nil {
		t.Fatal("member changed grants")
	}
	forged := principal("owner")
	forged.Team = "foreign"
	if err := s.Authorize(ctx, forged, SubmitWork, "payments"); err == nil {
		t.Fatal("cross-team authorization")
	}
	if err := s.GrantProject(ctx, principal("owner"), team+"member", "payments", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(ctx, principal("member"), SubmitWork, "payments"); err == nil {
		t.Fatal("revoked project still allowed")
	}
}
