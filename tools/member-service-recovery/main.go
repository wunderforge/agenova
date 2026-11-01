// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Trusted one-use initializer, never a control-plane runtime or Agent command.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/lib/pq"
)

const runtimeRole = "agenova_runtime_recovery"
const baseRole = "agenova_runtime"
const ownership = "agenova/member-service-recovery/v1"

func main() {
	apply := flag.Bool("apply", false, "create the explicitly approved dedicated login")
	flag.Parse()
	if run(*apply) != nil {
		fmt.Fprintln(os.Stderr, "trusted member recovery failed; raw diagnostics suppressed")
		os.Exit(1)
	}
}

// Only a SCRAM verifier enters DDL, never the plaintext runtime password.
func verifier(password string, salt []byte) (string, error) {
	if len(salt) < 16 || len(password) < 32 || len(password) > 128 {
		return "", errors.New("invalid credential bounds")
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, 4096, 32)
	if err != nil {
		return "", errors.New("credential derivation failed")
	}
	mac := func(label string) []byte { h := hmac.New(sha256.New, key); h.Write([]byte(label)); return h.Sum(nil) }
	stored := sha256.Sum256(mac("Client Key"))
	b64 := base64.StdEncoding.EncodeToString
	return "SCRAM-SHA-256$4096:" + b64(salt) + "$" + b64(stored[:]) + ":" + b64(mac("Server Key")), nil
}

func databaseURL(user, password, host, query string) string {
	u := url.URL{Scheme: "postgres", Host: host, Path: "/agenova", User: url.UserPassword(user, password)}
	u.RawQuery = query
	return u.String()
}

type report struct {
	Mode            string `json:"mode"`
	Schema          int    `json:"schema"`
	RuntimeRole     string `json:"runtimeRole"`
	Created         bool   `json:"created"`
	CountsUnchanged bool   `json:"countsUnchanged"`
	LoginVerified   bool   `json:"loginVerified"`
}

func recoverLogin(ctx context.Context, db *sql.DB, password string, apply bool) (report, error) {
	fail := func() (report, error) { return report{}, errors.New("recovery precondition or verification failed") }
	var actor string
	var schema, members, projects int
	if db.QueryRowContext(ctx, "SELECT current_user").Scan(&actor) != nil || actor != "agenova_operator" {
		return fail()
	}
	if db.QueryRowContext(ctx, "SELECT max(version) FROM agenova_schema_migrations").Scan(&schema) != nil || schema != 11 {
		return fail()
	}
	if db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_members").Scan(&members) != nil || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_projects").Scan(&projects) != nil {
		return fail()
	}
	safeRole := func(name string) bool {
		var safe bool
		err := db.QueryRowContext(ctx, `SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT has_schema_privilege(rolname,'public','CREATE') AND NOT has_database_privilege(rolname,current_database(),'CREATE') AND NOT has_table_privilege(rolname,'agenova_identity_audit','UPDATE,DELETE') AND NOT has_table_privilege(rolname,'agenova_work_facts','UPDATE,DELETE') AND has_table_privilege(rolname,'agenova_members','SELECT') FROM pg_roles WHERE rolname=$1`, name).Scan(&safe)
		return err == nil && safe
	}
	if !safeRole(baseRole) {
		return fail()
	}
	out := report{Mode: "inspect", Schema: schema, RuntimeRole: runtimeRole, CountsUnchanged: true}
	if !apply {
		return out, nil
	}
	var exists bool
	if db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", runtimeRole).Scan(&exists) != nil {
		return fail()
	}
	if exists {
		var marker string
		var inherited bool
		if db.QueryRowContext(ctx, "SELECT COALESCE(shobj_description(oid,'pg_authid'),'') FROM pg_roles WHERE rolname=$1", runtimeRole).Scan(&marker) != nil || marker != ownership || !safeRole(runtimeRole) {
			return fail()
		}
		if db.QueryRowContext(ctx, "SELECT pg_has_role($1,$2,'MEMBER')", runtimeRole, baseRole).Scan(&inherited) != nil || !inherited {
			return fail()
		}
	} else {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return fail()
		}
		encoded, err := verifier(password, salt)
		if err != nil {
			return fail()
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fail()
		}
		defer tx.Rollback()
		// Password is already a high-entropy SCRAM verifier, not a plaintext DDL literal.
		statements := []string{"CREATE ROLE " + pq.QuoteIdentifier(runtimeRole) + " LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 12 PASSWORD " + pq.QuoteLiteral(encoded), "GRANT " + pq.QuoteIdentifier(baseRole) + " TO " + pq.QuoteIdentifier(runtimeRole), "COMMENT ON ROLE " + pq.QuoteIdentifier(runtimeRole) + " IS " + pq.QuoteLiteral(ownership)}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fail()
			}
		}
		if err := tx.Commit(); err != nil {
			return fail()
		}
		out.Created = true
	}
	var afterMembers, afterProjects int
	var exactMembership bool
	if db.QueryRowContext(ctx, `SELECT count(*)=1 AND COALESCE(bool_and(r.rolname=$2 AND NOT m.admin_option),false) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid WHERE m.member=(SELECT oid FROM pg_roles WHERE rolname=$1)`, runtimeRole, baseRole).Scan(&exactMembership) != nil || !exactMembership {
		return fail()
	}
	if !safeRole(runtimeRole) || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_members").Scan(&afterMembers) != nil || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_projects").Scan(&afterProjects) != nil || members != afterMembers || projects != afterProjects {
		return fail()
	}
	out.Mode = "apply"
	return out, nil
}

func run(apply bool) error {
	var credentials struct{ Username, Password string }
	raw, err := os.ReadFile("/run/operator/credentials.json")
	if err != nil || len(raw) > 32<<10 || json.Unmarshal(raw, &credentials) != nil || credentials.Username != "agenova_operator" || credentials.Password == "" {
		return errors.New("credentials unavailable")
	}
	password, err := os.ReadFile("/run/operator/runtime-password")
	if err != nil || len(password) < 32 || len(password) > 128 {
		return errors.New("runtime credential unavailable")
	}
	query := url.Values{"sslmode": {"verify-full"}, "sslrootcert": {"/etc/rds/rds-ca.pem"}, "connect_timeout": {"8"}}.Encode()
	host := "agenova-production.cpm4eoa4ms54.ap-southeast-2.rds.amazonaws.com:5432"
	db, err := sql.Open("postgres", databaseURL(credentials.Username, credentials.Password, host, query))
	if err != nil {
		return errors.New("database unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := recoverLogin(ctx, db, string(password), apply)
	if err != nil {
		return err
	}
	if apply {
		runtime, err := sql.Open("postgres", databaseURL(runtimeRole, string(password), host, query))
		if err != nil {
			return errors.New("login verification failed")
		}
		defer runtime.Close()
		runtime.SetMaxOpenConns(1)
		var current string
		if runtime.QueryRowContext(ctx, "SELECT current_user FROM agenova_members LIMIT 1").Scan(&current) != nil || current != runtimeRole {
			return errors.New("login verification failed")
		}
		out.LoginVerified = true
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
