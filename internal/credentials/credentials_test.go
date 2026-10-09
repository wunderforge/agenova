// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
	"gopkg.in/yaml.v3"
)

var testRef = credentials.Reference{Resolver: "test.example/credential/memory", Name: "provider", Key: "api-key"}

type memoryResolver struct {
	mu       sync.Mutex
	values   map[credentials.Reference][]byte
	calls    int
	seen     []credentials.Reference
	returned [][]byte
	err      error
}

func (m *memoryResolver) Resolve(ctx context.Context, ref credentials.Reference) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > credentials.CallTimeout {
		return nil, errors.New("resolution deadline missing")
	}
	m.calls++
	m.seen = append(m.seen, ref)
	value := bytes.Clone(m.values[ref])
	m.returned = append(m.returned, value)
	return value, m.err
}

func binding(t *testing.T, m *memoryResolver) (*credentials.Registry, *credentials.Binding) {
	t.Helper()
	r, err := credentials.New([]credentials.Registration{{ID: testRef.Resolver, Version: "0.1.0", Resolver: m, Allowed: []credentials.Reference{testRef}}})
	if err != nil {
		t.Fatal("registry setup failed")
	}
	b, err := r.Bind(testRef)
	if err != nil {
		t.Fatal("binding setup failed")
	}
	return r, b
}

func TestReferenceRegistryAndBindingAreFailClosed(t *testing.T) {
	m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte("synthetic-material")}}
	allowed := []credentials.Reference{testRef}
	registrations := []credentials.Registration{{ID: testRef.Resolver, Version: "0.1.0", Resolver: m, Allowed: allowed}}
	r, err := credentials.New(registrations)
	if err != nil {
		t.Fatal("registry setup failed")
	}
	b, err := r.Bind(testRef)
	if err != nil {
		t.Fatal("binding setup failed")
	}
	other := testRef
	other.Name = "unrelated"
	allowed[0] = other
	registrations[0].Resolver = &memoryResolver{}
	for _, ref := range []credentials.Reference{
		{}, {Resolver: "unknown", Name: testRef.Name, Key: testRef.Key}, other,
		{Resolver: testRef.Resolver, Name: "provider\x00", Key: testRef.Key},
		{Resolver: testRef.Resolver, Name: "provider\n", Key: testRef.Key},
		{Resolver: testRef.Resolver, Name: " provider", Key: testRef.Key},
		{Resolver: testRef.Resolver, Name: strings.Repeat("a", 257), Key: testRef.Key},
		{Resolver: testRef.Resolver, Name: "\xff", Key: testRef.Key},
	} {
		if _, err := r.Bind(ref); err != credentials.ErrRejected || m.calls != 0 {
			t.Fatal("invalid reference reached resolver")
		}
	}
	used := 0
	if b.Use(context.Background(), func(_ context.Context, value []byte) error {
		used++
		if !bytes.Equal(value, []byte("synthetic-material")) {
			return errors.New("wrong selected material")
		}
		return nil
	}) != nil || used != 1 || m.calls != 1 || m.seen[0] != testRef {
		t.Fatal("captured binding changed")
	}
	for _, registration := range []credentials.Registration{
		{}, {ID: testRef.Resolver, Version: "0.1.0", Resolver: (*memoryResolver)(nil), Allowed: []credentials.Reference{testRef}},
		{ID: testRef.Resolver, Version: "0.1.0", Resolver: m, Allowed: []credentials.Reference{testRef, testRef}},
		{ID: "different", Version: "0.1.0", Resolver: m, Allowed: []credentials.Reference{testRef}},
		{ID: testRef.Resolver, Version: "v1", Resolver: m, Allowed: []credentials.Reference{testRef}},
	} {
		if _, err := credentials.New([]credentials.Registration{registration}); err != credentials.ErrConfiguration {
			t.Fatal("invalid configuration accepted")
		}
	}
	if _, err := credentials.New([]credentials.Registration{registrations[0], registrations[0]}); err != credentials.ErrConfiguration {
		t.Fatal("duplicate resolver accepted")
	}
}

func TestScopedMaterialRedactionReleaseAndRotation(t *testing.T) {
	const sentinel = "synthetic-material-must-not-escape"
	m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte(sentinel)}}
	r, b := binding(t, m)
	var retained []byte
	if b.Use(context.Background(), func(_ context.Context, value []byte) error { retained = value; return nil }) != nil {
		t.Fatal("scoped use failed")
	}
	if !bytes.Equal(retained, make([]byte, len(retained))) || !bytes.Equal(m.returned[0], make([]byte, len(m.returned[0]))) {
		t.Fatal("owned material was not cleared")
	}
	registration := credentials.Registration{ID: testRef.Resolver, Version: "0.1.0", Resolver: m, Allowed: []credentials.Reference{testRef}}
	for _, value := range []any{r, b, registration, &registration} {
		for _, format := range []string{"%v", "%+v", "%#v", "%x", "%q"} {
			if strings.Contains(fmt.Sprintf(format, value), sentinel) {
				t.Fatal("formatted host object leaked material")
			}
		}
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != `"[redacted]"` {
			t.Fatal("JSON host object was not redacted")
		}
		encoded, err = yaml.Marshal(value)
		if err != nil || strings.Contains(string(encoded), sentinel) {
			t.Fatal("YAML host object leaked material")
		}
	}
	m.values[testRef] = []byte("rotated-synthetic-material")
	if b.Use(context.Background(), func(_ context.Context, value []byte) error {
		if !bytes.Equal(value, m.values[testRef]) {
			return errors.New("rotation not observed")
		}
		return errors.New(sentinel)
	}) != credentials.ErrUse {
		t.Fatal("consumer diagnostics were not sanitized")
	}
	if m.calls != 2 {
		t.Fatal("resolution unexpectedly cached")
	}
}

func TestUnavailableResolutionNeverCallsConsumer(t *testing.T) {
	for _, test := range []struct {
		name  string
		value []byte
		err   error
	}{
		{"missing", nil, nil}, {"empty", []byte{}, nil}, {"oversized", make([]byte, credentials.MaxValueBytes+1), nil},
		{"provider fault", []byte("private-provider-output"), errors.New("private-provider-error")},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: test.value}, err: test.err}
			_, b := binding(t, m)
			calls := 0
			if b.Use(context.Background(), func(context.Context, []byte) error { calls++; return nil }) != credentials.ErrUnavailable || calls != 0 || m.calls != 1 {
				t.Fatal("unavailable source reached external consumer")
			}
			if !bytes.Equal(m.returned[0], make([]byte, len(m.returned[0]))) {
				t.Fatal("failed-source material was not cleared")
			}
		})
	}
}

type resolverFunc func(context.Context, credentials.Reference) ([]byte, error)

func (f resolverFunc) Resolve(ctx context.Context, ref credentials.Reference) ([]byte, error) {
	return f(ctx, ref)
}

func TestCancellationAndDeadlineStopBeforeUse(t *testing.T) {
	m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte("synthetic")}}
	_, b := binding(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	consumer := func(context.Context, []byte) error { t.Fatal("cancelled call reached consumer"); return nil }
	if b.Use(ctx, consumer) != context.Canceled || m.calls != 0 {
		t.Fatal("cancelled call reached resolver")
	}
	if b.Use(nil, consumer) != credentials.ErrRejected || b.Use(context.Background(), nil) != credentials.ErrRejected || m.calls != 0 {
		t.Fatal("invalid use dispatched")
	}
	r, err := credentials.New([]credentials.Registration{{ID: testRef.Resolver, Version: "0.1.0", Allowed: []credentials.Reference{testRef}, Resolver: resolverFunc(func(ctx context.Context, _ credentials.Reference) ([]byte, error) {
		<-ctx.Done()
		return []byte("private-late-material"), errors.New("private-late-error")
	})}})
	if err != nil {
		t.Fatal("deadline registry failed")
	}
	b, err = r.Bind(testRef)
	if err != nil {
		t.Fatal("deadline binding failed")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if b.Use(ctx, consumer) != context.DeadlineExceeded {
		t.Fatal("caller deadline not preserved")
	}
}

func TestResolutionDeadlineDoesNotLimitConsumer(t *testing.T) {
	for _, callerTimeout := range []time.Duration{0, time.Second, 20 * time.Second} {
		t.Run(callerTimeout.String(), func(t *testing.T) {
			caller := context.Background()
			if callerTimeout != 0 {
				var cancel context.CancelFunc
				caller, cancel = context.WithTimeout(caller, callerTimeout)
				defer cancel()
			}
			var resolution context.Context
			r, err := credentials.New([]credentials.Registration{{ID: testRef.Resolver, Version: "0.1.0", Allowed: []credentials.Reference{testRef}, Resolver: resolverFunc(func(ctx context.Context, _ credentials.Reference) ([]byte, error) {
				resolution = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > credentials.CallTimeout {
					t.Fatal("resolver deadline missing or unbounded")
				}
				return []byte("synthetic-material"), nil
			})}})
			if err != nil {
				t.Fatal("setup failed")
			}
			b, err := r.Bind(testRef)
			if err != nil {
				t.Fatal("binding failed")
			}
			if err := b.Use(caller, func(ctx context.Context, _ []byte) error {
				if ctx != caller {
					t.Error("consumer inherited the resolver timeout instead of caller context")
				}
				if resolution.Err() != context.Canceled || ctx.Err() != nil {
					t.Error("resolution context not released independently of live consumer")
				}
				return nil
			}); err != nil {
				t.Fatalf("consumer failed: %v", err)
			}
		})
	}
}

func TestConcurrentUsesHaveIndependentClearedBuffers(t *testing.T) {
	m := &memoryResolver{values: map[credentials.Reference][]byte{testRef: []byte("synthetic")}}
	_, b := binding(t, m)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Use(context.Background(), func(_ context.Context, value []byte) error {
				if !bytes.Equal(value, []byte("synthetic")) {
					return errors.New("wrong material")
				}
				return nil
			}) != nil {
				t.Error("concurrent use failed")
			}
		}()
	}
	wg.Wait()
	if m.calls != 20 {
		t.Fatal("concurrent resolution count mismatch")
	}
	for _, value := range m.returned {
		if !bytes.Equal(value, make([]byte, len(value))) {
			t.Fatal("concurrent buffer not cleared")
		}
	}
}
