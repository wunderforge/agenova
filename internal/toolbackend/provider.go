// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolbackend

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"
)

var ErrUnavailable = errors.New("configured tool transport is unavailable")
var ErrProvider = errors.New("tool provider failed")
var ErrResult = errors.New("invalid tool provider result")
var ErrTimeout = errors.New("tool provider timed out")
var ErrResponseTooLarge = errors.New("tool response exceeds the byte limit")
var ErrProtocol = errors.New("tool provider returned an unsupported or malformed response")

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

type Binding struct {
	Descriptor          Descriptor
	Provider            Provider
	MaxObservationBytes int
}

// Set is immutable after construction; all inputs/outputs are detached. It
// validates catalog routes and results, while the Gateway owns permissions.
type Set struct {
	catalog  Catalog
	bindings map[string]Binding
}

func NewSet(bindings []Binding) (*Set, error) {
	entries := make([]Descriptor, 0, len(bindings))
	for _, binding := range bindings {
		if nilProvider(binding.Provider) || binding.MaxObservationBytes < 1 || binding.MaxObservationBytes > 16<<10 {
			return nil, ErrCatalog
		}
		entries = append(entries, binding.Descriptor)
	}
	catalog, err := NewCatalog(entries)
	if err != nil {
		return nil, err
	}
	result := &Set{catalog: catalog, bindings: map[string]Binding{}}
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
	result, err := binding.Provider.Invoke(ctx, call)
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err != nil {
		for _, safe := range []error{ErrUnavailable, ErrTimeout, ErrResponseTooLarge, ErrProtocol} {
			if errors.Is(err, safe) {
				return Result{}, safe
			}
		}
		return Result{}, ErrProvider
	}
	if !utf8.ValidString(result.Text) || strings.ContainsRune(result.Text, 0) || len(result.Text) > 1<<20 {
		return Result{}, ErrResult
	}
	if result.ResultRef != "" && (!bounded(result.ResultRef, 256) || strings.ContainsAny(result.ResultRef, " \t?#@") || strings.Contains(result.ResultRef, "://")) {
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
