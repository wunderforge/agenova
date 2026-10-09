// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package postgres owns PostgreSQL storage semantics, not invocation authority.
package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/memory"
)

//go:embed migrations/001_memory.sql
var migration string

// MigrationSQL is operator-only input. New never grants privileges or migrates.
func MigrationSQL() string { return migration }

var errFailed = errors.New("memory backend failed")

type Backend struct{ db *sql.DB }

var _ memory.Backend = (*Backend)(nil)

const readinessSQL = `SELECT
 (SELECT version = 1 FROM agenova_memory.schema_version WHERE singleton),
 (SELECT count(*) > 0 AND bool_and(NOT r.rolsuper AND NOT r.rolbypassrls
   AND NOT r.rolcreaterole AND NOT r.rolcreatedb AND NOT r.rolreplication
   AND left(r.rolname, 3) <> 'pg_')
  FROM pg_catalog.pg_roles r WHERE pg_has_role(current_user, r.oid, 'MEMBER')),
 (SELECT count(*) = 1 AND bool_and(NOT pg_has_role(current_user, n.nspowner, 'MEMBER')
   AND NOT has_schema_privilege(current_user, n.oid, 'CREATE'))
  FROM pg_catalog.pg_namespace n WHERE n.nspname = 'agenova_memory')
 AND (SELECT NOT pg_has_role(current_user, d.datdba, 'MEMBER')
   AND NOT has_database_privilege(current_user, d.oid, 'CREATE')
  FROM pg_catalog.pg_database d WHERE d.datname = current_database()),
 (SELECT count(*) = 2 AND bool_and(c.relrowsecurity AND c.relforcerowsecurity
   AND NOT pg_has_role(current_user, c.relowner, 'MEMBER')
   AND has_table_privilege(current_user, c.oid, 'SELECT')
   AND has_table_privilege(current_user, c.oid, 'INSERT')
   AND NOT has_table_privilege(current_user, c.oid, 'UPDATE')
   AND NOT has_any_column_privilege(current_user, c.oid, 'UPDATE')
   AND NOT has_table_privilege(current_user, c.oid, 'DELETE')
   AND NOT has_table_privilege(current_user, c.oid, 'TRIGGER')
   AND NOT has_table_privilege(current_user, c.oid, 'REFERENCES')
   AND NOT has_any_column_privilege(current_user, c.oid, 'REFERENCES')
   AND NOT has_table_privilege(current_user, c.oid, 'MAINTAIN')
   AND NOT has_table_privilege(current_user, c.oid, 'TRUNCATE'))
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'agenova_memory' AND c.relname IN ('records', 'receipts')),
 (SELECT count(*) = 1 AND bool_and(
   NOT pg_has_role(current_user, c.relowner, 'MEMBER')
   AND has_table_privilege(current_user, c.oid, 'SELECT')
   AND NOT has_table_privilege(current_user, c.oid, 'INSERT')
   AND NOT has_table_privilege(current_user, c.oid, 'UPDATE')
   AND NOT has_table_privilege(current_user, c.oid, 'DELETE')
   AND NOT has_table_privilege(current_user, c.oid, 'TRUNCATE')
   AND NOT has_table_privilege(current_user, c.oid, 'TRIGGER')
   AND NOT has_table_privilege(current_user, c.oid, 'REFERENCES')
   AND NOT has_any_column_privilege(current_user, c.oid, 'REFERENCES')
   AND NOT has_table_privilege(current_user, c.oid, 'MAINTAIN')
   AND NOT has_any_column_privilege(current_user, c.oid, 'INSERT')
   AND NOT has_any_column_privilege(current_user, c.oid, 'UPDATE'))
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'agenova_memory' AND c.relname = 'schema_version')`

// New receives a host-owned connection pool after governed credential resolution.
// It accepts no DSN, secret, task path, or implicit environment configuration.
// A supported driver and the #155 installation composition are still required.
func New(ctx context.Context, db *sql.DB) (*Backend, error) {
	if ctx == nil || db == nil {
		return nil, memory.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, memory.CallTimeout)
	defer cancel()
	var version, role, schema, tables, marker bool
	if err := db.QueryRowContext(ctx, readinessSQL).Scan(&version, &role, &schema, &tables, &marker); err != nil {
		return nil, fault(err)
	}
	if !version || !role || !schema || !tables || !marker {
		return nil, memory.ErrUnavailable
	}
	return &Backend{db: db}, nil
}

const tenantSQL = `SELECT set_config('agenova.team', $1, true), set_config('agenova.project', $2, true), set_config('agenova.scope', $3, true)`
const receiptSQL = `SELECT p.request_digest, r.id, r.body, r.source_claim, r.created_at
 FROM agenova_memory.receipts p JOIN agenova_memory.records r
 ON r.team=p.team AND r.project=p.project AND r.scope=p.scope AND r.id=p.memory_id
 WHERE p.team=$1 AND p.project=$2 AND p.scope=$3 AND p.invocation_id=$4`
const insertSQL = `INSERT INTO agenova_memory.records (team, project, scope, id, body, source_claim)
 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, body, source_claim, created_at`
const receiptInsertSQL = `INSERT INTO agenova_memory.receipts (team, project, scope, invocation_id, request_digest, memory_id) VALUES ($1,$2,$3,$4,$5,$6)`
const searchSQL = `SELECT id, body, source_claim, created_at FROM agenova_memory.records
 WHERE team=$1 AND project=$2 AND scope=$3 AND strpos(lower(body), lower($4)) > 0
 ORDER BY created_at DESC, id DESC LIMIT $5`

func (b *Backend) begin(ctx context.Context, n memory.Namespace, readOnly bool) (*sql.Tx, context.Context, context.CancelFunc, error) {
	if b == nil || b.db == nil || ctx == nil {
		return nil, nil, func() {}, memory.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, memory.CallTimeout)
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: readOnly})
	if err != nil {
		cancel()
		return nil, nil, func() {}, fault(err)
	}
	if _, err := tx.ExecContext(ctx, tenantSQL, n.Team, n.Project, n.Scope); err != nil {
		_ = tx.Rollback()
		cancel()
		return nil, nil, func() {}, fault(err)
	}
	return tx, ctx, cancel, nil
}

func validNamespace(n memory.Namespace) bool {
	return validText(n.Team, 256) && validText(n.Project, 256) && validText(n.Scope, 256)
}
func validText(s string, max int) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && len(s) <= max && !strings.ContainsRune(s, 0)
}

func (b *Backend) Write(ctx context.Context, n memory.Namespace, in memory.WriteInput) (memory.Record, error) {
	if !validNamespace(n) || !validText(in.Body, memory.MaxBodyBytes) || !validText(in.ClaimID, 256) || !validText(in.InvocationID, 256) {
		return memory.Record{}, errFailed
	}
	tx, ctx, cancel, err := b.begin(ctx, n, false)
	if err != nil {
		return memory.Record{}, err
	}
	defer cancel()
	defer tx.Rollback()
	// Serialize identical receipts within the ownership tuple. Hash collisions
	// can only serialize unrelated writes; they cannot grant access to a row.
	lock, _ := json.Marshal([]string{n.Team, n.Project, n.Scope, in.InvocationID})
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, string(lock)); err != nil {
		return memory.Record{}, fault(err)
	}
	encoded, _ := json.Marshal([]string{in.ClaimID, in.Body})
	digest := sha256.Sum256(encoded)
	requestDigest := hex.EncodeToString(digest[:])
	record := memory.Record{Namespace: n}
	var previousDigest string
	err = tx.QueryRowContext(ctx, receiptSQL, n.Team, n.Project, n.Scope, in.InvocationID).Scan(&previousDigest, &record.Reference, &record.Body, &record.SourceClaim, &record.CreatedAt)
	if err == nil {
		if previousDigest != requestDigest || !validRecord(record) || record.Body != in.Body || record.SourceClaim != in.ClaimID {
			return memory.Record{}, errFailed
		}
		// The receipt was already committed. This transaction adds no data;
		// rollback releases the advisory lock without obscuring a known commit.
		return record, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return memory.Record{}, fault(err)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return memory.Record{}, errFailed
	}
	record.Reference = "memory:" + hex.EncodeToString(id[:])
	err = tx.QueryRowContext(ctx, insertSQL, n.Team, n.Project, n.Scope, record.Reference, in.Body, in.ClaimID).Scan(&record.Reference, &record.Body, &record.SourceClaim, &record.CreatedAt)
	if err != nil {
		return memory.Record{}, fault(err)
	}
	if !validRecord(record) || record.Body != in.Body || record.SourceClaim != in.ClaimID {
		return memory.Record{}, errFailed
	}
	if _, err := tx.ExecContext(ctx, receiptInsertSQL, n.Team, n.Project, n.Scope, in.InvocationID, requestDigest, record.Reference); err != nil {
		return memory.Record{}, fault(err)
	}
	if err := ctx.Err(); err != nil {
		return memory.Record{}, fault(err)
	}
	if err := tx.Commit(); err != nil {
		// Only a server-declared rollback is definite. Driver/network errors,
		// including cancellation around COMMIT, cannot promise no side effect.
		var state interface{ SQLState() string }
		if errors.As(err, &state) {
			code := state.SQLState()
			// 40003 means statement_completion_unknown despite its class 40.
			if code != "40003" && (strings.HasPrefix(code, "40") || strings.HasPrefix(code, "23") || strings.HasPrefix(code, "25")) {
				return memory.Record{}, errFailed
			}
		}
		return memory.Record{}, memory.ErrWriteUncertain
	}
	return record, nil
}

func (b *Backend) Search(ctx context.Context, n memory.Namespace, in memory.SearchInput) ([]memory.Record, error) {
	if in.Limit == 0 {
		in.Limit = memory.DefaultLimit
	}
	if !validNamespace(n) || !validText(in.Query, memory.MaxQueryBytes) || !validText(in.InvocationID, 256) || in.Limit < 1 || in.Limit > memory.MaxLimit {
		return nil, errFailed
	}
	tx, ctx, cancel, err := b.begin(ctx, n, true)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, searchSQL, n.Team, n.Project, n.Scope, in.Query, in.Limit)
	if err != nil {
		return nil, fault(err)
	}
	defer rows.Close()
	records := []memory.Record{}
	for rows.Next() {
		record := memory.Record{Namespace: n}
		if err := rows.Scan(&record.Reference, &record.Body, &record.SourceClaim, &record.CreatedAt); err != nil {
			return nil, fault(err)
		}
		if !validRecord(record) || len(records) >= in.Limit {
			return nil, errFailed
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fault(err)
	}
	if err := rows.Close(); err != nil {
		return nil, fault(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fault(err)
	}
	return records, nil
}

func validRecord(r memory.Record) bool {
	return facts.ValidMemoryReference(r.Reference) && validText(r.Body, memory.MaxBodyBytes) && validText(r.SourceClaim, 256) && !r.CreatedAt.IsZero()
}

func fault(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) && (strings.HasPrefix(state.SQLState(), "08") || strings.HasPrefix(state.SQLState(), "53") || strings.HasPrefix(state.SQLState(), "57")) {
		return memory.ErrUnavailable
	}
	var network net.Error
	if errors.As(err, &network) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return memory.ErrUnavailable
	}
	return errFailed
}
