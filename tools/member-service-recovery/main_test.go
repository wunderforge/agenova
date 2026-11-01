// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVerifierBoundsAndNoPlaintextDDL(t *testing.T) {
	password := strings.Repeat("a", 43)
	encoded, err := verifier(password, []byte("0123456789abcdef"))
	if err != nil || !strings.HasPrefix(encoded, "SCRAM-SHA-256$4096:") || strings.Contains(encoded, password) {
		t.Fatal("invalid SCRAM encoding")
	}
	for _, bad := range []string{"", "short", strings.Repeat("x", 129)} {
		if _, err := verifier(bad, make([]byte, 16)); err == nil {
			t.Fatal("credential bound omitted")
		}
	}
	if _, err := verifier(password, make([]byte, 4)); err == nil {
		t.Fatal("salt too short")
	}
	u, err := url.Parse(databaseURL("user", "p@ss:/?&", "example.invalid:5432", "sslmode=verify-full"))
	if err != nil || u.User.Username() != "user" || u.Query().Get("sslmode") != "verify-full" {
		t.Fatal("database URL boundaries lost")
	}
	if decoded, _ := u.User.Password(); decoded != "p@ss:/?&" {
		t.Fatal("password was not encoded")
	}
	if _, err := base64.StdEncoding.DecodeString(strings.Split(strings.Split(encoded, "$")[1], ":")[1]); err != nil {
		t.Fatal("invalid salt")
	}
}

// Explicit disposable PostgreSQL only; no cloud credential or live DB is used.
func TestRecoveryPostgresIsAdditiveAndRetrySafe(t *testing.T) {
	dsn := os.Getenv("AGENOVA_RECOVERY_TEST_DSN")
	if dsn == "" {
		t.Skip("explicit disposable PostgreSQL fixture not supplied")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "/agenova_recovery_ci" {
		t.Fatal("refusing non-fixture database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal("fixture unavailable")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	setup := []string{
		`CREATE ROLE agenova_operator LOGIN SUPERUSER PASSWORD 'fixture-operator-only'`,
		`CREATE ROLE agenova_runtime LOGIN PASSWORD 'fixture-original-role-password'`,
		`CREATE TABLE agenova_schema_migrations(version int); INSERT INTO agenova_schema_migrations VALUES(11)`,
		`CREATE TABLE agenova_members(id int); INSERT INTO agenova_members VALUES(1)`,
		`CREATE TABLE agenova_projects(id int); INSERT INTO agenova_projects VALUES(1)`,
		`CREATE TABLE agenova_identity_audit(id int)`,
		`CREATE TABLE agenova_work_facts(id int)`,
		`GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA public TO agenova_runtime`,
	}
	for _, statement := range setup {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal("fixture setup failed")
		}
	}
	var original string
	if db.QueryRowContext(ctx, "SELECT rolpassword FROM pg_authid WHERE rolname='agenova_runtime'").Scan(&original) != nil {
		t.Fatal("fixture baseline unavailable")
	}
	u.User = url.UserPassword("agenova_operator", "fixture-operator-only")
	operator, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("fixture operator unavailable")
	}
	defer operator.Close()
	password := strings.Repeat("A", 43)
	inspected, err := recoverLogin(ctx, operator, password, false)
	if err != nil || inspected.Created {
		t.Fatal("inspect changed a credential", err)
	}
	created, err := recoverLogin(ctx, operator, password, true)
	if err != nil || !created.Created || !created.CountsUnchanged {
		t.Fatal("additive recovery failed", err)
	}
	u.User = url.UserPassword(runtimeRole, password)
	login, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("runtime fixture unavailable")
	}
	defer login.Close()
	if login.PingContext(ctx) != nil {
		t.Fatal("actual SCRAM login failed")
	}
	var after string
	if db.QueryRowContext(ctx, "SELECT rolpassword FROM pg_authid WHERE rolname='agenova_runtime'").Scan(&after) != nil || after != original {
		t.Fatal("existing password changed")
	}
	again, err := recoverLogin(ctx, operator, strings.Repeat("B", 43), true)
	if err != nil || again.Created {
		t.Fatal("retry rotated an existing credential", err)
	}
	if _, err := db.ExecContext(ctx, "COMMENT ON ROLE agenova_runtime_recovery IS 'unowned'"); err != nil {
		t.Fatal("fixture mutation failed")
	}
	if _, err := recoverLogin(ctx, operator, password, true); err == nil {
		t.Fatal("unknown ownership overwritten")
	}
}
