// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package workloadidentity issues bounded Agenova worker credentials. A token
// authenticates a worker to Agenova Gateways; it is never an AWS credential.
package workloadidentity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const Issuer = "agenova-workload"

var ErrInvalid = errors.New("invalid workload identity")

type Identity struct {
	ClaimID  string `json:"claimId"`
	WorkerID string `json:"workerId"`
	Audience string `json:"aud"`
	Issuer   string `json:"iss"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
	ID       string `json:"jti"`
}

type Service struct {
	key     []byte
	now     func() time.Time
	mu      sync.RWMutex
	revoked map[string]struct{}
}

// New requires a server-owned key of at least 256 bits. The key is copied and
// must not be exposed through worker configuration, evidence, or diagnostics.
func New(key []byte, now func() time.Time) (*Service, error) {
	if len(key) < 32 {
		return nil, errors.New("workload identity key must contain at least 256 bits")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{key: append([]byte(nil), key...), now: now, revoked: make(map[string]struct{})}, nil
}

func (s *Service) Issue(claimID, workerID, audience string, deadline time.Time) (string, error) {
	if s == nil || claimID == "" || workerID == "" || !validAudience(audience) {
		return "", ErrInvalid
	}
	now := s.now()
	if !deadline.After(now) {
		return "", ErrInvalid
	}
	expiry := now.Add(5 * time.Minute)
	if deadline.Before(expiry) {
		expiry = deadline
	}
	if expiry.Unix() <= now.Unix() {
		return "", ErrInvalid
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", errors.New("workload identity entropy unavailable")
	}
	identity := Identity{ClaimID: claimID, WorkerID: workerID, Audience: audience, Issuer: Issuer, IssuedAt: now.Unix(), Expires: expiry.Unix(), ID: hex.EncodeToString(nonce[:])}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(identity)
	if err != nil {
		return "", ErrInvalid
	}
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + s.sign(unsigned), nil
}

func (s *Service) Verify(token, audience string) (Identity, error) {
	if s == nil || !validAudience(audience) || len(token) > 8192 {
		return Identity{}, ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) {
		return Identity{}, ErrInvalid
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(unsigned))) {
		return Identity{}, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, ErrInvalid
	}
	var identity Identity
	if err := json.Unmarshal(payload, &identity); err != nil || identity.ClaimID == "" || identity.WorkerID == "" || identity.ID == "" || identity.Audience != audience || identity.Issuer != Issuer || identity.Expires <= identity.IssuedAt || s.now().Unix() >= identity.Expires || s.now().Unix() < identity.IssuedAt {
		return Identity{}, ErrInvalid
	}
	s.mu.RLock()
	_, revoked := s.revoked[identity.ID]
	s.mu.RUnlock()
	if revoked {
		return Identity{}, ErrInvalid
	}
	return identity, nil
}

// Revoke invalidates one valid credential before its expiry. It never logs or
// returns the token. Terminal claim state is checked separately at each call.
func (s *Service) Revoke(token, audience string) error {
	identity, err := s.Verify(token, audience)
	if err != nil {
		return ErrInvalid
	}
	s.mu.Lock()
	s.revoked[identity.ID] = struct{}{}
	s.mu.Unlock()
	return nil
}

func (s *Service) sign(unsigned string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validAudience(audience string) bool {
	return audience == "tool-gateway" || audience == "model-gateway"
}
