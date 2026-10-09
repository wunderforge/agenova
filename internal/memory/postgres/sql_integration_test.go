//go:build memorypostgres

// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This selected gate executes the production schema/readiness SQL through psql.
// It does not exercise a Go driver, installed credentials, or worker governance.
// A missing explicit image is a failure, never a skipped real-backend gate.
func TestPostgresSQLIntegration(t *testing.T) {
	image := os.Getenv("AGENOVA_MEMORY_POSTGRES_IMAGE")
	if !regexp.MustCompile(`^(docker\.io/library/)?postgres@sha256:[0-9a-f]{64}$`).MatchString(image) {
		t.Fatal("selected PostgreSQL gate requires AGENOVA_MEMORY_POSTGRES_IMAGE with an official image digest")
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal("cannot allocate isolated database test identity")
	}
	d := &sqlFixture{t: t}
	d.id = d.docker("", "create", "--pull=never", "--name", "agenova-e17-sql-"+hex.EncodeToString(suffix[:]),
		"--label", "io.agenova.ticket=179", "--network", "none", "--env", "POSTGRES_HOST_AUTH_METHOD=trust",
		image, "-c", "listen_addresses=", "-c", "log_statement=none")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(d.id) {
		t.Fatal("Docker did not return an owned test container identity")
	}
	t.Cleanup(func() {
		// Only remove the ID created by this test, including its anonymous volume.
		d.docker("", "rm", "--force", "--volumes", d.id)
	})
	d.docker("", "start", d.id)
	d.waitReady()
	t.Logf("image=%s; server=%s; network=none; transport=container-local socket", image, d.sql("", "SHOW server_version"))
	d.sql("", `CREATE ROLE memory_owner NOLOGIN;
CREATE ROLE memory_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
CREATE ROLE memory_privileged LOGIN SUPERUSER;
CREATE ROLE memory_inherited NOLOGIN;
GRANT CREATE ON DATABASE postgres TO memory_owner;
SET ROLE memory_owner;
`+MigrationSQL()+`
RESET ROLE;
GRANT USAGE ON SCHEMA agenova_memory TO memory_app;
GRANT SELECT ON agenova_memory.schema_version TO memory_app;
GRANT SELECT, INSERT ON agenova_memory.records, agenova_memory.receipts TO memory_app;`)
	// Fresh synthetic passwords are sent only through private stdin, not argv,
	// logs or evidence. All credential files belong to this disposable container.
	var passwords strings.Builder
	for _, role := range []string{"memory_app", "memory_privileged"} {
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal("cannot allocate isolated database test credential")
		}
		password := hex.EncodeToString(secret[:])
		d.sql("", "SET password_encryption = 'scram-sha-256'; ALTER ROLE "+role+" PASSWORD '"+password+"'")
		passwords.WriteString("*:5432:postgres:" + role + ":" + password + "\n")
	}
	d.docker(passwords.String(), "exec", "--interactive", d.id, "sh", "-c", "umask 077; cat > /tmp/e17-pgpass")
	d.docker("local all memory_app,memory_privileged scram-sha-256\n", "exec", "--interactive", d.id, "sh", "-c", `cat > /tmp/e17-hba; cat "$PGDATA/pg_hba.conf" >> /tmp/e17-hba; cat /tmp/e17-hba > "$PGDATA/pg_hba.conf"; rm /tmp/e17-hba`)
	d.expect("", "SELECT pg_reload_conf()", "t")
	d.expect("memory_app", "SELECT current_user = 'memory_app' AND session_user = 'memory_app' AND system_user = 'scram-sha-256:memory_app'", "t")
	d.expect("memory_app", readinessSQL, "t|t|t|t|t")

	t.Run("authenticated identity rejects session authorization masking", func(t *testing.T) {
		for _, login := range []string{"", "memory_privileged"} {
			t.Run("login_"+login, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				identity := "system_user IS NULL"
				if login != "" {
					identity = "system_user = 'scram-sha-256:memory_privileged'"
				}
				fixture.expect(login, "SET SESSION AUTHORIZATION memory_app; SELECT current_user = session_user AND current_user = 'memory_app' AND "+identity+"; RESET SESSION AUTHORIZATION; SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname = current_user", "t\nt")
				flags := strings.Split(fixture.sql(login, "SET SESSION AUTHORIZATION memory_app; "+readinessSQL), "|")
				if len(flags) != 5 || flags[1] != "f" {
					t.Fatal("readiness accepted a privileged authenticated identity masked by session authorization")
				}
			})
		}
	})

	t.Run("unauthenticated data login rejects readiness", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		fixture.sql("", "CREATE ROLE memory_trusted LOGIN; GRANT memory_app TO memory_trusted WITH INHERIT TRUE, SET FALSE")
		t.Cleanup(func() { fixture.sql("", "DROP ROLE memory_trusted") })
		fixture.expect("memory_trusted", "SELECT current_user = session_user AND current_user = 'memory_trusted' AND system_user IS NULL", "t")
		flags := strings.Split(fixture.sql("memory_trusted", readinessSQL), "|")
		if len(flags) != 5 || flags[1] != "f" {
			t.Fatal("readiness accepted a data-only login without authenticated identity proof")
		}
	})

	t.Run("masked privileged session rejects readiness", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		fixture.expect("", "SET ROLE memory_app; SELECT current_user = 'memory_app' AND session_user = 'postgres'", "t")
		flags := strings.Split(fixture.sql("", "SET ROLE memory_app; "+readinessSQL), "|")
		if len(flags) != 5 || flags[1] != "f" {
			t.Fatal("readiness accepted a privileged login masked by a data-only current role")
		}
	})

	t.Run("non inheriting SET roles reject readiness", func(t *testing.T) {
		d.sql("", `CREATE ROLE memory_hidden NOLOGIN; CREATE ROLE memory_bridge NOLOGIN;
GRANT USAGE ON SCHEMA agenova_memory TO memory_hidden;
GRANT SELECT, TRUNCATE ON agenova_memory.records, agenova_memory.receipts TO memory_hidden;`)
		for _, member := range []string{"memory_app", "memory_bridge"} {
			t.Run(member, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				fixture.sql("", "GRANT memory_hidden TO "+member+" WITH INHERIT FALSE, SET TRUE;")
				if member == "memory_bridge" {
					fixture.sql("", "GRANT memory_bridge TO memory_app WITH INHERIT TRUE, SET TRUE;")
				}
				t.Cleanup(func() {
					fixture.sql("", "REVOKE memory_hidden FROM "+member+";")
					if member == "memory_bridge" {
						fixture.sql("", "REVOKE memory_bridge FROM memory_app;")
					}
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect("memory_app", "SELECT has_table_privilege(current_user, 'agenova_memory.records', 'TRUNCATE'), pg_has_role(current_user, 'memory_hidden', 'SET'), pg_has_role(current_user, 'memory_hidden', 'USAGE')", "f|t|f")
				// This connection authenticates as the application role itself.
				// Rollback preserves data after proving native SET/TRUNCATE access.
				fixture.expect("memory_app", `BEGIN;
SET ROLE memory_hidden;
TRUNCATE agenova_memory.records, agenova_memory.receipts;
SELECT current_user = 'memory_hidden' AND session_user = 'memory_app';
ROLLBACK;`, "t")
				fixture.expectUnsafeFlag(1)
			})
		}
		d.sql("", "GRANT memory_hidden TO memory_app WITH INHERIT FALSE, SET FALSE;")
		d.expect("memory_app", readinessSQL, "t|t|t|t|t")
		d.sql("", "REVOKE memory_hidden FROM memory_app; REVOKE TRUNCATE ON agenova_memory.records, agenova_memory.receipts FROM memory_hidden; GRANT memory_hidden TO memory_app WITH INHERIT TRUE, SET TRUE;")
		d.expect("memory_app", readinessSQL, "t|t|t|t|t")
		d.sql("", "REVOKE memory_hidden FROM memory_app;")
	})

	t.Run("grant options reject readiness", func(t *testing.T) {
		for _, role := range []string{"memory_app", "memory_inherited"} {
			for _, object := range []struct{ name, column string }{{"records", "body"}, {"receipts", "request_digest"}, {"schema_version", "version"}} {
				privileges := []string{"SELECT", "INSERT"}
				if object.name == "schema_version" {
					privileges = privileges[:1]
				}
				for _, privilege := range privileges {
					for _, columnGrant := range []bool{false, true} {
						grant := privilege
						if columnGrant {
							grant += " (" + object.column + ")"
						}
						t.Run(object.name+"/"+grant+"/"+role, func(t *testing.T) {
							fixture := &sqlFixture{t: t, id: d.id}
							if role == "memory_inherited" {
								fixture.sql("", "GRANT memory_inherited TO memory_app;")
							}
							fixture.sql("", "GRANT "+grant+" ON agenova_memory."+object.name+" TO "+role+" WITH GRANT OPTION;")
							t.Cleanup(func() {
								fixture.sql("", "REVOKE GRANT OPTION FOR "+grant+" ON agenova_memory."+object.name+" FROM "+role+" CASCADE;")
								if role == "memory_inherited" {
									fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
								}
								fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
							})
							fixture.expect("memory_app", "SELECT has_any_column_privilege(current_user, 'agenova_memory."+object.name+"', '"+privilege+" WITH GRANT OPTION')", "t")
							index := 3
							if object.name == "schema_version" {
								index = 4
							}
							fixture.expectUnsafeFlag(index)
						})
					}
				}
			}
			t.Run("schema USAGE/"+role, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				if role == "memory_inherited" {
					fixture.sql("", "GRANT memory_inherited TO memory_app;")
				}
				fixture.sql("", "GRANT USAGE ON SCHEMA agenova_memory TO "+role+" WITH GRANT OPTION;")
				t.Cleanup(func() {
					fixture.sql("", "REVOKE GRANT OPTION FOR USAGE ON SCHEMA agenova_memory FROM "+role+" CASCADE;")
					if role == "memory_inherited" {
						fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
					}
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect("memory_app", "SELECT has_schema_privilege(current_user, 'agenova_memory', 'USAGE WITH GRANT OPTION')", "t")
				fixture.expectUnsafeFlag(2)
			})
		}
	})

	t.Run("role membership administration rejects readiness", func(t *testing.T) {
		d.sql("", "CREATE ROLE memory_delegate NOLOGIN;")
		for _, member := range []string{"memory_app", "memory_delegate"} {
			t.Run(member, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				fixture.sql("", "GRANT memory_inherited TO "+member+" WITH ADMIN OPTION;")
				if member == "memory_delegate" {
					fixture.sql("", "GRANT memory_delegate TO memory_app;")
				}
				t.Cleanup(func() {
					fixture.sql("", "REVOKE memory_inherited FROM "+member+";")
					if member == "memory_delegate" {
						fixture.sql("", "REVOKE memory_delegate FROM memory_app;")
					}
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect(member, "SELECT pg_has_role(current_user, 'memory_inherited', 'MEMBER WITH ADMIN OPTION')", "t")
				fixture.expect("memory_app", "SELECT pg_has_role(current_user, '"+member+"', 'MEMBER')", "t")
				fixture.expectUnsafeFlag(1)
			})
		}
	})

	t.Run("administrative roles reject readiness", func(t *testing.T) {
		for _, attribute := range []string{"SUPERUSER", "BYPASSRLS", "CREATEROLE", "CREATEDB", "REPLICATION"} {
			for _, role := range []string{"memory_app", "memory_inherited"} {
				t.Run(attribute+"/"+role, func(t *testing.T) {
					fixture := &sqlFixture{t: t, id: d.id}
					fixture.sql("", "ALTER ROLE "+role+" "+attribute+";")
					if role == "memory_inherited" {
						fixture.sql("", "GRANT memory_inherited TO memory_app;")
					}
					t.Cleanup(func() {
						fixture.sql("", "ALTER ROLE "+role+" NO"+attribute+";")
						if role == "memory_inherited" {
							fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
						}
						fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
					})
					column := "rol" + strings.ToLower(attribute)
					if attribute == "SUPERUSER" {
						column = "rolsuper"
					}
					fixture.expect("", "SELECT "+column+" FROM pg_catalog.pg_roles WHERE rolname='"+role+"'", "t")
					if role == "memory_inherited" {
						fixture.expect("memory_app", "SELECT pg_has_role(current_user, 'memory_inherited', 'MEMBER')", "t")
					}
					fixture.expectUnsafeFlag(1)
				})
			}
		}
	})

	t.Run("predefined server roles reject readiness", func(t *testing.T) {
		for _, role := range []string{"pg_read_server_files", "pg_write_server_files", "pg_execute_server_program", "pg_monitor", "pg_read_all_data", "pg_write_all_data"} {
			t.Run(role, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				fixture.sql("", "GRANT "+role+" TO memory_inherited; GRANT memory_inherited TO memory_app;")
				t.Cleanup(func() {
					fixture.sql("", "REVOKE memory_inherited FROM memory_app; REVOKE "+role+" FROM memory_inherited;")
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect("memory_app", "SELECT pg_has_role(current_user, '"+role+"', 'MEMBER')", "t")
				fixture.expectUnsafeFlag(1)
			})
		}
	})

	t.Run("schema and database owners reject readiness", func(t *testing.T) {
		for _, role := range []string{"memory_app", "memory_inherited"} {
			for _, object := range []string{"schema", "database"} {
				t.Run(object+"/"+role, func(t *testing.T) {
					fixture := &sqlFixture{t: t, id: d.id}
					if role == "memory_inherited" {
						fixture.sql("", "GRANT memory_inherited TO memory_app;")
					}
					if object == "schema" {
						fixture.sql("", "ALTER SCHEMA agenova_memory OWNER TO "+role+"; REVOKE CREATE ON SCHEMA agenova_memory FROM "+role+";")
					} else {
						fixture.sql("", "ALTER DATABASE postgres OWNER TO "+role+";")
					}
					t.Cleanup(func() {
						if object == "schema" {
							fixture.sql("", "ALTER SCHEMA agenova_memory OWNER TO memory_owner; GRANT CREATE ON SCHEMA agenova_memory TO memory_owner; GRANT USAGE ON SCHEMA agenova_memory TO memory_app;")
						} else {
							fixture.sql("", "ALTER DATABASE postgres OWNER TO postgres;")
						}
						if role == "memory_inherited" {
							fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
						}
						fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
					})
					if object == "schema" {
						fixture.expect("memory_app", "SELECT has_schema_privilege(current_user, n.oid, 'CREATE'), pg_has_role(current_user, n.nspowner, 'MEMBER') FROM pg_catalog.pg_namespace n WHERE n.nspname='agenova_memory'", "f|t")
					} else {
						fixture.expect("memory_app", "SELECT pg_has_role(current_user, d.datdba, 'MEMBER') FROM pg_catalog.pg_database d WHERE d.datname=current_database()", "t")
					}
					fixture.expectUnsafeFlag(2)
				})
			}
		}
	})

	t.Run("database CREATE rejects readiness", func(t *testing.T) {
		for _, grantee := range []string{"memory_app", "memory_inherited", "PUBLIC"} {
			t.Run(grantee, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				if grantee == "memory_inherited" {
					fixture.sql("", "GRANT memory_inherited TO memory_app;")
				}
				fixture.sql("", "GRANT CREATE ON DATABASE postgres TO "+grantee+";")
				t.Cleanup(func() {
					fixture.sql("", "REVOKE CREATE ON DATABASE postgres FROM "+grantee+";")
					if grantee == "memory_inherited" {
						fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
					}
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect("memory_app", "SELECT has_database_privilege(current_user, current_database(), 'CREATE')", "t")
				fixture.expectUnsafeFlag(2)
			})
		}
	})

	t.Run("unexpected table privileges reject readiness", func(t *testing.T) {
		for _, object := range []string{"records", "receipts", "schema_version"} {
			for _, privilege := range []string{"TRIGGER", "REFERENCES", "MAINTAIN"} {
				for _, grantee := range []string{"memory_app", "memory_inherited", "PUBLIC"} {
					t.Run(object+"/"+privilege+"/"+grantee, func(t *testing.T) {
						fixture := &sqlFixture{t: t, id: d.id}
						if grantee == "memory_inherited" {
							fixture.sql("", "GRANT memory_inherited TO memory_app;")
						}
						fixture.sql("", "GRANT "+privilege+" ON agenova_memory."+object+" TO "+grantee+";")
						t.Cleanup(func() {
							fixture.sql("", "REVOKE "+privilege+" ON agenova_memory."+object+" FROM "+grantee+";")
							if grantee == "memory_inherited" {
								fixture.sql("", "REVOKE memory_inherited FROM memory_app;")
							}
							fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
						})
						fixture.expect("memory_app", "SELECT has_table_privilege(current_user, 'agenova_memory."+object+"', '"+privilege+"')", "t")
						expected := "t|t|t|f|t"
						if object == "schema_version" {
							expected = "t|t|t|t|f"
						}
						fixture.expect("memory_app", readinessSQL, expected)
					})
				}
			}
		}
	})

	t.Run("column references reject readiness", func(t *testing.T) {
		for _, tc := range []struct{ object, column string }{{"records", "source_claim"}, {"receipts", "memory_id"}, {"schema_version", "version"}} {
			t.Run(tc.object, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				fixture.sql("", "GRANT REFERENCES ("+tc.column+") ON agenova_memory."+tc.object+" TO memory_app;")
				t.Cleanup(func() {
					fixture.sql("", "REVOKE REFERENCES ("+tc.column+") ON agenova_memory."+tc.object+" FROM memory_app;")
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				fixture.expect("memory_app", "SELECT has_table_privilege(current_user, 'agenova_memory."+tc.object+"', 'REFERENCES'), has_any_column_privilege(current_user, 'agenova_memory."+tc.object+"', 'REFERENCES')", "f|t")
				expected := "t|t|t|f|t"
				if tc.object == "schema_version" {
					expected = "t|t|t|t|f"
				}
				fixture.expect("memory_app", readinessSQL, expected)
			})
		}
	})

	t.Run("column privileges reject readiness", func(t *testing.T) {
		for _, tc := range []struct {
			name, object, column, grantee, setup, reset string
		}{
			{name: "record body", object: "records", column: "body", grantee: "memory_app"},
			{name: "receipt digest", object: "receipts", column: "request_digest", grantee: "memory_app"},
			{name: "record inherited", object: "records", column: "source_claim", grantee: "memory_inherited", setup: "GRANT memory_inherited TO memory_app;", reset: "REVOKE memory_inherited FROM memory_app;"},
			{name: "receipt public", object: "receipts", column: "memory_id", grantee: "PUBLIC"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				fixture := &sqlFixture{t: t, id: d.id}
				fixture.sql("", tc.setup+"GRANT UPDATE ("+tc.column+") ON agenova_memory."+tc.object+" TO "+tc.grantee+";")
				t.Cleanup(func() {
					fixture.sql("", "REVOKE UPDATE ("+tc.column+") ON agenova_memory."+tc.object+" FROM "+tc.grantee+";"+tc.reset)
					fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
				})
				// The independent oracle reproduces why table-only checks miss this.
				fixture.expect("memory_app", "SELECT has_table_privilege(current_user, 'agenova_memory."+tc.object+"', 'UPDATE'), has_any_column_privilege(current_user, 'agenova_memory."+tc.object+"', 'UPDATE')", "f|t")
				fixture.expect("memory_app", readinessSQL, "t|t|t|f|t")
			})
		}
	})

	t.Run("marker privileges reject readiness", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		for _, operation := range []string{"UPDATE", "INSERT"} {
			fixture.sql("", "GRANT "+operation+" (version) ON agenova_memory.schema_version TO memory_app;")
			fixture.expect("memory_app", readinessSQL, "t|t|t|t|f")
			fixture.sql("", "REVOKE "+operation+" (version) ON agenova_memory.schema_version FROM memory_app;")
			fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
		}
	})

	t.Run("RLS and transaction context", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		fixture.expect("memory_app", `
SELECT (SELECT count(*) FROM agenova_memory.records) = 0 AND (SELECT count(*) FROM agenova_memory.receipts) = 0;
BEGIN;
`+sqlNamespace("team-a", "payments", "team-docs")+`
INSERT INTO agenova_memory.records (team, project, scope, id, body, source_claim)
 VALUES ('team-a', 'payments', 'team-docs', 'memory:00000000000000000000000000000001', 'campaign_%_synthetic', 'claim:sql-a');
INSERT INTO agenova_memory.receipts (team, project, scope, invocation_id, request_digest, memory_id)
 VALUES ('team-a', 'payments', 'team-docs', 'invocation:sql-a', repeat('0', 64), 'memory:00000000000000000000000000000001');
SELECT (SELECT count(*) FROM agenova_memory.records) = 1 AND (SELECT count(*) FROM agenova_memory.receipts) = 1;
COMMIT;
SELECT (SELECT count(*) FROM agenova_memory.records) = 0 AND (SELECT count(*) FROM agenova_memory.receipts) = 0;
BEGIN;
`+sqlNamespace("team-b", "payments", "team-docs")+`
SELECT (SELECT count(*) FROM agenova_memory.records) = 0 AND (SELECT count(*) FROM agenova_memory.receipts) = 0;
DO $$ BEGIN
 BEGIN
  INSERT INTO agenova_memory.records (team, project, scope, id, body, source_claim)
   VALUES ('team-a', 'payments', 'team-docs', 'memory:00000000000000000000000000000002', 'synthetic', 'claim:sql-b');
  RAISE EXCEPTION 'cross-namespace INSERT unexpectedly permitted';
 EXCEPTION WHEN insufficient_privilege THEN NULL;
 END;
END $$;
ROLLBACK;
BEGIN;
`+sqlNamespace("team-a", "other-project", "team-docs")+`
SELECT (SELECT count(*) FROM agenova_memory.records) = 0 AND (SELECT count(*) FROM agenova_memory.receipts) = 0;
ROLLBACK;
BEGIN;
`+sqlNamespace("team-a", "payments", "other-scope")+`
SELECT (SELECT count(*) FROM agenova_memory.records) = 0 AND (SELECT count(*) FROM agenova_memory.receipts) = 0;
ROLLBACK;`, "t\nt\nt\nt\nt\nt")
	})

	t.Run("literal search and durable receipt constraints", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		fixture.expect("memory_app", "BEGIN;"+sqlNamespace("team-a", "payments", "team-docs")+`
SELECT count(*) = 1 FROM agenova_memory.records WHERE strpos(lower(body), lower('CAMPAIGN_%_')) > 0;
SELECT count(*) = 0 FROM agenova_memory.records WHERE strpos(lower(body), lower('campaign__')) > 0;
DO $$ BEGIN
 BEGIN
  INSERT INTO agenova_memory.receipts (team, project, scope, invocation_id, request_digest, memory_id)
   VALUES ('team-a', 'payments', 'team-docs', 'invocation:sql-a', repeat('1', 64), 'memory:00000000000000000000000000000001');
  RAISE EXCEPTION 'duplicate receipt unexpectedly permitted';
 EXCEPTION WHEN unique_violation THEN NULL;
 END;
 BEGIN
  INSERT INTO agenova_memory.receipts (team, project, scope, invocation_id, request_digest, memory_id)
   VALUES ('team-a', 'payments', 'team-docs', 'invocation:missing-record', repeat('1', 64), 'memory:00000000000000000000000000000003');
  RAISE EXCEPTION 'orphan receipt unexpectedly permitted';
 EXCEPTION WHEN foreign_key_violation THEN NULL;
 END;
END $$;
SELECT count(*) = 1 FROM agenova_memory.receipts;
COMMIT;
BEGIN READ ONLY;
`+sqlNamespace("team-a", "payments", "team-docs")+`
DO $$ BEGIN
 BEGIN
  INSERT INTO agenova_memory.records (team, project, scope, id, body, source_claim)
   VALUES ('team-a', 'payments', 'team-docs', 'memory:00000000000000000000000000000004', 'synthetic', 'claim:sql-a');
  RAISE EXCEPTION 'read-only INSERT unexpectedly permitted';
 EXCEPTION WHEN read_only_sql_transaction THEN NULL;
 END;
END $$;
ROLLBACK;`, "t\nt\nt")
	})

	t.Run("database restart retains rows and receipts", func(t *testing.T) {
		fixture := &sqlFixture{t: t, id: d.id}
		volumeTemplate := "{{range .Mounts}}{{if eq .Destination \"/var/lib/postgresql\"}}{{.Name}}{{end}}{{end}}"
		volume := fixture.docker("", "inspect", "--format", volumeTemplate, fixture.id)
		if volume == "" {
			t.Fatal("restart gate requires an identified persistent test volume")
		}
		fixture.docker("", "restart", "--time", "10", fixture.id)
		fixture.waitReady()
		if fixture.docker("", "inspect", "--format", volumeTemplate, fixture.id) != volume {
			t.Fatal("database restart changed the storage identity")
		}
		fixture.expect("memory_app", readinessSQL, "t|t|t|t|t")
		fixture.expect("memory_app", "BEGIN;"+sqlNamespace("team-a", "payments", "team-docs")+`
SELECT (SELECT count(*) FROM agenova_memory.records WHERE strpos(body, 'campaign_%_') > 0) = 1
 AND (SELECT count(*) FROM agenova_memory.receipts WHERE invocation_id = 'invocation:sql-a') = 1;
COMMIT;`, "t")
	})
}

// Values in this fixture are fixed synthetic operator inputs, never worker input.
func sqlNamespace(team, project, scope string) string {
	return "DO $$ BEGIN PERFORM set_config('agenova.team', '" + team + "', true); PERFORM set_config('agenova.project', '" + project + "', true); PERFORM set_config('agenova.scope', '" + scope + "', true); END $$;"
}

type sqlFixture struct {
	t  *testing.T
	id string
}

func (d *sqlFixture) docker(input string, args ...string) string {
	d.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		// Do not publish SQL text, bodies, raw server errors, or command output.
		d.t.Fatalf("isolated PostgreSQL %s command failed", args[0])
	}
	return strings.TrimSpace(string(output))
}

func (d *sqlFixture) sql(role, statement string) string {
	d.t.Helper()
	login := role
	if login == "" {
		login = "postgres"
	} else if role != "memory_app" && role != "memory_privileged" && role != "memory_trusted" {
		// Group-role oracles are operator sessions, never readiness positives.
		login = "postgres"
		statement = "SET SESSION AUTHORIZATION " + role + ";" + statement
	}
	return d.docker(statement+";", "exec", "--interactive", "--env", "PGPASSFILE=/tmp/e17-pgpass", d.id, "psql", "--username="+login, "--dbname=postgres", "--no-password", "--no-psqlrc", "--quiet", "--tuples-only", "--no-align", "--set=ON_ERROR_STOP=1")
}

func (d *sqlFixture) expect(role, statement, expected string) {
	d.t.Helper()
	if d.sql(role, statement) != expected {
		d.t.Fatal("live SQL assertion failed; private result withheld")
	}
}

func (d *sqlFixture) expectUnsafeFlag(index int) {
	d.t.Helper()
	flags := strings.Split(d.sql("memory_app", readinessSQL), "|")
	if len(flags) != 5 || flags[index] != "f" {
		d.t.Fatal("live readiness accepted an unsafe application role or privilege")
	}
}

func (d *sqlFixture) waitReady() {
	d.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		// The entrypoint briefly starts a temporary server during initialization.
		// Wait for the final postgres process to own PID 1 before admitting SQL.
		process, err := exec.CommandContext(ctx, "docker", "exec", d.id, "cat", "/proc/1/comm").Output()
		cmd := exec.CommandContext(ctx, "docker", "exec", d.id, "pg_isready", "--username=postgres", "--dbname=postgres", "--quiet")
		ready := err == nil && strings.TrimSpace(string(process)) == "postgres" && cmd.Run() == nil
		cancel()
		if ready {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	d.t.Fatal("isolated PostgreSQL did not become ready within 30 seconds")
}
