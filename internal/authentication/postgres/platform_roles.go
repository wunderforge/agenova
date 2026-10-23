// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
	"time"
)

const PlatformAdmin = "platform-admin"

// ChangePlatformRole changes the entire application role atomically. Old scoped
// grants are removed, so demotion cannot retain hidden governance/deployment access.
// Project membership and cloud IAM remain independent of application role.
func (s Store) ChangePlatformRole(ctx context.Context, p v0.Principal, c invitations.MemberChange) error {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return authentication.ErrForbidden
	}
	if c.Principal == "" || c.ScopeKind != "" || c.Confirmation != "" || c.ScopeRef == "" || len(c.ScopeRef) > 128 || c.Project == "" || len(c.Project) > 128 || (c.Role != PlatformAdmin && c.Role != PlatformOperator && c.Role != MemberRole) {
		return invitations.ErrInvalid
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
	// Owner and administrators may assign all operational roles. Protect Owner and self.
	if target == Owner || (actor != Owner && actor != Admin) || c.Principal == p.Subject {
		return authentication.ErrForbidden
	}
	var active, project bool
	if tx.QueryRowContext(ctx, `SELECT active FROM agenova_members WHERE principal=$1 AND team=$2`, c.Principal, p.Team).Scan(&active) != nil || !active {
		return authentication.ErrForbidden
	}
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_projects WHERE team=$1 AND project=$2)`, p.Team, c.Project).Scan(&project) != nil || !project {
		return authentication.ErrForbidden
	}
	account, grant := MemberRole, MemberRole
	if c.Role == PlatformAdmin {
		account, grant = Admin, Admin
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_members SET account_role=$3 WHERE principal=$1 AND team=$2`, c.Principal, p.Team, account); err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM agenova_scoped_roles WHERE principal=$1 AND team=$2`, c.Principal, p.Team); err != nil {
		return invitations.ErrUnavailable
	}
	if c.Role == PlatformOperator {
		if _, err = tx.ExecContext(ctx, `DELETE FROM agenova_project_grants WHERE principal=$1 AND team=$2`, c.Principal, p.Team); err != nil {
			return invitations.ErrUnavailable
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_scoped_roles(principal,team,role,scope_kind,scope_ref) VALUES($1,$2,'platform-operator','installation',$3)`, c.Principal, p.Team, c.ScopeRef); err != nil {
			return invitations.ErrUnavailable
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE agenova_project_grants SET role=$3 WHERE principal=$1 AND team=$2`, c.Principal, p.Team, grant); err != nil {
			return invitations.ErrUnavailable
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_project_grants(principal,team,project,role) VALUES($1,$2,$3,$4) ON CONFLICT(principal,project) DO UPDATE SET role=EXCLUDED.role WHERE agenova_project_grants.team=EXCLUDED.team`, c.Principal, p.Team, c.Project, grant); err != nil {
			return invitations.ErrUnavailable
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, p.Subject, "platform-role."+c.Role+".installation."+c.ScopeRef, c.Principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}
