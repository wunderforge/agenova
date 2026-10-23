// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"regexp"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

var projectSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// CreateProject uses the live account role, not a caller-supplied team or grant.
// Creation and audit are atomic. Reapplication does not rewrite policy, templates,
// membership or historical Work. All new projects start without explicit grants.
func (s Store) CreateProject(ctx context.Context, p v0.Principal, project string) (bool, error) {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return false, authentication.ErrForbidden
	}
	if !projectSlug.MatchString(project) {
		return false, invitations.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, invitations.ErrUnavailable
	}
	defer tx.Rollback()
	var role string
	if tx.QueryRowContext(ctx, `SELECT account_role FROM agenova_members WHERE principal=$1 AND team=$2 AND active FOR UPDATE`, p.Subject, p.Team).Scan(&role) != nil || role != Owner && role != Admin {
		return false, authentication.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,215012))`, p.Team); err != nil {
		return false, invitations.ErrUnavailable
	}
	var count int
	var exists bool
	if tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(bool_or(project=$2),false) FROM agenova_projects WHERE team=$1`, p.Team, project).Scan(&count, &exists) != nil {
		return false, invitations.ErrUnavailable
	}
	if exists {
		return false, tx.Commit()
	}
	if count >= 100 {
		return false, invitations.ErrInvalid
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_projects(team,project) VALUES($1,$2)`, p.Team, project); err != nil {
		return false, invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'project.create',$2)`, p.Subject, p.Team+"/"+project); err != nil {
		return false, invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return false, invitations.ErrUnavailable
	}
	return true, nil
}
