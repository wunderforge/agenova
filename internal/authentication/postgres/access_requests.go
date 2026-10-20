// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
	"time"
)

// RegisterGoogleAccess accepts only a callback-verified identity. Team is pinned
// by operator configuration, not Google claims or browser/native request input.
// Registration creates no membership, grants or application credentials.
func (s Store) RegisterGoogleAccess(ctx context.Context, id googleoidc.LoginIdentity, team string) (string, error) {
	if id.Issuer != googleoidc.Issuer || id.Subject == "" || len(id.Subject) > 256 || !id.EmailAuthoritative || len(id.Email) > 254 || id.Email == "" || team == "" || len(team) > 128 {
		return "", authentication.ErrForbidden
	}
	if s.DB == nil {
		return "", invitations.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", invitations.ErrUnavailable
	}
	defer tx.Rollback()
	// Serialize registration capacity and duplicate login without locking member
	// rows in the inverse order of administrative role transactions.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215008)`); err != nil {
		return "", invitations.ErrUnavailable
	}
	var active bool
	var memberTeam string
	err = tx.QueryRowContext(ctx, `SELECT active,team FROM agenova_members WHERE issuer=$1 AND subject=$2`, id.Issuer, id.Subject).Scan(&active, &memberTeam)
	if err == nil {
		if active && memberTeam == team {
			return "active", nil
		}
		return "", authentication.ErrForbidden
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", invitations.ErrUnavailable
	}
	var status, existingTeam string
	err = tx.QueryRowContext(ctx, `SELECT status,team FROM agenova_access_requests WHERE issuer=$1 AND subject=$2`, id.Issuer, id.Subject).Scan(&status, &existingTeam)
	if err == nil {
		if existingTeam != team {
			return "", authentication.ErrForbidden
		}
		return status, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", invitations.ErrUnavailable
	}
	var count int
	var initialized bool
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_members WHERE team=$1 AND active AND account_role='owner'),(SELECT count(*) FROM agenova_access_requests WHERE team=$1)`, team).Scan(&initialized, &count) != nil || !initialized || count >= 10000 {
		return "", invitations.ErrUnavailable
	}
	var b [16]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", invitations.ErrUnavailable
	}
	key := hex.EncodeToString(b[:])
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_access_requests(id,issuer,subject,email,team) VALUES($1,$2,$3,$4,$5)`, key, id.Issuer, id.Subject, id.Email, team); err != nil {
		return "", invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'access.requested',$1)`, "request:"+key); err != nil {
		return "", invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return "", invitations.ErrUnavailable
	}
	return "pending", nil
}
func accessReviewer(ctx context.Context, tx *sql.Tx, p v0.Principal) (string, error) {
	if p.AuthenticationContext != "oidc" || p.Team == "" || p.Subject == "" {
		return "", authentication.ErrForbidden
	}
	role, _, err := lockMemberPair(ctx, tx, p, p.Subject)
	if err != nil {
		return "", err
	}
	if role != Owner && role != Admin {
		return "", authentication.ErrForbidden
	}
	return role, nil
}
func (s Store) ListAccessRequests(ctx context.Context, p v0.Principal) ([]invitations.AccessRequest, error) {
	if s.DB == nil {
		return nil, invitations.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, invitations.ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = accessReviewer(ctx, tx, p); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,email,status FROM agenova_access_requests WHERE team=$1 AND status='pending' ORDER BY created_at,id LIMIT 10000`, p.Team)
	if err != nil {
		return nil, invitations.ErrUnavailable
	}
	defer rows.Close()
	out := []invitations.AccessRequest{}
	for rows.Next() {
		var a invitations.AccessRequest
		if rows.Scan(&a.ID, &a.Email, &a.Status) != nil {
			return nil, invitations.ErrUnavailable
		}
		out = append(out, a)
	}
	if rows.Err() != nil {
		return nil, invitations.ErrUnavailable
	}
	return out, nil
}
func (s Store) ReviewAccessRequest(ctx context.Context, p v0.Principal, c invitations.AccessReview) error {
	if c.ID == "" || len(c.ID) > 128 || len(c.Project) > 128 {
		return invitations.ErrInvalid
	}
	if c.PlatformRole != "" {
		if c.Action != "approve" || c.AccountRole != "" || c.Role != "" || c.Installation == "" {
			return invitations.ErrInvalid
		}
		switch c.PlatformRole {
		case PlatformAdmin:
			c.AccountRole, c.Role = Admin, Admin
		case PlatformOperator, MemberRole:
			c.AccountRole, c.Role = MemberRole, MemberRole
		default:
			return invitations.ErrInvalid
		}
	}
	switch c.Action {
	case "approve":
		if c.AccountRole != MemberRole && c.AccountRole != Admin {
			return invitations.ErrInvalid
		}
		if c.Project == "" || (c.Role != MemberRole && c.Role != Viewer && c.Role != Admin) {
			return invitations.ErrInvalid
		}
	case "reject":
		if c.AccountRole != "" || c.Project != "" || c.Role != "" {
			return invitations.ErrInvalid
		}
	default:
		return invitations.ErrInvalid
	}
	if s.DB == nil {
		return invitations.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.ErrUnavailable
	}
	defer tx.Rollback()
	actor, err := accessReviewer(ctx, tx, p)
	if err != nil {
		return err
	}
	if c.Action == "approve" {
		if actor != Owner && c.PlatformRole == "" && (c.AccountRole == Admin || c.Role == Admin) {
			return authentication.ErrForbidden
		}
		var permitted bool
		if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_projects WHERE team=$1 AND project=$2) AND ($3 OR EXISTS(SELECT 1 FROM agenova_project_grants WHERE principal=$4 AND team=$1 AND project=$2 AND role='admin'))`, p.Team, c.Project, actor == Owner, p.Subject).Scan(&permitted) != nil || !permitted {
			return authentication.ErrForbidden
		}
	}
	var issuer, subject string
	if tx.QueryRowContext(ctx, `SELECT issuer,subject FROM agenova_access_requests WHERE id=$1 AND team=$2 AND status='pending' FOR UPDATE`, c.ID, p.Team).Scan(&issuer, &subject) != nil {
		return authentication.ErrForbidden
	}
	principal := "user:request:" + c.ID
	status := "rejected"
	if c.Action == "approve" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_members(issuer,subject,principal,team,active,account_role) VALUES($1,$2,$3,$4,true,$5)`, issuer, subject, principal, p.Team, c.AccountRole); err != nil {
			return invitations.ErrUnavailable
		}
		if c.PlatformRole == PlatformOperator {
			if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_scoped_roles(principal,team,role,scope_kind,scope_ref) VALUES($1,$2,'platform-operator','installation',$3)`, principal, p.Team, c.Installation); err != nil {
				return invitations.ErrUnavailable
			}
		} else if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_project_grants(principal,team,project,role) VALUES($1,$2,$3,$4)`, principal, p.Team, c.Project, c.Role); err != nil {
			return invitations.ErrUnavailable
		}
		status = "approved"
	}
	var bound any
	if status == "approved" {
		bound = principal
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_access_requests SET status=$2,principal=$3,reviewed_at=now() WHERE id=$1`, c.ID, status, bound); err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, p.Subject, "access."+status+".platform."+c.PlatformRole+".account."+c.AccountRole+".project."+c.Project+".role."+c.Role, principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}
