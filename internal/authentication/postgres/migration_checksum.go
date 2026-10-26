// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// New migration records are independent of Windows checkout line endings.
// Nothing else (including whitespace, comments and SQL text) is normalized.
func migrationChecksum(script string) string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(script, "\r\n", "\n")))
	return hex.EncodeToString(sum[:])
}

func migrationChecksumMatches(version int, script, stored string) bool {
	if stored == migrationChecksum(script) {
		return true
	}
	// Versions 1-11 predate canonical LF checksums. Recognize only their exact
	// CRLF representation, without rewriting history or accepting other edits.
	if version < 1 || version > 11 {
		return false
	}
	legacy := strings.ReplaceAll(strings.ReplaceAll(script, "\r\n", "\n"), "\n", "\r\n")
	sum := sha256.Sum256([]byte(legacy))
	return stored == hex.EncodeToString(sum[:])
}
