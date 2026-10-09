// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package kubernetes keeps Secret API shapes inside the credential adapter.
package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/wunderforge/agenova/internal/credentials"
)

const (
	ResolverID   = "agenova.io/credential/kubernetes-secret"
	MaxJSONBytes = 2 << 20
)

var (
	dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	dataKey  = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,253}$`)
)

// Getter returns an owned, bounded JSON buffer for one exact Secret. It must
// honor context; no list, discovery, mutation or fallback is part of this API.
type Getter func(context.Context, string, string) ([]byte, error)

type Key struct{ Name, Key string }

type Resolver struct {
	namespace string
	allowed   map[Key]bool
	get       Getter
}

func New(namespace string, allowed []Key, get Getter) (*Resolver, error) {
	if !dnsLabel.MatchString(namespace) || len(allowed) == 0 || get == nil {
		return nil, credentials.ErrConfiguration
	}
	r := &Resolver{namespace: namespace, allowed: map[Key]bool{}, get: get}
	for _, key := range allowed {
		if !validName(key.Name) || !dataKey.MatchString(key.Key) || r.allowed[key] {
			return nil, credentials.ErrConfiguration
		}
		r.allowed[key] = true
	}
	return r, nil
}

func validName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if !dnsLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func (r *Resolver) Resolve(ctx context.Context, ref credentials.Reference) ([]byte, error) {
	if r == nil || ctx == nil || r.get == nil || ref.Resolver != ResolverID || !validName(ref.Name) || !dataKey.MatchString(ref.Key) || !r.allowed[Key{ref.Name, ref.Key}] {
		return nil, credentials.ErrRejected
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, credentials.CallTimeout)
	defer cancel()
	raw, err := r.get(ctx, r.namespace, ref.Name)
	defer clear(raw)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || len(raw) == 0 || len(raw) > MaxJSONBytes {
		return nil, credentials.ErrUnavailable
	}
	var secret struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
	if json.Unmarshal(raw, &secret) != nil || secret.APIVersion != "v1" || secret.Kind != "Secret" ||
		secret.Metadata.Name != ref.Name || secret.Metadata.Namespace != r.namespace || secret.Type != "Opaque" {
		return nil, credentials.ErrUnavailable
	}
	encoded, ok := secret.Data[ref.Key]
	if !ok || len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(credentials.MaxValueBytes) {
		return nil, credentials.ErrUnavailable
	}
	value, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(value) == 0 || len(value) > credentials.MaxValueBytes {
		clear(value)
		return nil, credentials.ErrUnavailable
	}
	return value, nil
}
