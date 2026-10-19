// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/authentication"
)

func TestApplicationSessionPersistenceRevocationAndDisable(t *testing.T) {
	db := invitationDB(t)
	s := Store{db}
	ctx := context.Background()
	m := Member{Issuer: "https://accounts.google.com", Subject: "google-sub", Principal: "member", Team: "team", Active: true}
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	raw, id, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh verifier/store simulates application restart, no process cache.
	v := ApplicationSessions{Store{db}}
	restored, err := v.Verify(ctx, raw)
	if err != nil || restored.SessionID != id.SessionID || restored.Subject != m.Subject {
		t.Fatal("durable session lookup", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT token_hash FROM agenova_application_sessions`).Scan(&stored); err != nil || stored == raw || strings.Contains(stored, raw[4:]) {
		t.Fatal("raw credential persisted")
	}
	if err := s.Revoke(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, raw); err == nil {
		t.Fatal("logout did not revoke")
	}
	raw, _, err = s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	m.Active = false
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, raw); err == nil {
		t.Fatal("disabled member retained session")
	}
	if _, _, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour); err == nil {
		t.Fatal("disabled member obtained session")
	}
	m.Active = true
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, raw); err == nil {
		t.Fatal("re-enable resurrected old login")
	}
	newRaw, _, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, newRaw); err != nil {
		t.Fatal("fresh login rejected", err)
	}
	if _, err := db.Exec(`UPDATE agenova_application_sessions SET authenticated_at=now()-interval '2 hours',expires_at=now()-interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, newRaw); err == nil {
		t.Fatal("expired credential accepted")
	}
	for _, raw := range []string{"google.jwt.token", "Bearer other", "ags_" + strings.Repeat("!", 43), ""} {
		if _, err := v.Verify(ctx, raw); err == nil {
			t.Fatal("foreign credential accepted")
		}
	}
}

func TestApplicationSessionLimits(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	if err := s.PutMember(ctx, "operator", Member{Issuer: "https://accounts.google.com", Subject: "sub", Principal: "member", Team: "team", Active: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		if _, _, err := s.IssueApplicationSession(ctx, "https://accounts.google.com", "sub", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.IssueApplicationSession(ctx, "https://accounts.google.com", "sub", time.Hour); err == nil {
		t.Fatal("session cap ignored")
	}
	if _, _, err := s.IssueApplicationSession(ctx, "https://accounts.google.com", "sub", 13*time.Hour); err == nil {
		t.Fatal("lifetime cap ignored")
	}
	if _, _, err := s.IssueApplicationSession(ctx, "https://accounts.google.com", "uninvited", time.Hour); err == nil {
		t.Fatal("uninvited identity issued session")
	}
	var _ authentication.Verifier = ApplicationSessions{}
}

func TestApplicationSessionInvalidatedLoginsDoNotConsumeCapacity(t *testing.T) {
	s := Store{invitationDB(t)}
	ctx := context.Background()
	m := Member{Issuer: "https://accounts.google.com", Subject: "sub", Principal: "member", Team: "team", Active: true}
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	verifier := ApplicationSessions{s}
	for i := 0; i < 40; i++ {
		raw, id, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
		if err != nil {
			t.Fatalf("login %d after logout: %v", i, err)
		}
		if err := s.Revoke(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := verifier.Verify(ctx, raw); err == nil {
			t.Fatal("revoked token revived")
		}
	}
	var revoked int
	if err := s.DB.QueryRow(`SELECT count(*) FROM agenova_revocations`).Scan(&revoked); err != nil || revoked != 40 {
		t.Fatalf("revocation tombstones lost: %d %v", revoked, err)
	}
	var old string
	for i := 0; i < 32; i++ {
		raw, _, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		old = raw
	}
	if _, _, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour); err == nil {
		t.Fatal("active session cap ignored")
	}
	m.Active = false
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	m.Active = true
	if err := s.PutMember(ctx, "operator", m); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := s.IssueApplicationSession(ctx, m.Issuer, m.Subject, time.Hour)
	if err != nil {
		t.Fatal("invalidated sessions exhausted fresh login capacity", err)
	}
	if _, err := verifier.Verify(ctx, old); err == nil {
		t.Fatal("re-enable revived old token")
	}
	if _, err := verifier.Verify(ctx, fresh); err != nil {
		t.Fatal("fresh login rejected", err)
	}
}
