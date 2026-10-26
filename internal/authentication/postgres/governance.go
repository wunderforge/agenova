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
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"sort"
	"time"
)

// ProjectRegistry gives Work a scoped view. The operator-installed defaults remain
// readable until a scoped record exists; a failed database read never falls back.
type ProjectRegistry struct {
	Store         Store
	Team, Project string
	Fallback      registration.Store
}

func (r ProjectRegistry) ActivePolicy() (policy.PolicyBundle, error) {
	var b []byte
	var value policy.PolicyBundle
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if r.Store.DB == nil {
		return value, errors.New("project registry unavailable")
	}
	err := r.Store.DB.QueryRowContext(ctx, `SELECT p.document FROM agenova_project_policy_active a JOIN agenova_project_policies p USING(team,project,id,version) WHERE a.team=$1 AND a.project=$2`, r.Team, r.Project).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		if r.Fallback == nil {
			return value, errors.New("project policy unavailable")
		}
		return r.Fallback.ActivePolicy()
	}
	if err != nil {
		return value, errors.New("project policy unavailable")
	}
	err = json.Unmarshal(b, &value)
	return value, err
}
func (r ProjectRegistry) Template(name string) (*v0.AgentTemplate, error) {
	var b []byte
	var value v0.AgentTemplate
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if r.Store.DB == nil {
		return nil, errors.New("project registry unavailable")
	}
	err := r.Store.DB.QueryRowContext(ctx, `SELECT document FROM agenova_project_templates WHERE team=$1 AND project=$2 AND name=$3`, r.Team, r.Project, name).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		if r.Fallback == nil {
			return nil, errors.New("project template unavailable")
		}
		return r.Fallback.Template(name)
	}
	if err != nil {
		return nil, errors.New("project template unavailable")
	}
	err = json.Unmarshal(b, &value)
	return &value, err
}

// ApplyGovernance commits immutable registration, policy activation and audit
// together. Actor row locking serializes against all current grant/revoke paths.
func (s Store) ApplyGovernance(ctx context.Context, p v0.Principal, project, kind string, data []byte, validate func(*v0.AgentTemplate) error, expected ...*v0.PolicyReference) (registration.Result, error) {
	var out registration.Result
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" || project == "" || len(project) > 128 || len(data) > 1<<20 {
		return out, authentication.ErrForbidden
	}
	var document []byte
	var name, version string
	switch kind {
	case "policy":
		bundle, err := policy.ParseDocumentYAML(data)
		if err != nil {
			return out, err
		}
		for _, rule := range bundle.Rules {
			if rule.Team != p.Team || rule.Project != project {
				return out, authentication.ErrForbidden
			}
		}
		name, version = bundle.ID, bundle.Version
		document, _ = json.Marshal(bundle)
		out.Kind = policy.DocumentKind
	case "template":
		template, err := v0.ParseAgentTemplateYAML(data)
		if err != nil {
			return out, err
		}
		if validate == nil {
			return out, errors.New("runtime compatibility check unavailable")
		}
		if err := validate(template); err != nil {
			return out, err
		}
		name = template.Metadata.Name
		document, _ = json.Marshal(template)
		out.Kind = "AgentTemplate"
	default:
		return out, authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, errors.New("registry unavailable")
	}
	defer tx.Rollback()
	var active bool
	if tx.QueryRowContext(ctx, `SELECT active FROM agenova_members WHERE principal=$1 AND team=$2 FOR UPDATE`, p.Subject, p.Team).Scan(&active) != nil || !active {
		return out, authentication.ErrForbidden
	}
	var allowed bool
	if tx.QueryRowContext(ctx, `SELECT (EXISTS(SELECT 1 FROM agenova_scoped_roles WHERE principal=$1 AND team=$2 AND role='governance-admin' AND ((scope_kind='project' AND scope_ref=$3) OR (scope_kind='account' AND scope_ref=$2))) OR EXISTS(SELECT 1 FROM agenova_members m WHERE m.principal=$1 AND m.team=$2 AND m.account_role IN ('owner','admin'))) AND EXISTS(SELECT 1 FROM agenova_projects WHERE team=$2 AND project=$3)`, p.Subject, p.Team, project).Scan(&allowed) != nil || !allowed {
		return out, authentication.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,215007))`, p.Team+":"+project); err != nil {
		return out, errors.New("registry lock unavailable")
	}
	var changed bool
	if kind == "policy" {
		if len(expected) > 1 {
			return out, registration.ErrConflict
		}
		if len(expected) == 1 && expected[0] != nil {
			var current v0.PolicyReference
			e := tx.QueryRowContext(ctx, `SELECT id,version FROM agenova_project_policy_active WHERE team=$1 AND project=$2`, p.Team, project).Scan(&current.ID, &current.Version)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return out, errors.New("policy precondition unavailable")
			}
			if current != *expected[0] {
				return out, registration.ErrConflict
			}
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO agenova_project_policies VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, p.Team, project, name, version, document)
		if e != nil {
			return out, errors.New("policy registration unavailable")
		}
		n, _ := result.RowsAffected()
		changed = n > 0
		var equal bool
		if tx.QueryRowContext(ctx, `SELECT document=$5::jsonb FROM agenova_project_policies WHERE team=$1 AND project=$2 AND id=$3 AND version=$4`, p.Team, project, name, version, document).Scan(&equal) != nil || !equal {
			return out, registration.ErrConflict
		}
		result, e = tx.ExecContext(ctx, `INSERT INTO agenova_project_policy_active VALUES($1,$2,$3,$4) ON CONFLICT(team,project) DO UPDATE SET id=EXCLUDED.id,version=EXCLUDED.version WHERE agenova_project_policy_active.id<>EXCLUDED.id OR agenova_project_policy_active.version<>EXCLUDED.version`, p.Team, project, name, version)
		if e != nil {
			return out, errors.New("policy activation unavailable")
		}
		n, _ = result.RowsAffected()
		changed = changed || n > 0
		out.Active = true
		if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_policy_activations(team,project,id,version,actor) VALUES($1,$2,$3,$4,$5)`, p.Team, project, name, version, p.Subject); err != nil {
			return out, errors.New("policy activation audit unavailable")
		}
	} else {
		result, e := tx.ExecContext(ctx, `INSERT INTO agenova_project_templates VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, p.Team, project, name, document)
		if e != nil {
			return out, errors.New("template registration unavailable")
		}
		n, _ := result.RowsAffected()
		changed = n > 0
		var equal bool
		if tx.QueryRowContext(ctx, `SELECT document=$4::jsonb FROM agenova_project_templates WHERE team=$1 AND project=$2 AND name=$3`, p.Team, project, name, document).Scan(&equal) != nil || !equal {
			return out, registration.ErrConflict
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, p.Subject, "governance."+kind+".apply", p.Team+"/"+project+"/"+name+"@"+version); err != nil {
		return out, errors.New("registry audit unavailable")
	}
	if tx.Commit() != nil {
		return out, errors.New("registry commit unavailable")
	}
	out.Name = name
	out.Version = version
	out.Changed = changed
	return out, nil
}

func (r ProjectRegistry) Lookup(name string) (*v0.AgentTemplate, error) { return r.Template(name) }

// ConfigurationProjects lists only projects whose configuration the current
// member may inspect. It grants no Work submission or evidence permissions.
func (s Store) ConfigurationProjects(ctx context.Context, p v0.Principal) ([]string, error) {
	if s.DB == nil || p.AuthenticationContext != "oidc" {
		return nil, authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := s.DB.QueryContext(ctx, `SELECT p.project FROM agenova_projects p JOIN agenova_members m ON m.team=p.team WHERE m.principal=$1 AND m.team=$2 AND m.active AND (m.account_role IN ('owner','admin') OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=m.principal AND g.team=p.team AND g.project=p.project) OR EXISTS(SELECT 1 FROM agenova_scoped_roles g WHERE g.principal=m.principal AND g.team=p.team AND g.role='governance-admin' AND ((g.scope_kind='account' AND g.scope_ref=p.team) OR (g.scope_kind='project' AND g.scope_ref=p.project)))) ORDER BY p.project LIMIT 101`, p.Subject, p.Team)
	if err != nil {
		return nil, errors.New("project configuration unavailable")
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	if len(result) > 100 {
		return nil, errors.New("project configuration limit exceeded")
	}
	return result, rows.Err()
}
func (r ProjectRegistry) TemplateNames(defaults []string) ([]string, error) {
	if r.Store.DB == nil {
		return nil, errors.New("project registry unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := r.Store.DB.QueryContext(ctx, `SELECT name FROM agenova_project_templates WHERE team=$1 AND project=$2 ORDER BY name LIMIT 101`, r.Team, r.Project)
	if err != nil {
		return nil, errors.New("project templates unavailable")
	}
	defer rows.Close()
	names := map[string]bool{}
	for _, n := range defaults {
		names[n] = true
	}
	for rows.Next() {
		var n string
		if err = rows.Scan(&n); err != nil {
			return nil, err
		}
		names[n] = true
	}
	if rows.Err() != nil || len(names) > 100 {
		return nil, errors.New("project templates unavailable")
	}
	result := []string{}
	for n := range names {
		result = append(result, n)
	}
	sort.Strings(result)
	return result, nil
}
