// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

func (s Store) ListMembers(ctx context.Context, p v0.Principal) ([]invitations.MemberView, error) {
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
	if err = invitationActor(ctx, tx, p, MemberRole); err != nil {
		return nil, err
	}
	// Team-qualified SQL; never return raw IdP subjects, session records or tokens.
	rows, err := tx.QueryContext(ctx, `SELECT m.principal,COALESCE(i.email,''),m.account_role,m.active,COALESCE(g.project,''),COALESCE(g.role,'') FROM agenova_members m LEFT JOIN agenova_invitations i ON i.principal=m.principal AND i.team=m.team AND i.status='accepted' LEFT JOIN agenova_project_grants g ON g.principal=m.principal AND g.team=m.team WHERE m.team=$1 ORDER BY m.principal,g.project`, p.Team)
	if err != nil {
		return nil, invitations.ErrUnavailable
	}
	defer rows.Close()
	result := []invitations.MemberView{}
	for rows.Next() {
		var m invitations.MemberView
		var g invitations.ProjectGrant
		if rows.Scan(&m.Principal, &m.Email, &m.AccountRole, &m.Active, &g.Project, &g.Role) != nil {
			return nil, invitations.ErrUnavailable
		}
		if len(result) == 0 || result[len(result)-1].Principal != m.Principal {
			m.Projects = []invitations.ProjectGrant{}
			result = append(result, m)
		}
		if g.Project != "" {
			result[len(result)-1].Projects = append(result[len(result)-1].Projects, g)
		}
	}
	if rows.Err() != nil {
		return nil, invitations.ErrUnavailable
	}
	rows.Close()
	for i := range result {
		grants, err := tx.QueryContext(ctx, `SELECT role,scope_kind,scope_ref FROM agenova_scoped_roles WHERE principal=$1 AND team=$2 ORDER BY role,scope_kind,scope_ref`, result[i].Principal, p.Team)
		if err != nil {
			return nil, invitations.ErrUnavailable
		}
		result[i].Roles = []invitations.ScopedRole{}
		for grants.Next() {
			var g invitations.ScopedRole
			if grants.Scan(&g.Role, &g.ScopeKind, &g.ScopeRef) != nil {
				grants.Close()
				return nil, invitations.ErrUnavailable
			}
			result[i].Roles = append(result[i].Roles, g)
		}
		err = grants.Err()
		grants.Close()
		if err != nil {
			return nil, invitations.ErrUnavailable
		}
	}
	return result, nil
}

// ChangeMember serializes role/activation changes with grant edits and session
// issuance. Owner is immutable here; ownership transfer is not a self-service path.
func (s Store) ChangeMember(ctx context.Context, p v0.Principal, c invitations.MemberChange) error {
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return authentication.ErrForbidden
	}
	if c.Principal == "" || len(c.Principal) > 256 || len(c.Project) > 128 {
		return invitations.ErrInvalid
	}
	if c.Action == "grant-role" || c.Action == "revoke-role" {
		return s.ChangeScopedRole(ctx, p, c)
	}
	if c.ScopeKind != "" || c.ScopeRef != "" {
		return invitations.ErrInvalid
	}
	if c.Action == "project-role" {
		if c.Project == "" || (c.Role != "" && c.Role != Admin && c.Role != MemberRole && c.Role != Viewer) {
			return invitations.ErrInvalid
		}
		return s.GrantProject(ctx, p, c.Principal, c.Project, c.Role)
	}
	if c.Project != "" {
		return invitations.ErrInvalid
	}
	switch c.Action {
	case "enable", "disable":
		if c.Role != "" {
			return invitations.ErrInvalid
		}
	case "account-role":
		if c.Role != Admin && c.Role != MemberRole {
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
	actor, target, err := lockMemberPair(ctx, tx, p, c.Principal)
	if err != nil {
		return err
	}
	if target == Owner || actor != Owner && (actor != Admin || target == Admin || c.Action == "account-role") {
		return authentication.ErrForbidden
	}
	if actor != Owner {
		var privileged bool
		if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_scoped_roles WHERE principal=$1 AND team=$2)`, c.Principal, p.Team).Scan(&privileged) != nil {
			return invitations.ErrUnavailable
		}
		if privileged {
			return authentication.ErrForbidden
		}
		var scoped bool
		// An account Admin may not disable a person with authority in projects
		// that the Admin does not administer, or any existing project Admin.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_project_grants WHERE principal=$1 AND team=$2) AND NOT EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=$1 AND g.team=$2 AND (g.role='admin' OR NOT EXISTS(SELECT 1 FROM agenova_project_grants a WHERE a.principal=$3 AND a.team=g.team AND a.project=g.project AND a.role='admin')))`, c.Principal, p.Team, p.Subject).Scan(&scoped)
		if err != nil || !scoped {
			return authentication.ErrForbidden
		}
	}
	switch c.Action {
	case "account-role":
		_, err = tx.ExecContext(ctx, `UPDATE agenova_members SET account_role=$3 WHERE principal=$1 AND team=$2`, c.Principal, p.Team, c.Role)
	case "disable":
		_, err = tx.ExecContext(ctx, `UPDATE agenova_members SET active=false,login_after=now() WHERE principal=$1 AND team=$2`, c.Principal, p.Team)
	case "enable":
		_, err = tx.ExecContext(ctx, `UPDATE agenova_members SET active=true WHERE principal=$1 AND team=$2`, c.Principal, p.Team)
	}
	if err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, p.Subject, "member."+c.Action+"."+c.Role, c.Principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}
func lockMemberPair(ctx context.Context, tx *sql.Tx, p v0.Principal, target string) (string, string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT principal,account_role,active FROM agenova_members WHERE team=$1 ORDER BY principal FOR UPDATE`, p.Team)
	if err != nil {
		return "", "", invitations.ErrUnavailable
	}
	defer rows.Close()
	var actorRole, targetRole string
	for rows.Next() {
		var principal, role string
		var active bool
		if rows.Scan(&principal, &role, &active) != nil {
			return "", "", invitations.ErrUnavailable
		}
		if principal == p.Subject && active {
			actorRole = role
		}
		if principal == target {
			targetRole = role
		}
	}
	if rows.Err() != nil {
		return "", "", invitations.ErrUnavailable
	}
	if actorRole == "" || targetRole == "" {
		return "", "", authentication.ErrForbidden
	}
	return actorRole, targetRole, nil
}
