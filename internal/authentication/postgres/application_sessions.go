// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/authentication"
)

func applicationTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IssueApplicationSession is trusted server-side login completion, never a
// caller-authored API. The caller must consume a verified, bound login attempt
// first. Only active existing memberships qualify; this cannot accept an invite.
// Lifetime is absolute: refreshing cannot extend it past the original login.
func (s Store) IssueApplicationSession(ctx context.Context, issuer, subject string, lifetime time.Duration) (string, authentication.Identity, error) {
	denied := authentication.ErrUnauthenticated
	if s.DB == nil || issuer != "https://accounts.google.com" || subject == "" || lifetime <= 0 || lifetime > 12*time.Hour {
		return "", authentication.Identity{}, denied
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", authentication.Identity{}, denied
	}
	raw := "ags_" + base64.RawURLEncoding.EncodeToString(b[:])
	sessionID, err := inviteID()
	if err != nil {
		return "", authentication.Identity{}, denied
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", authentication.Identity{}, denied
	}
	defer tx.Rollback()
	var principal string
	// Serializes new sessions for a member, and races with disable/re-enable.
	if tx.QueryRowContext(ctx, `SELECT principal FROM agenova_members WHERE issuer=$1 AND subject=$2 AND active FOR UPDATE`, issuer, subject).Scan(&principal) != nil {
		return "", authentication.Identity{}, denied
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM agenova_application_sessions WHERE issuer=$1 AND subject=$2 AND expires_at<=clock_timestamp()`, issuer, subject); err != nil {
		return "", authentication.Identity{}, denied
	}
	var count int
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM agenova_application_sessions WHERE issuer=$1 AND subject=$2`, issuer, subject).Scan(&count) != nil || count >= 32 {
		return "", authentication.Identity{}, denied
	}
	id := authentication.Identity{Issuer: issuer, Subject: subject, TokenID: sessionID, SessionID: sessionID}
	if err = tx.QueryRowContext(ctx, `INSERT INTO agenova_application_sessions(token_hash,session_id,issuer,subject,authenticated_at,expires_at) VALUES($1,$2,$3,$4,clock_timestamp(),clock_timestamp()+$5*interval '1 second') RETURNING authenticated_at,expires_at`, applicationTokenHash(raw), sessionID, issuer, subject, lifetime.Seconds()).Scan(&id.AuthenticatedAt, &id.ExpiresAt); err != nil {
		return "", authentication.Identity{}, denied
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'session.created',$1)`, principal); err != nil {
		return "", authentication.Identity{}, denied
	}
	if tx.Commit() != nil {
		return "", authentication.Identity{}, denied
	}
	return raw, id, nil
}

// ApplicationSessions verifies Agenova credentials, never Google ID/access
// tokens. Every read checks durable revocation and membership; no JWT cache can
// revive a disabled member or a logged-out session.
type ApplicationSessions struct{ Store }

func (s ApplicationSessions) Verify(ctx context.Context, raw string) (authentication.Identity, error) {
	denied := authentication.ErrUnauthenticated
	if s.DB == nil || len(raw) != 47 || !strings.HasPrefix(raw, "ags_") {
		return authentication.Identity{}, denied
	}
	if b, err := base64.RawURLEncoding.Strict().DecodeString(raw[4:]); err != nil || len(b) != 32 {
		return authentication.Identity{}, denied
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var id authentication.Identity
	err := s.DB.QueryRowContext(ctx, `SELECT a.issuer,a.subject,a.session_id,a.authenticated_at,a.expires_at FROM agenova_application_sessions a JOIN agenova_members m ON m.issuer=a.issuer AND m.subject=a.subject WHERE a.token_hash=$1 AND a.expires_at>clock_timestamp() AND m.active AND a.authenticated_at>m.login_after AND NOT EXISTS (SELECT 1 FROM agenova_revocations r WHERE r.issuer=a.issuer AND r.subject=a.subject AND r.session_id=a.session_id)`, applicationTokenHash(raw)).Scan(&id.Issuer, &id.Subject, &id.SessionID, &id.AuthenticatedAt, &id.ExpiresAt)
	if err != nil {
		return authentication.Identity{}, denied
	}
	id.TokenID = id.SessionID
	return id, nil
}
