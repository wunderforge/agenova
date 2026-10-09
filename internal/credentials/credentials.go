// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package credentials is a host-only resolution boundary, not claim authority.
package credentials

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxValueBytes = 64 << 10
	CallTimeout   = 5 * time.Second
)

var (
	resolverID       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.(?:[a-z0-9.-]+)/credential/[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
	resolverVersion  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-z0-9.-]+)?$`)
	ErrConfiguration = errors.New("credential resolver configuration invalid")
	ErrRejected      = errors.New("credential reference rejected")
	ErrUnavailable   = errors.New("credential resolution unavailable")
	ErrUse           = errors.New("credential use failed")
)

// Reference contains only operator-selected opaque metadata, never a value.
// It is not accepted from worker requests and does not confer any authority.
type Reference struct {
	Resolver string `json:"resolver" yaml:"resolver"`
	Name     string `json:"name" yaml:"name"`
	Key      string `json:"key" yaml:"key"`
}

func ValidateReference(ref Reference) error {
	if !validToken(ref.Resolver) || !resolverID.MatchString(ref.Resolver) || !validToken(ref.Name) || !validToken(ref.Key) {
		return ErrRejected
	}
	return nil
}

func validToken(s string) bool {
	return len(s) > 0 && len(s) <= 256 && utf8.ValidString(s) && strings.TrimSpace(s) == s &&
		!strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) })
}

// Resolver returns an owned buffer, which the caller clears. It must honor
// context and must not expose raw provider errors through other channels.
type Resolver interface {
	Resolve(context.Context, Reference) ([]byte, error)
}

type redacted struct{}

func (redacted) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[redacted]") }
func (redacted) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (redacted) MarshalYAML() (any, error)    { return "[redacted]", nil }

// Registration is trusted host configuration. No resolver is discovered or
// loaded dynamically; Allowed is an exact, defensively copied ceiling.
type Registration struct {
	redacted
	ID       string
	Version  string
	Resolver Resolver
	Allowed  []Reference
}

type Registry struct {
	redacted
	bindings map[Reference]Resolver
}

func New(registrations []Registration) (*Registry, error) {
	if len(registrations) == 0 {
		return nil, ErrConfiguration
	}
	r := &Registry{bindings: map[Reference]Resolver{}}
	ids := map[string]bool{}
	for _, registration := range registrations {
		if !validToken(registration.ID) || !resolverID.MatchString(registration.ID) || len(registration.Version) > 64 || !resolverVersion.MatchString(registration.Version) || nilResolver(registration.Resolver) || ids[registration.ID] || len(registration.Allowed) == 0 {
			return nil, ErrConfiguration
		}
		ids[registration.ID] = true
		for _, ref := range registration.Allowed {
			if ValidateReference(ref) != nil || ref.Resolver != registration.ID || r.bindings[ref] != nil {
				return nil, ErrConfiguration
			}
			r.bindings[ref] = registration.Resolver
		}
	}
	return r, nil
}

func nilResolver(r Resolver) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Binding can only be constructed by an explicit host registry. Use has no
// reference argument: worker input cannot retarget this captured handle.
type Binding struct {
	redacted
	ref      Reference
	resolver Resolver
}

func (r *Registry) Bind(ref Reference) (*Binding, error) {
	if r == nil || ValidateReference(ref) != nil {
		return nil, ErrRejected
	}
	resolver := r.bindings[ref]
	if resolver == nil {
		return nil, ErrRejected
	}
	return &Binding{ref: ref, resolver: resolver}, nil
}

// Use exposes bytes only to trusted adapter code during this callback. The
// adapter must honor context and must not copy bytes into public outputs.
// Clearing owned buffers is best-effort hygiene, not hostile-host isolation.
func (b *Binding) Use(ctx context.Context, consume func(context.Context, []byte) error) error {
	if b == nil || b.resolver == nil || ctx == nil || consume == nil {
		return ErrRejected
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	resolution, cancel := context.WithTimeout(ctx, CallTimeout)
	raw, err := b.resolver.Resolve(resolution, b.ref)
	resolutionErr := resolution.Err()
	cancel()
	defer clear(raw)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if resolutionErr != nil {
		return resolutionErr
	}
	if err != nil || len(raw) == 0 || len(raw) > MaxValueBytes {
		return ErrUnavailable
	}
	value := bytes.Clone(raw)
	clear(raw)
	defer clear(value)
	if consume(ctx, value) != nil {
		return ErrUse
	}
	return ctx.Err()
}
