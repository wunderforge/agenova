// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/memory"
)

func TestReadinessSQLRejectsTenantColumnUpdate(t *testing.T) {
	start := strings.Index(readinessSQL, "(SELECT count(*) = 2 AND bool_and(")
	end := strings.LastIndex(readinessSQL, "(SELECT count(*) = 1 AND bool_and(")
	if start < 0 || end <= start {
		t.Fatal("missing separate tenant-table safety aggregate")
	}
	guard := readinessSQL[start:end]
	if !strings.Contains(guard, "NOT has_any_column_privilege(current_user, c.oid, 'UPDATE')") {
		t.Fatal("tenant-table safety permits column-level UPDATE")
	}
	if !strings.Contains(guard, "c.relname IN ('records', 'receipts')") {
		t.Fatal("tenant-table safety does not cover both records and receipts")
	}
}

func TestReadinessSQLRejectsUnexpectedTablePrivileges(t *testing.T) {
	start := strings.Index(readinessSQL, "(SELECT count(*) = 2 AND bool_and(")
	marker := strings.LastIndex(readinessSQL, "(SELECT count(*) = 1 AND bool_and(")
	if start < 0 || marker <= start {
		t.Fatal("missing separate readiness aggregates")
	}
	for _, guard := range []string{readinessSQL[start:marker], readinessSQL[marker:]} {
		for _, privilege := range []string{"TRIGGER", "REFERENCES", "MAINTAIN"} {
			if !strings.Contains(guard, "NOT has_table_privilege(current_user, c.oid, '"+privilege+"')") {
				t.Errorf("readiness permits unexpected %s privilege", privilege)
			}
		}
		if !strings.Contains(guard, "NOT has_any_column_privilege(current_user, c.oid, 'REFERENCES')") {
			t.Error("readiness permits column-level REFERENCES")
		}
	}
}

func TestReadinessSQLRequiresReadOnlySchemaVersion(t *testing.T) {
	// Independent query-shape assertions complement scripted result handling;
	// neither executes PostgreSQL or proves effective live privileges.
	start := strings.LastIndex(readinessSQL, "(SELECT count(*) = 1 AND bool_and(")
	if start < 0 {
		t.Fatal("missing separate schema-version safety aggregate")
	}
	guard := readinessSQL[start:]
	for _, required := range []string{
		"NOT pg_has_role(current_user, c.relowner, 'MEMBER')",
		"has_table_privilege(current_user, c.oid, 'SELECT')",
		"NOT has_table_privilege(current_user, c.oid, 'INSERT')",
		"NOT has_table_privilege(current_user, c.oid, 'UPDATE')",
		"NOT has_table_privilege(current_user, c.oid, 'DELETE')",
		"NOT has_table_privilege(current_user, c.oid, 'TRUNCATE')",
		"NOT has_any_column_privilege(current_user, c.oid, 'INSERT')",
		"NOT has_any_column_privilege(current_user, c.oid, 'UPDATE')",
		"FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace",
		"n.nspname = 'agenova_memory' AND c.relname = 'schema_version'",
	} {
		if !strings.Contains(guard, required) {
			t.Errorf("schema-version guard lacks %q", required)
		}
	}
	if strings.Contains(guard, "relrowsecurity") || strings.Contains(guard, "relforcerowsecurity") {
		t.Fatal("read-only schema marker incorrectly requires tenant RLS")
	}
	if strings.Contains(guard, "\n   AND has_table_privilege(current_user, c.oid, 'INSERT')") {
		t.Fatal("schema marker incorrectly requires INSERT")
	}
}

func TestReadinessSQLRejectsAdministrativeRolesAndOwnership(t *testing.T) {
	for _, required := range []string{
		"NOT r.rolsuper", "NOT r.rolbypassrls", "NOT r.rolcreaterole", "NOT r.rolcreatedb", "NOT r.rolreplication",
		"left(r.rolname, 3) <> 'pg_'", "pg_has_role(current_user, r.oid, 'MEMBER')",
		"NOT pg_has_role(current_user, n.nspowner, 'MEMBER')",
		"NOT pg_has_role(current_user, d.datdba, 'MEMBER')",
		"NOT has_database_privilege(current_user, d.oid, 'CREATE')",
		"d.datname = current_database()",
	} {
		if !strings.Contains(readinessSQL, required) {
			t.Errorf("readiness lacks administrative/ownership guard %q", required)
		}
	}
}

func TestReadinessSchemaVersionResult(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value driver.Value
		ready bool
		err   error
	}{
		{"read-only non-owner", true, true, nil},
		{"unsafe or missing marker", false, false, memory.ErrUnavailable},
		{"unknown safety", nil, false, errFailed},
		{"invalid safety result", "private-driver-sentinel", false, errFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, s := backend(t, step{query: readinessSQL, rows: [][]driver.Value{{true, true, true, true, tc.value}}})
			got, err := New(context.Background(), b.db)
			if tc.ready {
				if got == nil || err != nil {
					t.Fatalf("safe readiness rejected: %v", err)
				}
			} else if got != nil || !errors.Is(err, tc.err) || strings.Contains(err.Error(), "private-driver-sentinel") {
				t.Fatalf("unsafe readiness not sanitized/closed: backend=%v error=%v", got, err)
			}
			if s.queries != 1 || len(s.begins) != 0 || s.commits != 0 || s.rollbacks != 0 {
				t.Fatal("readiness used additional or mutating SQL")
			}
		})
	}
}
