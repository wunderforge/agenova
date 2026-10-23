// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Trusted, short-lived operator job. Never used by the API or an Agent image.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/lib/pq"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
)

var roleName = regexp.MustCompile(`^agenova_[a-z0-9_]{1,48}$`)

func main() {
	apply := flag.Bool("apply", false, "apply the approved additive schema/grants")
	role := flag.String("role", "", "exact previously inspected runtime role")
	flag.Parse()
	if err := run(*apply, *role); err != nil {
		// Deliberately never format database, URL, provider or file errors.
		fmt.Fprintln(os.Stderr, "operator failed: "+err.Error()) // run returns only fixed developer-authored categories.
		os.Exit(1)
	}
}

func grantSQL(role string) (string, error) {
	if !roleName.MatchString(role) || role == "agenova_operator" {
		return "", errors.New("invalid runtime role")
	}
	name := pq.QuoteIdentifier(role)
	return `GRANT SELECT,INSERT,UPDATE ON agenova_organization_metadata TO ` + name + `;
GRANT SELECT,INSERT ON agenova_policy_activations TO ` + name + `;
GRANT USAGE ON SEQUENCE agenova_policy_activations_event_id_seq TO ` + name, nil
}

func run(apply bool, role string) error {
	var credentials struct{ Username, Password string }
	raw, err := os.ReadFile("/run/operator/credentials.json")
	if err != nil || len(raw) > 32<<10 || json.Unmarshal(raw, &credentials) != nil || credentials.Username != "agenova_operator" || credentials.Password == "" {
		return errors.New("operator credentials unavailable")
	}
	u := url.URL{Scheme: "postgres", Host: "agenova-production.cpm4eoa4ms54.ap-southeast-2.rds.amazonaws.com:5432", Path: "/agenova", User: url.UserPassword(credentials.Username, credentials.Password)}
	u.RawQuery = url.Values{"sslmode": {"verify-full"}, "sslrootcert": {"/etc/rds/rds-ca.pem"}, "connect_timeout": {"8"}}.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		return errors.New("database unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var database string
	if db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database) != nil || database != "agenova" {
		return errors.New("unexpected database")
	}
	var before, after int
	if db.QueryRowContext(ctx, "SELECT COALESCE(max(version),0) FROM agenova_schema_migrations").Scan(&before) != nil || (before != 9 && before != 11) {
		return errors.New("unexpected schema baseline")
	}
	rows, err := db.QueryContext(ctx, `SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname LIKE 'agenova_%' AND rolname <> current_user ORDER BY rolname`)
	if err != nil {
		return errors.New("runtime role inspection failed")
	}
	roles := []string{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) != nil {
			rows.Close()
			return errors.New("runtime role inspection failed")
		}
		roles = append(roles, name)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return errors.New("runtime role inspection failed")
	}
	if !apply {
		baseline, err := identitydb.MigrationBaseline(ctx, db)
		if err != nil {
			return err
		}
		checks := map[string]bool{}
		var value bool
		checks["runtimeRoleSafeQuery"] = db.QueryRowContext(ctx, `SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole FROM pg_roles WHERE rolname='agenova_runtime' AND rolcanlogin`).Scan(&value) == nil
		checks["runtimeRoleSafe"] = value
		value = false
		checks["existingReadQuery"] = db.QueryRowContext(ctx, `SELECT has_table_privilege('agenova_runtime','agenova_members','SELECT') AND has_table_privilege('agenova_runtime','agenova_project_policies','SELECT')`).Scan(&value) == nil
		checks["existingReads"] = value
		var count int
		checks["membersCountAvailable"] = db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_members").Scan(&count) == nil
		checks["projectsCountAvailable"] = db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_projects").Scan(&count) == nil
		for name, query := range map[string]string{
			"operatorSchemaCreate":     `SELECT has_schema_privilege(current_user,'public','CREATE')`,
			"operatorMigrationInsert":  `SELECT has_table_privilege(current_user,'agenova_schema_migrations','INSERT')`,
			"operatorPolicyReferences": `SELECT has_table_privilege(current_user,'agenova_project_policies','REFERENCES')`,
		} {
			value = false
			checks[name+"Query"] = db.QueryRowContext(ctx, query).Scan(&value) == nil
			checks[name] = value
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": "inspect", "schema": before, "runtimeRoles": roles, "checks": checks, "migrationBaseline": baseline})
	}
	statements, err := grantSQL(role)
	if err != nil {
		return err
	}
	var safe, existing bool
	if db.QueryRowContext(ctx, `SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole FROM pg_roles WHERE rolname=$1 AND rolcanlogin`, role).Scan(&safe) != nil || !safe || db.QueryRowContext(ctx, `SELECT has_table_privilege($1,'agenova_members','SELECT') AND has_table_privilege($1,'agenova_project_policies','SELECT')`, role).Scan(&existing) != nil || !existing {
		return errors.New("runtime role baseline not verified")
	}
	var members, projects, finalMembers, finalProjects int
	if db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_members").Scan(&members) != nil || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_projects").Scan(&projects) != nil {
		return errors.New("baseline counts unavailable")
	}
	if err = identitydb.Migrate(ctx, db); err != nil {
		return err // Migrate exposes only fixed categories, never database errors.
	}
	if _, err = db.ExecContext(ctx, statements); err != nil {
		return errors.New("runtime grants failed")
	}
	var privileges bool
	if db.QueryRowContext(ctx, `SELECT has_table_privilege($1,'agenova_organization_metadata','SELECT') AND has_table_privilege($1,'agenova_organization_metadata','INSERT') AND has_table_privilege($1,'agenova_organization_metadata','UPDATE') AND has_table_privilege($1,'agenova_policy_activations','SELECT') AND has_table_privilege($1,'agenova_policy_activations','INSERT') AND has_sequence_privilege($1,'agenova_policy_activations_event_id_seq','USAGE') AND NOT has_table_privilege($1,'agenova_organization_metadata','DELETE') AND NOT has_table_privilege($1,'agenova_policy_activations','DELETE') AND NOT has_schema_privilege($1,'public','CREATE')`, role).Scan(&privileges) != nil || !privileges {
		return errors.New("runtime privilege verification failed")
	}
	if db.QueryRowContext(ctx, "SELECT max(version) FROM agenova_schema_migrations").Scan(&after) != nil || after != 11 || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_members").Scan(&finalMembers) != nil || db.QueryRowContext(ctx, "SELECT count(*) FROM agenova_projects").Scan(&finalProjects) != nil || members != finalMembers || projects != finalProjects || identitydb.CheckMigration(ctx, db) != nil {
		return errors.New("migration postcondition failed")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": "apply", "schemaBefore": before, "schemaAfter": after, "runtimeRole": role, "minimalGrantsVerified": privileges, "memberAndProjectCountsUnchanged": true})
}
