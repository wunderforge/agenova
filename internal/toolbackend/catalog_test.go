// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolbackend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func descriptor() Descriptor {
	return Descriptor{Description: "Read an allowed file and return untrusted text.", Operation: "repo.read", ResourceScope: "repo:example/a", Parameter: "file", MaxBytes: 128, AllowedValues: []string{"z.txt", "README.md"}}
}
func TestCatalogFreezesRoutesAndIntersectsAuthority(t *testing.T) {
	input := descriptor()
	other := descriptor()
	other.ResourceScope = "repo:example/b"
	other.AllowedValues = []string{"other.txt"}
	catalog, err := NewCatalog([]Descriptor{input, other})
	if err != nil {
		t.Fatal(err)
	}
	input.AllowedValues[0] = "secret.txt"
	public := catalog.Entries()
	public[0].AllowedValues[0] = "modified"
	subset := catalog.Intersect([]string{"repo.read"}, []string{"repo:example/b"})
	if len(subset.Entries()) != 1 || subset.Validate("repo.read", "repo:example/b", map[string]string{"file": "other.txt"}) != nil {
		t.Fatal("intersection lost allowed route")
	}
	for _, probe := range []struct {
		op, scope, value string
		extra            bool
	}{
		{"repo.read", "repo:example/a", "README.md", false}, {"repo.write", "repo:example/b", "other.txt", false},
		{"repo.read", "repo:example/b", "README.md", false}, {"repo.read", "repo:example/b", "other.txt", true},
	} {
		parameters := map[string]string{"file": probe.value}
		if probe.extra {
			parameters["url"] = "https://outside.invalid"
		}
		if subset.Validate(probe.op, probe.scope, parameters) == nil {
			t.Fatal("catalog allowed out-of-route request")
		}
	}
	if catalog.Validate("repo.read", "repo:example/a", map[string]string{"file": "README.md"}) != nil {
		t.Fatal("caller mutation widened or corrupted catalog")
	}
	if len(catalog.Intersect(nil, nil).Entries()) != 0 || catalog.Supports("repo.write") {
		t.Fatal("catalog grants access")
	}
}
func TestCatalogRejectsDuplicateAndIncompatibleRoutes(t *testing.T) {
	duplicate := descriptor()
	incompatible := descriptor()
	incompatible.ResourceScope = "repo:example/b"
	incompatible.Parameter = "path"
	for _, entries := range [][]Descriptor{{descriptor(), duplicate}, {descriptor(), incompatible}} {
		if _, err := NewCatalog(entries); err == nil {
			t.Fatal("ambiguous catalog accepted")
		}
	}
}

type providerFunc func(context.Context, Invocation) (Result, error)

func (f providerFunc) Invoke(ctx context.Context, call Invocation) (Result, error) {
	return f(ctx, call)
}
func TestSetBoundsUntrustedResultsWithoutChangingCallerParameters(t *testing.T) {
	calls := 0
	set, err := NewSet([]Binding{{Descriptor: descriptor(), MaxObservationBytes: 4, Provider: providerFunc(func(_ context.Context, call Invocation) (Result, error) {
		calls++
		call.Parameters["file"] = "changed"
		return Result{Text: "你好!", ResultRef: "artifact:readme"}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	input := Invocation{ID: "host-call", ClaimID: "claim-a", Operation: "repo.read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "README.md"}}
	result, err := set.Invoke(context.Background(), input)
	if err != nil || !result.Untrusted || !result.Truncated || result.Text != "你" || !utf8.ValidString(result.Text) || result.ResultRef != "artifact:readme" || input.Parameters["file"] != "README.md" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	input.Parameters["file"] = "outside.txt"
	if _, err := set.Invoke(context.Background(), input); err == nil || calls != 1 {
		t.Fatal("invalid argument reached provider")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := set.Invoke(ctx, input); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("cancelled invocation reached provider")
	}
}
func TestSetDoesNotLeakProviderErrorsOrUnsafeReferences(t *testing.T) {
	for _, tc := range []struct {
		name             string
		result           Result
		err, errorWanted error
	}{
		{name: "raw error", err: errors.New("secret endpoint and credential"), errorWanted: ErrProvider},
		{name: "unavailable", err: ErrUnavailable, errorWanted: ErrUnavailable},
		{name: "bounded response", err: fmt.Errorf("private endpoint: %w", ErrResponseTooLarge), errorWanted: ErrResponseTooLarge},
		{name: "timeout", err: fmt.Errorf("private endpoint: %w", ErrTimeout), errorWanted: ErrTimeout},
		{name: "malformed", err: ErrProtocol, errorWanted: ErrProtocol},
		{name: "url", result: Result{ResultRef: "https://private.invalid"}, errorWanted: ErrResult},
		{name: "oversize", result: Result{Text: strings.Repeat("a", (1<<20)+1)}, errorWanted: ErrResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, err := NewSet([]Binding{{Descriptor: descriptor(), MaxObservationBytes: 4, Provider: providerFunc(func(context.Context, Invocation) (Result, error) { return tc.result, tc.err })}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = set.Invoke(context.Background(), Invocation{ID: "call", ClaimID: "claim", Operation: "repo.read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "README.md"}})
			if err != tc.errorWanted {
				t.Fatalf("got %v", err)
			}
		})
	}
	var typedNil *nilClient
	if _, err := NewSet([]Binding{{Descriptor: descriptor(), MaxObservationBytes: 4, Provider: typedNil}}); err == nil {
		t.Fatal("typed nil accepted")
	}
}

type nilClient struct{}

func (*nilClient) Invoke(context.Context, Invocation) (Result, error) { panic("nil") }

func TestCatalogRequiresBoundedTrustedDescriptions(t *testing.T) {
	for _, description := range []string{"", strings.Repeat("a", 257), "unsafe\nrole"} {
		entry := descriptor()
		entry.Description = description
		if _, err := NewCatalog([]Descriptor{entry}); err == nil {
			t.Fatal("invalid description accepted")
		}
	}
}
