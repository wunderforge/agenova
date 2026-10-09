// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
)

var ref = credentials.Reference{Resolver: ResolverID, Name: "provider", Key: "api-key"}

func secretJSON(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "provider", "namespace": "agenova"}, "type": "Opaque", "data": map[string]any{"api-key": base64.StdEncoding.EncodeToString([]byte("synthetic-secret-material"))}}
}

func encode(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal("synthetic encoding failed")
	}
	return data
}

func TestSecretLookupIsExactBoundedAndImmutable(t *testing.T) {
	keys := []Key{{Name: ref.Name, Key: ref.Key}}
	calls := 0
	current := secretJSON(t)
	r, err := New("agenova", keys, func(ctx context.Context, namespace, name string) ([]byte, error) {
		calls++
		if namespace != "agenova" || name != "provider" {
			t.Fatal("foreign Secret selected")
		}
		if d, ok := ctx.Deadline(); !ok || time.Until(d) > credentials.CallTimeout {
			t.Fatal("lookup unbounded")
		}
		return encode(t, current), nil
	})
	if err != nil {
		t.Fatal("resolver setup failed")
	}
	keys[0] = Key{Name: "unrelated", Key: ref.Key}
	for _, bad := range []credentials.Reference{
		{}, {Resolver: "unknown", Name: ref.Name, Key: ref.Key},
		{Resolver: ResolverID, Name: "unrelated", Key: ref.Key}, {Resolver: ResolverID, Name: ref.Name, Key: "other-key"},
		{Resolver: ResolverID, Name: "../provider", Key: ref.Key}, {Resolver: ResolverID, Name: ref.Name, Key: "../key"},
	} {
		if _, err := r.Resolve(context.Background(), bad); err != credentials.ErrRejected || calls != 0 {
			t.Fatal("invalid or unauthorized lookup dispatched")
		}
	}
	value, err := r.Resolve(context.Background(), ref)
	if err != nil || !bytes.Equal(value, []byte("synthetic-secret-material")) || calls != 1 {
		t.Fatal("authorized Secret resolution failed")
	}
	clear(value)
	current["data"] = map[string]any{"api-key": base64.StdEncoding.EncodeToString([]byte("rotated-synthetic"))}
	value, err = r.Resolve(context.Background(), ref)
	if err != nil || !bytes.Equal(value, []byte("rotated-synthetic")) || calls != 2 {
		t.Fatal("later resolution did not reload")
	}
	clear(value)
}

func TestSecretFailuresAreNonDisclosingAndStopConsumer(t *testing.T) {
	for _, test := range []struct {
		name   string
		modify func(map[string]any)
		raw    []byte
		err    error
	}{
		{name: "missing", err: errors.New("private-not-found")}, {name: "forbidden", err: errors.New("private-forbidden")}, {name: "unavailable", err: errors.New("private-backend-error")},
		{name: "wrong kind", modify: func(v map[string]any) { v["kind"] = "ConfigMap" }},
		{name: "wrong version", modify: func(v map[string]any) { v["apiVersion"] = "other" }},
		{name: "foreign name", modify: func(v map[string]any) { v["metadata"].(map[string]any)["name"] = "unrelated" }},
		{name: "foreign namespace", modify: func(v map[string]any) { v["metadata"].(map[string]any)["namespace"] = "unrelated" }},
		{name: "wrong Secret type", modify: func(v map[string]any) { v["type"] = "kubernetes.io/service-account-token" }},
		{name: "missing key", modify: func(v map[string]any) { v["data"] = map[string]any{} }},
		{name: "invalid base64", modify: func(v map[string]any) { v["data"] = map[string]any{"api-key": "private-not-base64"} }},
		{name: "empty value", modify: func(v map[string]any) { v["data"] = map[string]any{"api-key": ""} }},
		{name: "oversized value", modify: func(v map[string]any) {
			v["data"] = map[string]any{"api-key": base64.StdEncoding.EncodeToString(make([]byte, credentials.MaxValueBytes+1))}
		}},
		{name: "malformed JSON", raw: []byte(`{"private":"malformed"`)},
		{name: "trailing JSON", raw: []byte(`{} {}`)},
		{name: "oversized JSON", raw: bytes.Repeat([]byte("x"), MaxJSONBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := secretJSON(t)
			if test.modify != nil {
				test.modify(current)
			}
			calls, external := 0, 0
			raw := test.raw
			if raw == nil {
				raw = encode(t, current)
			}
			resolver, err := New("agenova", []Key{{Name: ref.Name, Key: ref.Key}}, func(context.Context, string, string) ([]byte, error) { calls++; return bytes.Clone(raw), test.err })
			if err != nil {
				t.Fatal("resolver setup failed")
			}
			registry, err := credentials.New([]credentials.Registration{{ID: ResolverID, Version: "0.1.0", Resolver: resolver, Allowed: []credentials.Reference{ref}}})
			if err != nil {
				t.Fatal("registry setup failed")
			}
			binding, err := registry.Bind(ref)
			if err != nil {
				t.Fatal("binding setup failed")
			}
			if binding.Use(context.Background(), func(context.Context, []byte) error { external++; return nil }) != credentials.ErrUnavailable || calls != 1 || external != 0 {
				t.Fatal("failed Secret reached external consumer")
			}
		})
	}
}

func TestSecretConstructionAndCancellationHaveZeroCalls(t *testing.T) {
	calls := 0
	get := func(context.Context, string, string) ([]byte, error) { calls++; return nil, nil }
	for _, ns := range []string{"", "other/namespace", "UPPER", "-invalid", strings.Repeat("a", 64)} {
		if _, err := New(ns, []Key{{Name: ref.Name, Key: ref.Key}}, get); err != credentials.ErrConfiguration {
			t.Fatal("invalid namespace accepted")
		}
	}
	for _, keys := range [][]Key{nil, {{Name: "../secret", Key: ref.Key}}, {{Name: ref.Name, Key: "bad/key"}}, {{Name: ref.Name, Key: ref.Key}, {Name: ref.Name, Key: ref.Key}}} {
		if _, err := New("agenova", keys, get); err != credentials.ErrConfiguration {
			t.Fatal("invalid key selection accepted")
		}
	}
	r, err := New("agenova", []Key{{Name: ref.Name, Key: ref.Key}}, get)
	if err != nil {
		t.Fatal("setup failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Resolve(ctx, ref); err != context.Canceled || calls != 0 {
		t.Fatal("cancelled Secret call dispatched")
	}
	if _, err := r.Resolve(nil, ref); err != credentials.ErrRejected || calls != 0 {
		t.Fatal("nil context dispatched")
	}
}

func TestKubectlUsesExplicitIdentityAndSanitizesOutput(t *testing.T) {
	var args []string
	get, err := kubectlGetter("kind-explicit", func(ctx context.Context, a ...string) ([]byte, error) {
		args = append([]string(nil), a...)
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("command unbounded")
		}
		return []byte("private-command-output"), errors.New("private-command-error")
	})
	if err != nil {
		t.Fatal("getter setup failed")
	}
	if _, err := get(context.Background(), "agenova", "provider"); err != credentials.ErrUnavailable {
		t.Fatal("command error was not sanitized")
	}
	want := []string{"--context", "kind-explicit", "--namespace", "agenova", "--request-timeout=5s", "get", "secret", "provider", "-o", "json"}
	if !reflect.DeepEqual(args, want) {
		t.Fatal("command identity/selection changed")
	}
	for _, contextName := range []string{"", " leading", "line\ncontext", "zero\x00context"} {
		if _, err := NewKubectl(contextName); err != credentials.ErrConfiguration {
			t.Fatal("implicit or malformed context accepted")
		}
	}
}

func TestKubectlOutputBoundCannotBeBypassedByReaderFrom(t *testing.T) {
	var buffer boundedBuffer
	if _, err := io.Copy(&buffer, io.LimitReader(strings.NewReader(strings.Repeat("x", MaxJSONBytes+1)), int64(MaxJSONBytes+1))); err == nil {
		t.Fatal("oversized command output accepted")
	}
}
