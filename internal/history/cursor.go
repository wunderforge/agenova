// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package history

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
)

// SealPage and OpenPage protect pagination metadata, never read authority.
// Callers must reauthorize each page. Rotating the key invalidates old cursors.
func SealPage(key []byte, value any) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrUnavailable
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", ErrUnavailable
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", ErrUnavailable
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, raw, []byte("agenova-authenticated-history-v1"))), nil
}
func OpenPage(key []byte, token string, value any) error {
	if token == "" || len(token) > 512 {
		return ErrCursor
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return ErrUnavailable
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < aead.NonceSize() {
		return ErrCursor
	}
	raw, err = aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("agenova-authenticated-history-v1"))
	if err != nil || json.Unmarshal(raw, value) != nil {
		return ErrCursor
	}
	return nil
}
