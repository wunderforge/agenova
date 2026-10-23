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

func TestMigrationBaselineIsReadOnlyAndDistinguishesDrift(t *testing.T) {
	db := invitationDB(t)
	ctx := context.Background()
	baseline, err := MigrationBaseline(ctx, db)
	if err != nil || len(baseline["exact"])+len(baseline["lineEndingOnly"]) != len(migrations) || len(baseline["different"]) != 0 {
		t.Fatal(baseline, err)
	}
	variant := strings.ReplaceAll(migrations[0], "\r\n", "\n")
	if variant == migrations[0] {
		variant = strings.ReplaceAll(variant, "\n", "\r\n")
	}
	sum := sha256.Sum256([]byte(variant))
	checksum := hex.EncodeToString(sum[:])
	if _, err := db.Exec(`UPDATE agenova_schema_migrations SET checksum=$1 WHERE version=1`, checksum); err != nil {
		t.Fatal(err)
	}
	baseline, err = MigrationBaseline(ctx, db)
	if err != nil || len(baseline["lineEndingOnly"]) == 0 || baseline["lineEndingOnly"][0] != 1 || len(baseline["different"]) != 0 {
		t.Fatal(baseline, err)
	}
	var unchanged string
	if db.QueryRow(`SELECT checksum FROM agenova_schema_migrations WHERE version=1`).Scan(&unchanged) != nil || unchanged != checksum {
		t.Fatal("inspection rewrote applied history")
	}
	if _, err := db.Exec(`UPDATE agenova_schema_migrations SET checksum='different' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	baseline, err = MigrationBaseline(ctx, db)
	if err != nil || len(baseline["different"]) != 1 || baseline["different"][0] != 1 {
		t.Fatal(baseline, err)
	}
}
