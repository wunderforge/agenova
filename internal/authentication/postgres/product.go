// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/policy"
	"strings"
	"time"
)

type OrganizationInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

func (s Store) Organization(ctx context.Context, p v0.Principal) (OrganizationInfo, error) {
	out := OrganizationInfo{ID: p.Team, DisplayName: p.Team}
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Team == "" || p.Subject == "" {
		return out, authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var active bool
	if s.DB.QueryRowContext(ctx, `SELECT active FROM agenova_members WHERE principal=$1 AND team=$2`, p.Subject, p.Team).Scan(&active) != nil || !active {
		return out, authentication.ErrForbidden
	}
	err := s.DB.QueryRowContext(ctx, `SELECT display_name,description FROM agenova_organization_metadata WHERE team=$1`, p.Team).Scan(&out.DisplayName, &out.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}

func (s Store) UpdateOrganization(ctx context.Context, p v0.Principal, name, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return authentication.ErrForbidden
	}
	if name == "" || len(name) > 128 || len(description) > 1024 {
		return invitations.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.ErrUnavailable
	}
	defer tx.Rollback()
	var role string
	if tx.QueryRowContext(ctx, `SELECT account_role FROM agenova_members WHERE principal=$1 AND team=$2 AND active FOR UPDATE`, p.Subject, p.Team).Scan(&role) != nil || (role != Owner && role != Admin) {
		return authentication.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_organization_metadata(team,display_name,description) VALUES($1,$2,$3) ON CONFLICT(team) DO UPDATE SET display_name=EXCLUDED.display_name,description=EXCLUDED.description`, p.Team, name, description); err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'organization.metadata.update',$2)`, p.Subject, p.Team); err != nil {
		return invitations.ErrUnavailable
	}
	return tx.Commit()
}

type PolicyVersion struct {
	Policy policy.PolicyBundle `json:"policy"`
	Active bool                `json:"active"`
}
type PolicyHistory struct {
	Active   *v0.PolicyReference `json:"active"`
	Versions []PolicyVersion     `json:"versions"`
	Events   []PolicyActivation  `json:"events"`
}
type PolicyActivation struct {
	Actor  string    `json:"actor"`
	Target string    `json:"target"`
	At     time.Time `json:"at"`
}

// PolicyHistoryFor authorizes configuration inspection on every read. No fallback
// document is labelled a historical project-owned version.
func (s Store) PolicyHistoryFor(ctx context.Context, p v0.Principal, project string) (PolicyHistory, error) {
	out := PolicyHistory{Versions: []PolicyVersion{}, Events: []PolicyActivation{}}
	projects, err := s.ConfigurationProjects(ctx, p)
	if err != nil {
		return out, err
	}
	allowed := false
	for _, value := range projects {
		allowed = allowed || value == project
	}
	if !allowed {
		return out, authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Recheck membership and scope in the transaction's statement snapshot.
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var allowedNow bool
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_members m JOIN agenova_projects p ON p.team=m.team WHERE m.principal=$1 AND m.team=$2 AND m.active AND p.project=$3 AND (m.account_role IN ('owner','admin') OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=m.principal AND g.team=p.team AND g.project=p.project) OR EXISTS(SELECT 1 FROM agenova_scoped_roles g WHERE g.principal=m.principal AND g.team=p.team AND g.role='governance-admin' AND ((g.scope_kind='account' AND g.scope_ref=p.team) OR (g.scope_kind='project' AND g.scope_ref=p.project)))))`, p.Subject, p.Team, project).Scan(&allowedNow) != nil || !allowedNow {
		return out, authentication.ErrForbidden
	}
	var ref v0.PolicyReference
	err = tx.QueryRowContext(ctx, `SELECT id,version FROM agenova_project_policy_active WHERE team=$1 AND project=$2`, p.Team, project).Scan(&ref.ID, &ref.Version)
	if err == nil {
		out.Active = &ref
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT document FROM agenova_project_policies WHERE team=$1 AND project=$2 ORDER BY id,version LIMIT 101`, p.Team, project)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var data []byte
		var b policy.PolicyBundle
		if rows.Scan(&data) != nil || json.Unmarshal(data, &b) != nil {
			rows.Close()
			return out, invitations.ErrUnavailable
		}
		out.Versions = append(out.Versions, PolicyVersion{b, out.Active != nil && b.ID == ref.ID && b.Version == ref.Version})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Versions) > 100 {
		return out, invitations.ErrUnavailable // Never present silently truncated history.
	}
	events, err := tx.QueryContext(ctx, `SELECT actor,id || '@' || version,activated_at FROM agenova_policy_activations WHERE team=$1 AND project=$2 ORDER BY event_id DESC LIMIT 50`, p.Team, project)
	if err != nil {
		return out, err
	}
	for events.Next() {
		var e PolicyActivation
		if err = events.Scan(&e.Actor, &e.Target, &e.At); err != nil {
			events.Close()
			return out, err
		}
		out.Events = append(out.Events, e)
	}
	err = events.Err()
	events.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
