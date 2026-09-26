// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package toolbackend owns provider-neutral tool catalogs and call results.
// Catalog membership constrains routing; it never grants claim authority.
package toolbackend

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrCatalog = errors.New("invalid tool catalog")
var ErrArguments = errors.New("tool operation, resource or arguments are not configured")
var operationName = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
var parameterName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// Descriptor is safe to advertise to a worker after authority intersection.
// It contains no provider endpoint, transport, native tool name or credential.
// The initial contract supports one required, allowlisted string parameter.
type Descriptor struct {
	Operation     string   `json:"operation"`
	Description   string   `json:"description"`
	ResourceScope string   `json:"resourceScope"`
	Parameter     string   `json:"parameter"`
	MaxBytes      int      `json:"maxBytes"`
	AllowedValues []string `json:"allowedValues"`
}

type Catalog struct{ entries []Descriptor }

func NewCatalog(entries []Descriptor) (Catalog, error) {
	if len(entries) > 32 {
		return Catalog{}, ErrCatalog
	}
	result := Catalog{}
	seen := map[string]bool{}
	shapes := map[string]string{}
	for _, entry := range entries {
		if !bounded(entry.Description, 256) || !operationName.MatchString(entry.Operation) || len(entry.Operation) > 128 || !bounded(entry.ResourceScope, 256) || strings.ContainsAny(entry.ResourceScope, "*? \t\r\n") || !strings.Contains(entry.ResourceScope, ":") || !parameterName.MatchString(entry.Parameter) || entry.MaxBytes < 1 || entry.MaxBytes > 4096 || len(entry.AllowedValues) == 0 || len(entry.AllowedValues) > 64 {
			return Catalog{}, ErrCatalog
		}
		key := routeKey(entry.Operation, entry.ResourceScope)
		if seen[key] || (shapes[entry.Operation] != "" && shapes[entry.Operation] != entry.Parameter) {
			return Catalog{}, ErrCatalog
		}
		seen[key], shapes[entry.Operation] = true, entry.Parameter
		entry.AllowedValues = append([]string(nil), entry.AllowedValues...)
		sort.Strings(entry.AllowedValues)
		for i, value := range entry.AllowedValues {
			if !bounded(value, entry.MaxBytes) || (i > 0 && value == entry.AllowedValues[i-1]) {
				return Catalog{}, ErrCatalog
			}
		}
		result.entries = append(result.entries, entry)
	}
	sort.Slice(result.entries, func(i, j int) bool {
		return routeKey(result.entries[i].Operation, result.entries[i].ResourceScope) < routeKey(result.entries[j].Operation, result.entries[j].ResourceScope)
	})
	data, err := json.Marshal(result.entries)
	if err != nil || len(data) > 16<<10 {
		return Catalog{}, ErrCatalog
	}
	return result, nil
}

func (c Catalog) Entries() []Descriptor {
	entries := make([]Descriptor, len(c.entries))
	for i, entry := range c.entries {
		entries[i] = entry
		entries[i].AllowedValues = append([]string(nil), entry.AllowedValues...)
	}
	return entries
}

// Intersect returns a detached snapshot. Grants for uninstalled operations
// should additionally be diagnosed with Supports before Work allocation.
func (c Catalog) Intersect(operations, scopes []string) Catalog {
	entries := []Descriptor{}
	for _, entry := range c.entries {
		if contains(operations, entry.Operation) && contains(scopes, entry.ResourceScope) {
			entries = append(entries, entry)
		}
	}
	result, _ := NewCatalog(entries) // A subset preserves the already validated bounds.
	return result
}

func (c Catalog) Supports(operation string) bool {
	for _, entry := range c.entries {
		if entry.Operation == operation {
			return true
		}
	}
	return false
}

func (c Catalog) Validate(operation, scope string, parameters map[string]string) error {
	for _, entry := range c.entries {
		if entry.Operation != operation || entry.ResourceScope != scope {
			continue
		}
		value, ok := parameters[entry.Parameter]
		if !ok || len(parameters) != 1 || !bounded(value, entry.MaxBytes) || !contains(entry.AllowedValues, value) {
			return ErrArguments
		}
		return nil
	}
	return ErrArguments
}

func (c Catalog) Parameter(operation, scope string) (string, bool) {
	for _, entry := range c.entries {
		if entry.Operation == operation && entry.ResourceScope == scope {
			return entry.Parameter, true
		}
	}
	return "", false
}

func SplitOperation(operation string) (string, string) {
	index := strings.LastIndexByte(operation, '.')
	if index < 1 {
		return "", ""
	}
	return operation[:index], operation[index+1:]
}
func routeKey(operation, scope string) string { return operation + "\x00" + scope }
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func bounded(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}
