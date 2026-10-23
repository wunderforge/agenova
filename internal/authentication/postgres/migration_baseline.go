// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

// MigrationBaseline reports only version classifications, not stored hashes or
// SQL. It never accepts a checksum, changes a script or commits a migration.
func MigrationBaseline(ctx context.Context, db *sql.DB) (map[string][]int, error) {
	out := map[string][]int{"exact": {}, "lineEndingOnly": {}, "different": {}}
	rows, err := db.QueryContext(ctx, `SELECT version,checksum FROM agenova_schema_migrations ORDER BY version`)
	if err != nil {
		return nil, errors.New("migration baseline unavailable")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version int
		var checksum string
		if rows.Scan(&version, &checksum) != nil || version != count+1 || version > len(migrations) {
			return nil, errors.New("migration baseline invalid")
		}
		count++
		key := "different"
		for _, variant := range []string{migrations[version-1], strings.ReplaceAll(migrations[version-1], "\r\n", "\n"), strings.ReplaceAll(strings.ReplaceAll(migrations[version-1], "\r\n", "\n"), "\n", "\r\n")} {
			sum := sha256.Sum256([]byte(variant))
			if checksum == hex.EncodeToString(sum[:]) {
				key = "lineEndingOnly"
				if variant == migrations[version-1] {
					key = "exact"
				}
				break
			}
		}
		out[key] = append(out[key], version)
	}
	if rows.Err() != nil {
		return nil, errors.New("migration baseline unavailable")
	}
	return out, nil
}
