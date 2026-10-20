// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package sqlite

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"github.com/wunderforge/agenova/internal/history"
)

// Cursor state is encrypted: raw global ordering can reveal hidden Work counts.
// This local key protects pagination privacy/integrity, never grants read access.
func (r *Repository) initializeCursor(version int) error {
	if version < 2 {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return history.ErrUnavailable
		}
		tx, err := r.db.Begin()
		if err != nil {
			return history.ErrUnavailable
		}
		defer tx.Rollback()
		if _, err = tx.Exec("CREATE TABLE history_settings(name TEXT PRIMARY KEY,value BLOB NOT NULL)"); err != nil {
			return history.ErrUnavailable
		}
		if _, err = tx.Exec("INSERT INTO history_settings(name,value) VALUES('cursor-key',?)", key); err != nil {
			return history.ErrUnavailable
		}
		if _, err = tx.Exec("PRAGMA user_version=2"); err != nil {
			return history.ErrUnavailable
		}
		if tx.Commit() != nil {
			return history.ErrUnavailable
		}
	}
	var key []byte
	if r.db.QueryRow("SELECT value FROM history_settings WHERE name='cursor-key'").Scan(&key) != nil {
		return history.ErrUnavailable
	}
	if len(key) != 32 {
		return history.ErrUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return history.ErrUnavailable
	}
	r.cursor, err = cipher.NewGCM(block)
	if err != nil {
		return history.ErrUnavailable
	}
	return nil
}
func (r *Repository) sealCursor(upper, before uint64) (string, error) {
	nonce := make([]byte, r.cursor.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", history.ErrUnavailable
	}
	payload := make([]byte, 16)
	binary.BigEndian.PutUint64(payload, upper)
	binary.BigEndian.PutUint64(payload[8:], before)
	return base64.RawURLEncoding.EncodeToString(r.cursor.Seal(nonce, nonce, payload, []byte("agenova-history-v1"))), nil
}
func (r *Repository) openCursor(value string) (uint64, uint64, error) {
	if len(value) > 128 {
		return 0, 0, history.ErrCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	n := r.cursor.NonceSize()
	if err != nil || len(raw) < n {
		return 0, 0, history.ErrCursor
	}
	payload, err := r.cursor.Open(nil, raw[:n], raw[n:], []byte("agenova-history-v1"))
	if err != nil || len(payload) != 16 {
		return 0, 0, history.ErrCursor
	}
	upper, before := binary.BigEndian.Uint64(payload), binary.BigEndian.Uint64(payload[8:])
	if upper == 0 || before == 0 || before > upper {
		return 0, 0, history.ErrCursor
	}
	return upper, before, nil
}
