// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func rawChecksum(script string) string {
	sum := sha256.Sum256([]byte(script))
	return hex.EncodeToString(sum[:])
}

func TestMigrationChecksumOnlyNormalizesLineEndings(t *testing.T) {
	script := "CREATE TABLE a (id int);\n-- immutable\n"
	legacy := strings.ReplaceAll(script, "\n", "\r\n")
	if migrationChecksum(script) != migrationChecksum(legacy) || !migrationChecksumMatches(1, script, rawChecksum(legacy)) || !migrationChecksumMatches(11, legacy, rawChecksum(script)) {
		t.Fatal("legacy/new line endings incompatible")
	}
	for _, changed := range []string{strings.Replace(script, "int", "text", 1), script + "\n", strings.Replace(script, "immutable", "edited", 1), strings.Replace(script, "id int", "id  int", 1)} {
		if migrationChecksumMatches(1, changed, rawChecksum(script)) || migrationChecksumMatches(1, changed, rawChecksum(legacy)) {
			t.Fatal("non-line-ending edit accepted")
		}
	}
	if migrationChecksumMatches(12, script, rawChecksum(legacy)) {
		t.Fatal("legacy compatibility leaked into future versions")
	}
}

func TestLegacyMigrationChecksumsStayImmutableAfterUpgrade(t *testing.T) {
	db := invitationDB(t)
	ctx := context.Background()
	legacy := map[int]string{}
	for i, script := range migrations {
		legacy[i+1] = rawChecksum(strings.ReplaceAll(strings.ReplaceAll(script, "\r\n", "\n"), "\n", "\r\n"))
		if _, err := db.Exec(`UPDATE agenova_schema_migrations SET checksum=$1 WHERE version=$2`, legacy[i+1], i+1); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckMigration(ctx, db); err != nil {
		t.Fatal("legacy startup", err)
	}
	// Recreate the actual released version-9 boundary in this disposable schema.
	if _, err := db.Exec(`DROP TABLE agenova_policy_activations; DROP TABLE agenova_organization_metadata; DELETE FROM agenova_schema_migrations WHERE version>9`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal("legacy additive 9-to-11 upgrade", err)
	}
	if err := CheckMigration(ctx, db); err != nil {
		t.Fatal("upgraded startup", err)
	}
	for version, expected := range legacy {
		if version > 9 {
			expected = migrationChecksum(migrations[version-1])
		}
		var actual string
		if db.QueryRow(`SELECT checksum FROM agenova_schema_migrations WHERE version=$1`, version).Scan(&actual) != nil || actual != expected {
			t.Fatal("applied checksum rewritten", version)
		}
	}
	if _, err := db.Exec(`UPDATE agenova_schema_migrations SET checksum='unrecognized' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if CheckMigration(ctx, db) == nil || Migrate(ctx, db) == nil {
		t.Fatal("meaningful checksum drift accepted")
	}
}
