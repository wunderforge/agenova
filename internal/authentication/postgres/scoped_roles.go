// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

const GovernanceAdmin = "governance-admin"
const PlatformOperator = "platform-operator"

// ChangeScopedRole assigns additive duties. Only the active account Owner can
// delegate them. It never writes IAM/RBAC, PolicyBundle or claim authority.
func (s Store) ChangeScopedRole(ctx context.Context, actor v0.Principal, c invitations.MemberChange) error {
	if s.DB == nil || actor.AuthenticationContext != "oidc" || actor.Team == "" || actor.Subject == "" {
		return authentication.ErrForbidden
	}
	valid := c.Role == GovernanceAdmin && (c.ScopeKind == "account" || c.ScopeKind == "project") || c.Role == PlatformOperator && c.ScopeKind == "installation"
	if !valid || c.Principal == "" || c.Project != "" || c.ScopeRef == "" || len(c.ScopeRef) > 128 || strings.TrimSpace(c.ScopeRef) != c.ScopeRef || strings.ContainsAny(c.ScopeRef, "\x00\r\n") || (c.Action != "grant-role" && c.Action != "revoke-role") {
		return invitations.ErrInvalid
	}
	if c.ScopeKind == "account" && c.ScopeRef != actor.Team {
		return authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.ErrUnavailable
	}
	defer tx.Rollback()
	// Stable lock order serializes against membership removal and role changes.
	rows, err := tx.QueryContext(ctx, `SELECT principal,account_role,active FROM agenova_members WHERE team=$1 ORDER BY principal FOR UPDATE`, actor.Team)
	if err != nil {
		return invitations.ErrUnavailable
	}
	owner, target := false, false
	for rows.Next() {
		var id, role string
		var active bool
		if rows.Scan(&id, &role, &active) != nil {
			rows.Close()
			return invitations.ErrUnavailable
		}
		if id == actor.Subject {
			owner = active && role == Owner
		}
		if id == c.Principal {
			target = active
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return invitations.ErrUnavailable
	}
	if !owner || !target {
		return authentication.ErrForbidden
	}
	if c.ScopeKind == "project" {
		var exists bool
		if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_projects WHERE team=$1 AND project=$2)`, actor.Team, c.ScopeRef).Scan(&exists) != nil {
			return invitations.ErrUnavailable
		}
		if !exists {
			return authentication.ErrForbidden
		}
	}
	if c.Action == "grant-role" {
		_, err = tx.ExecContext(ctx, `INSERT INTO agenova_scoped_roles(principal,team,role,scope_kind,scope_ref) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, c.Principal, actor.Team, c.Role, c.ScopeKind, c.ScopeRef)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM agenova_scoped_roles WHERE principal=$1 AND team=$2 AND role=$3 AND scope_kind=$4 AND scope_ref=$5`, c.Principal, actor.Team, c.Role, c.ScopeKind, c.ScopeRef)
	}
	if err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, actor.Subject, c.Action+":"+c.Role+":"+c.ScopeKind+":"+c.ScopeRef, c.Principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}

// AuthorizeScoped checks explicit duties, never implicit Owner/admin privilege.
// A successful platform check is not cluster authorization. IAM/RBAC must still
// independently authorize deployment. Unknown operations always fail closed.
func (s Store) AuthorizeScoped(ctx context.Context, p v0.Principal, action, kind, ref string) error {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" || ref == "" {
		return authentication.ErrForbidden
	}
	role := ""
	switch action {
	case "policy.register", "template.register":
		if kind != "account" && kind != "project" {
			return authentication.ErrForbidden
		}
		role = GovernanceAdmin
	case "platform.configure", "platform.diagnose", "platform.recover":
		if kind != "installation" {
			return authentication.ErrForbidden
		}
		role = PlatformOperator
	default:
		return authentication.ErrForbidden
	}
	if kind == "account" && ref != p.Team {
		return authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var allowed bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_members m JOIN agenova_scoped_roles g ON g.principal=m.principal AND g.team=m.team WHERE m.principal=$1 AND m.team=$2 AND m.active AND g.role=$3 AND ((g.scope_kind=$4 AND g.scope_ref=$5) OR ($4='project' AND g.scope_kind='account' AND g.scope_ref=$2)) AND ($4<>'project' OR EXISTS(SELECT 1 FROM agenova_projects p WHERE p.team=$2 AND p.project=$5)))`, p.Subject, p.Team, role, kind, ref).Scan(&allowed)
	if err != nil || !allowed {
		return authentication.ErrForbidden
	}
	return nil
}
