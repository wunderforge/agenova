// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolbackend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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
	set, err := NewSet([]Binding{{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: providerFunc(func(_ context.Context, call Invocation) (Result, error) {
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
		{name: "credential unavailable", err: fmt.Errorf("secret e16/token: %w", ErrCredentialUnavailable), errorWanted: ErrCredentialUnavailable},
		{name: "credential rejected", err: fmt.Errorf("Bearer abc: %w", ErrCredentialRejected), errorWanted: ErrCredentialRejected},
		{name: "url", result: Result{ResultRef: "https://private.invalid"}, errorWanted: ErrResult},
		{name: "oversize", result: Result{Text: strings.Repeat("a", (1<<20)+1)}, errorWanted: ErrResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, err := NewSet([]Binding{{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: providerFunc(func(context.Context, Invocation) (Result, error) { return tc.result, tc.err })}})
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
	if _, err := NewSet([]Binding{{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: typedNil}}); err == nil {
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

func TestSetEnforcesSharedBackendConcurrency(t *testing.T) {
	second := descriptor()
	second.ResourceScope = "repo:example/b"
	var mu sync.Mutex
	active, peak, calls := 0, 0, 0
	release := make(chan struct{})
	entered := make(chan struct{}, 8)
	provider := providerFunc(func(ctx context.Context, _ Invocation) (Result, error) {
		mu.Lock()
		active, calls = active+1, calls+1
		if active > peak {
			peak = active
		}
		mu.Unlock()
		entered <- struct{}{}
		<-release
		mu.Lock()
		active--
		mu.Unlock()
		return Result{Text: "ok"}, nil
	})
	set, err := NewSet([]Binding{
		{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 2, Provider: provider},
		{Descriptor: second, Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 2, Provider: provider},
	})
	if err != nil {
		t.Fatal(err)
	}
	call := func(scope string) Invocation {
		return Invocation{ID: "call", ClaimID: "claim", Operation: "repo.read", ResourceScope: scope, Parameters: map[string]string{"file": "README.md"}}
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		scope := []string{"repo:example/a", "repo:example/b"}[i%2]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := set.Invoke(context.Background(), call(scope)); err != nil {
				t.Error(err)
			}
		}()
	}
	<-entered
	<-entered
	// Two profiles share one backend limit: a third call must wait, and a
	// cancelled waiter must never reach the provider.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := set.Invoke(ctx, call("repo:example/a")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting call returned %v", err)
	}
	close(release)
	wg.Wait()
	if peak != 2 || calls != 4 {
		t.Fatalf("peak=%d calls=%d, want peak 2 and 4 calls", peak, calls)
	}
}

func TestSetReleasesCapacityAfterProviderErrors(t *testing.T) {
	failures := 0
	set, err := NewSet([]Binding{{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: providerFunc(func(context.Context, Invocation) (Result, error) {
		failures++
		return Result{}, ErrUnavailable
	})}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := set.Invoke(ctx, Invocation{ID: "call", ClaimID: "claim", Operation: "repo.read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "README.md"}})
		cancel()
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("call %d: %v (capacity was not released)", i, err)
		}
	}
	if failures != 3 {
		t.Fatalf("provider saw %d calls, want 3", failures)
	}
}

func TestSetRejectsInconsistentOrMissingConcurrency(t *testing.T) {
	second := descriptor()
	second.ResourceScope = "repo:example/b"
	provider := providerFunc(func(context.Context, Invocation) (Result, error) { return Result{}, nil })
	for name, bindings := range map[string][]Binding{
		"missing limit":   {{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, Provider: provider}},
		"limit too large": {{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 17, Provider: provider}},
		"missing backend": {{Descriptor: descriptor(), MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: provider}},
		"disagreeing limits": {
			{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 1, Provider: provider},
			{Descriptor: second, Backend: "docs", MaxObservationBytes: 4, MaxConcurrentCalls: 2, Provider: provider},
		},
	} {
		if _, err := NewSet(bindings); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
