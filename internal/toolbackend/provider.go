// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolbackend

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/wunderforge/agenova/internal/facts"
)

var ErrUnavailable = errors.New("configured tool transport is unavailable")
var ErrProvider = errors.New("tool provider failed")
var ErrResult = errors.New("invalid tool provider result")
var ErrTimeout = errors.New("tool provider timed out")
var ErrResponseTooLarge = errors.New("tool response exceeds the byte limit")
var ErrProtocol = errors.New("tool provider returned an unsupported or malformed response")

// ErrCredentialUnavailable means a configured credential could not be
// resolved, so no request was sent. ErrCredentialRejected means the server
// refused the credential that was sent. Neither carries the credential.
var ErrCredentialUnavailable = errors.New("tool provider credential is unavailable")
var ErrCredentialRejected = errors.New("tool provider rejected the credential")

// Invocation is issued by the host after Gateway authorization and recording.
// ClaimID is correlation only; this type does not authenticate a caller.
type Invocation struct {
	ID            string
	ClaimID       string
	Operation     string
	ResourceScope string
	Parameters    map[string]string
}

// ResultRef is separate from the stable attempt/outcome Target. The service
// owns evidence projection; arbitrary provider references are not URLs to fetch.
type Result struct {
	Text      string
	ResultRef string
	Untrusted bool
	Truncated bool
}

type Provider interface {
	Invoke(context.Context, Invocation) (Result, error)
}

// Factory is implemented by an adapter, without a dependency on its consumer.
// Construction validates configuration but must not contact the provider.
type Factory interface {
	NewToolProvider(instance map[string]any, profiles []map[string]any) (Provider, error)
}

// Limiter is implemented by a provider that has a configured concurrency
// ceiling. The installed builder copies it into each Binding.
type Limiter interface {
	MaxConcurrentCalls() int
}

// Binding routes one catalog entry to a provider. Bindings that name the same
// Backend share one concurrency limit, so they must agree on it.
type Binding struct {
	Descriptor          Descriptor
	Provider            Provider
	Backend             string
	MaxObservationBytes int
	MaxConcurrentCalls  int
}

// Set is immutable after construction; all inputs/outputs are detached. It
// validates catalog routes and results, while the Gateway owns permissions.
type Set struct {
	catalog  Catalog
	bindings map[string]Binding
	slots    map[string]chan struct{}
}

func NewSet(bindings []Binding) (*Set, error) {
	entries := make([]Descriptor, 0, len(bindings))
	limits := map[string]int{}
	for _, binding := range bindings {
		if nilProvider(binding.Provider) || binding.MaxObservationBytes < 1 || binding.MaxObservationBytes > 16<<10 ||
			!bounded(binding.Backend, 256) || binding.MaxConcurrentCalls < 1 || binding.MaxConcurrentCalls > 16 {
			return nil, ErrCatalog
		}
		if limit, ok := limits[binding.Backend]; ok && limit != binding.MaxConcurrentCalls {
			return nil, ErrCatalog
		}
		limits[binding.Backend] = binding.MaxConcurrentCalls
		entries = append(entries, binding.Descriptor)
	}
	catalog, err := NewCatalog(entries)
	if err != nil {
		return nil, err
	}
	result := &Set{catalog: catalog, bindings: map[string]Binding{}, slots: map[string]chan struct{}{}}
	for backend, limit := range limits {
		result.slots[backend] = make(chan struct{}, limit)
	}
	for _, binding := range bindings {
		binding.Descriptor.AllowedValues = append([]string(nil), binding.Descriptor.AllowedValues...)
		result.bindings[routeKey(binding.Descriptor.Operation, binding.Descriptor.ResourceScope)] = binding
	}
	return result, nil
}
func (s *Set) Catalog() Catalog {
	if s == nil {
		return Catalog{}
	}
	return s.catalog
}

// ValidResultRef reports whether an optional result reference meets the
// shared contract (facts.ValidResultRef): bounded, on one line, and never a
// fetchable URL or one carrying whitespace, a query, a fragment or userinfo.
// A provider omits a reference that does not, rather than turning a
// successful call into a failed one.
func ValidResultRef(ref string) bool {
	return facts.ValidResultRef(ref)
}

func (s *Set) Invoke(ctx context.Context, call Invocation) (Result, error) {
	if s == nil {
		return Result{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !bounded(call.ID, 256) || !bounded(call.ClaimID, 256) {
		return Result{}, ErrArguments
	}
	if err := s.catalog.Validate(call.Operation, call.ResourceScope, call.Parameters); err != nil {
		return Result{}, err
	}
	parameters := make(map[string]string, len(call.Parameters))
	for k, v := range call.Parameters {
		parameters[k] = v
	}
	call.Parameters = parameters
	binding := s.bindings[routeKey(call.Operation, call.ResourceScope)]
	// Wait for backend capacity before entering the provider; a cancelled
	// wait never reaches it.
	slots := s.slots[binding.Backend]
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	defer func() { <-slots }()
	result, err := binding.Provider.Invoke(ctx, call)
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err != nil {
		for _, safe := range []error{ErrUnavailable, ErrTimeout, ErrResponseTooLarge, ErrProtocol, ErrCredentialUnavailable, ErrCredentialRejected} {
			if errors.Is(err, safe) {
				return Result{}, safe
			}
		}
		return Result{}, ErrProvider
	}
	if !utf8.ValidString(result.Text) || strings.ContainsRune(result.Text, 0) || len(result.Text) > 1<<20 {
		return Result{}, ErrResult
	}
	if result.ResultRef != "" && !ValidResultRef(result.ResultRef) {
		return Result{}, ErrResult
	}
	if len(result.Text) > binding.MaxObservationBytes {
		data := result.Text[:binding.MaxObservationBytes]
		for !utf8.ValidString(data) {
			data = data[:len(data)-1]
		}
		result.Text = data
		result.Truncated = true
	}
	result.Untrusted = true
	return result, nil
}
func nilProvider(provider Provider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return value.IsNil()
	}
	return false
}
