// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"github.com/wunderforge/agenova/internal/workstore"
	"time"
)

//go:embed 002-invitations.sql
var invitationSchema string

//go:embed 003-application-sessions.sql
var applicationSessionSchema string

//go:embed 004-google-invitations.sql
var googleInvitationSchema string

//go:embed 006-scoped-roles.sql
var scopedRoleSchema string

var migrations = []string{Schema, invitationSchema, applicationSessionSchema, googleInvitationSchema, workstore.Schema, scopedRoleSchema}

// Migrate is an explicit operator action. Runtime credentials should have no DDL
// rights. One transaction and advisory lock prevent partial/concurrent schemas.
// Version 1 is immutable after release; future changes require another migration.
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("migration database unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("migration unavailable")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215000)`); err != nil {
		return errors.New("migration lock unavailable")
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS agenova_schema_migrations (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return errors.New("migration registry unavailable")
	}

	var newest int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(version),0) FROM agenova_schema_migrations`).Scan(&newest); err != nil || newest > len(migrations) {
		return errors.New("database version is unsupported")
	}
	for i, script := range migrations {
		sum := sha256.Sum256([]byte(script))
		checksum := hex.EncodeToString(sum[:])
		var existing string
		err = tx.QueryRowContext(ctx, `SELECT checksum FROM agenova_schema_migrations WHERE version=$1`, i+1).Scan(&existing)
		if err == nil && existing != checksum {
			return errors.New("migration checksum conflict; do not rewrite applied migrations")
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errors.New("migration registry unavailable")
		}
		if existing == "" {
			if _, err = tx.ExecContext(ctx, script); err != nil {
				return errors.New("schema migration failed")
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_schema_migrations(version,checksum) VALUES($1,$2)`, i+1, checksum); err != nil {
				return errors.New("migration record failed")
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.New("migration commit failed")
	}
	return nil
}

func CheckMigration(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT version,checksum FROM agenova_schema_migrations ORDER BY version`)
	if err != nil {
		return errors.New("database migration is missing or incompatible")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var v int
		var checksum string
		if err = rows.Scan(&v, &checksum); err != nil || v != count+1 || v > len(migrations) {
			return errors.New("database migration is incompatible")
		}
		sum := sha256.Sum256([]byte(migrations[count]))
		if checksum != hex.EncodeToString(sum[:]) {
			return errors.New("database migration checksum conflict")
		}
		count++
	}
	if rows.Err() != nil || count != len(migrations) {
		return errors.New("database migration is incomplete")
	}
	return nil
}
