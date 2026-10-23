// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

const (
	Owner           = "owner"
	Admin           = "admin"
	MemberRole      = "member"
	Viewer          = "viewer"
	SubmitWork      = "work.submit"
	ReadOwnWork     = "work.read-own"
	ReadProjectWork = "work.read-project"
	ManageMembers   = "members.manage"
)

// Authorize is an application access check, never a claim authority grant.
// Model/tools/runtime ceilings must still pass the existing admission resolver.
func (s Store) Authorize(ctx context.Context, p v0.Principal, action, project string) error {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" || project == "" {
		return authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account, role string
	err := s.DB.QueryRowContext(ctx, `SELECT m.account_role,COALESCE(g.role,'') FROM agenova_members m
 JOIN agenova_projects p ON p.team=m.team AND p.project=$3
 LEFT JOIN agenova_project_grants g ON g.principal=m.principal AND g.team=m.team AND g.project=p.project
 WHERE m.principal=$1 AND m.team=$2 AND m.active`, p.Subject, p.Team, project).Scan(&account, &role)
	if err != nil {
		return authentication.ErrForbidden
	}
	allowed := false
	switch action {
	case SubmitWork, ReadOwnWork:
		allowed = account == Owner || account == Admin || role == Admin || role == MemberRole
	case ReadProjectWork:
		allowed = account == Owner || account == Admin || role == Admin || role == Viewer
	case ManageMembers:
		allowed = account == Owner || account == Admin || role == Admin
	}
	if !allowed {
		return authentication.ErrForbidden
	}
	return nil
}

// GrantProject serializes administrative edits and records their actor. Project
// admins may grant Member/Viewer only; granting Admin requires the account Owner.
// No method here writes cloud IAM/RBAC or changes the governance PolicyBundle.
func (s Store) GrantProject(ctx context.Context, actor v0.Principal, target, project, role string) error {
	if s.DB == nil || actor.AuthenticationContext != "oidc" || target == "" || project == "" || (role != Admin && role != MemberRole && role != Viewer && role != "") {
		return authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("membership transaction unavailable")
	}
	defer tx.Rollback()
	// Lock the team membership rows in a stable order. Disabling/promoting the
	// actor cannot race this grant and commit unnoticed.
	rows, err := tx.QueryContext(ctx, `SELECT principal,account_role,active FROM agenova_members WHERE team=$1 ORDER BY principal FOR UPDATE`, actor.Team)
	if err != nil {
		return errors.New("membership transaction unavailable")
	}
	var actorRole, targetRole string
	var targetActive bool
	for rows.Next() {
		var id, r string
		var active bool
		if err := rows.Scan(&id, &r, &active); err != nil {
			rows.Close()
			return errors.New("membership transaction unavailable")
		}
		if id == actor.Subject && active {
			actorRole = r
		}
		if id == target {
			targetRole = r
			targetActive = active
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.New("membership transaction unavailable")
	}
	if !targetActive || targetRole == Owner || actorRole == "" {
		return authentication.ErrForbidden
	}
	var projectAdmin bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_project_grants WHERE principal=$1 AND team=$2 AND project=$3 AND role='admin')`, actor.Subject, actor.Team, project).Scan(&projectAdmin); err != nil {
		return errors.New("membership transaction unavailable")
	}
	if actorRole != Owner && (role == Admin || actorRole != Admin && !projectAdmin) {
		return authentication.ErrForbidden
	}
	var old string
	// Prevent ordinary admins demoting an existing project admin.
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT role FROM agenova_project_grants WHERE principal=$1 AND project=$2),'')`, target, project).Scan(&old); err != nil {
		return errors.New("membership transaction unavailable")
	}
	if actorRole != Owner && old == Admin {
		return authentication.ErrForbidden
	}
	if role == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM agenova_project_grants WHERE principal=$1 AND project=$2`, target, project)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO agenova_project_grants(principal,team,project,role) VALUES($1,$2,$3,$4) ON CONFLICT(principal,project) DO UPDATE SET role=EXCLUDED.role WHERE agenova_project_grants.team=EXCLUDED.team`, target, actor.Team, project, role)
	}
	if err != nil {
		return errors.New("project grant rejected")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, actor.Subject, "project."+project+".role."+role, target); err != nil {
		return errors.New("membership audit unavailable")
	}
	if err = tx.Commit(); err != nil {
		return errors.New("membership commit unavailable")
	}
	return nil
}
