// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package websession keeps browser OAuth credentials behind a server-side
// session. Cookies contain only random handles; SQL sees encrypted payloads.
package websession

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
)

var errSession = errors.New("session unavailable")

type Store struct {
	DB   *sql.DB
	aead cipher.AEAD
}

func NewStore(db *sql.DB, key []byte) (*Store, error) {
	if db == nil || len(key) != 32 {
		return nil, errSession
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errSession
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errSession
	}
	return &Store{db, aead}, nil
}
func hash(handle string) string { h := sha256.Sum256([]byte(handle)); return hex.EncodeToString(h[:]) }
func (s *Store) seal(handle string, value any) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil || len(b) > 64<<10 {
		return nil, errSession
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errSession
	}
	return s.aead.Seal(nonce, nonce, b, []byte(handle)), nil
}
func (s *Store) open(handle string, b []byte, value any) error {
	n := s.aead.NonceSize()
	if len(b) < n || len(b) > 65<<10 {
		return errSession
	}
	plain, err := s.aead.Open(nil, b[:n], b[n:], []byte(handle))
	if err != nil || json.Unmarshal(plain, value) != nil {
		return errSession
	}
	return nil
}
func (s *Store) Create(ctx context.Context, handle string, value any, expiry time.Time) error {
	b, err := s.seal(handle, value)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errSession
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215002)`); err != nil {
		return errSession
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM agenova_browser_sessions WHERE expires_at<=now()`); err != nil {
		return errSession
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM agenova_browser_sessions`).Scan(&n); err != nil || n >= 2048 {
		return errSession
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_browser_sessions(id_hash,payload,expires_at) VALUES($1,$2,$3)`, hash(handle), b, expiry); err != nil {
		return errSession
	}
	if tx.Commit() != nil {
		return errSession
	}
	return nil
}

func (s *Store) Load(ctx context.Context, handle string, value any) (int64, error) {
	var b []byte
	var version int64
	err := s.DB.QueryRowContext(ctx, `SELECT payload,version FROM agenova_browser_sessions WHERE id_hash=$1 AND expires_at>now()`, hash(handle)).Scan(&b, &version)
	if err != nil {
		return 0, errSession
	}
	return version, s.open(handle, b, value)
}

// Save uses compare-and-swap. A concurrent logout cannot resurrect a session.
func (s *Store) Save(ctx context.Context, handle string, version int64, value any) error {
	b, err := s.seal(handle, value)
	if err != nil {
		return err
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE agenova_browser_sessions SET payload=$1,version=version+1 WHERE id_hash=$2 AND version=$3 AND expires_at>now()`, b, hash(handle), version)
	if err != nil {
		return errSession
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return errSession
	}
	return nil
}
func (s *Store) Delete(ctx context.Context, handle string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM agenova_browser_sessions WHERE id_hash=$1`, hash(handle))
	if err != nil {
		return errSession
	}
	return nil
}
func (s *Store) PutAttempt(ctx context.Context, handle string, value any) error {
	b, err := s.seal(handle, value)
	if err != nil {
		return err
	}
	// A hard shared cap prevents anonymous login-start requests growing storage.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errSession
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215001)`); err != nil {
		return errSession
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM agenova_login_attempts WHERE expires_at<=now()`); err != nil {
		return errSession
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM agenova_login_attempts`).Scan(&n); err != nil || n >= 128 {
		return errSession
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_login_attempts(id_hash,payload,expires_at) VALUES($1,$2,now()+interval '5 minutes')`, hash(handle), b); err != nil {
		return errSession
	}
	if tx.Commit() != nil {
		return errSession
	}
	return nil
}
func (s *Store) TakeAttempt(ctx context.Context, handle string, value any) error {
	var b []byte
	err := s.DB.QueryRowContext(ctx, `DELETE FROM agenova_login_attempts WHERE id_hash=$1 AND expires_at>now() RETURNING payload`, hash(handle)).Scan(&b)
	if err != nil {
		return errSession
	}
	return s.open(handle, b, value)
}
