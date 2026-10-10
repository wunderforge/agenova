// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package kubernetes

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
	"github.com/wunderforge/agenova/internal/modelprovider"
)

var (
	credentialContext   = flag.String("kube-context", "", "explicit existing test kubectl context (required)")
	credentialNamespace = flag.String("namespace", "", "new exclusive agenova-155- namespace (required)")
)

// This is an opt-in real-backend campaign. It creates only task-owned resources
// in a new, explicitly named namespace and uses synthetic bytes through stdin.
// No existing Secret value is read. Compile without touching a cluster with:
//
// go test -tags integration -run '^$' ./internal/credentials/kubernetes
//
// Run only after selecting the existing test cluster and an unused namespace:
//
// go test -v -tags integration -run '^TestSecretLive' -timeout 3m ./internal/credentials/kubernetes -args -kube-context <context> -namespace agenova-155-<unique-suffix>
func TestSecretLiveNamedRBACReloadAndSanitization(t *testing.T) {
	if *credentialContext == "" || strings.TrimSpace(*credentialContext) != *credentialContext ||
		!dnsLabel.MatchString(*credentialNamespace) || !strings.HasPrefix(*credentialNamespace, "agenova-155-") || len(*credentialNamespace) <= len("agenova-155-") {
		t.Fatal("credential integration requires explicit -kube-context and a new -namespace with agenova-155- prefix")
	}
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Fatal("credential integration requires kubectl")
	}
	if _, err := NewKubectl(*credentialContext); err != nil {
		t.Fatal("credential integration context is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	f := newCredentialLiveFixture(t, ctx)
	first, second := syntheticMaterial(t), syntheticMaterial(t)
	defer clear(first)
	defer clear(second)
	f.createSecret(t, ctx, "selected", "Opaque", map[string][]byte{"token": first})
	f.createSecret(t, ctx, "unselected", "Opaque", map[string][]byte{"token": second})
	f.createSecret(t, ctx, "wrong-type", "kubernetes.io/basic-auth", map[string][]byte{"username": []byte("synthetic"), "password": first, "token": first})
	f.createReader(t, ctx)

	identity := "system:serviceaccount:" + f.namespace + ":credential-reader"
	getterCalls, consumerCalls := 0, 0
	getter, err := kubectlGetter(f.kubeContext, func(callCtx context.Context, args ...string) ([]byte, error) {
		getterCalls++
		return execKubectl(callCtx, append([]string{"--as", identity}, args...)...)
	})
	if err != nil {
		t.Fatal("explicit scoped getter construction failed")
	}
	selected := credentials.Reference{Resolver: ResolverID, Name: "selected", Key: "token"}
	wrongType := credentials.Reference{Resolver: ResolverID, Name: "wrong-type", Key: "token"}
	// unselected is deliberately host-bound in this negative fixture: actual
	// Kubernetes RBAC must still deny it before the consumer is called.
	unselected := credentials.Reference{Resolver: ResolverID, Name: "unselected", Key: "token"}
	resolver, err := New(f.namespace, []Key{{selected.Name, selected.Key}, {wrongType.Name, wrongType.Key}, {unselected.Name, unselected.Key}}, getter)
	if err != nil {
		t.Fatal("scoped resolver construction failed")
	}
	registry, err := credentials.New([]credentials.Registration{{ID: ResolverID, Version: "0.1.0", Resolver: resolver, Allowed: []credentials.Reference{selected, wrongType, unselected}}})
	if err != nil {
		t.Fatal("explicit registry construction failed")
	}
	bind := func(ref credentials.Reference) *credentials.Binding {
		t.Helper()
		binding, bindErr := registry.Bind(ref)
		if bindErr != nil {
			t.Fatal("captured reference binding failed")
		}
		return binding
	}
	selectedBinding := bind(selected)
	assertUnavailable := func(t *testing.T, binding *credentials.Binding) {
		t.Helper()
		beforeGet, beforeConsumer := getterCalls, consumerCalls
		useErr := binding.Use(ctx, func(context.Context, []byte) error { consumerCalls++; return nil })
		if useErr != credentials.ErrUnavailable || getterCalls != beforeGet+1 || consumerCalls != beforeConsumer ||
			strings.Contains(useErr.Error(), string(first)) || strings.Contains(useErr.Error(), string(second)) {
			t.Fatal("source rejection must be sanitized and precede consumer execution")
		}
	}

	t.Run("named-get-positive-control", func(t *testing.T) {
		f.assertCanI(t, ctx, identity, true, "get", "secrets/selected")
		var retained []byte
		before := getterCalls
		useErr := selectedBinding.Use(ctx, func(callCtx context.Context, value []byte) error {
			consumerCalls++
			retained = value
			if callCtx != ctx || !bytes.Equal(value, first) {
				t.Fatal("selected value or caller context differs from captured input")
			}
			return nil
		})
		if useErr != nil || getterCalls != before+1 || !liveBytesCleared(retained) {
			t.Fatal("exact named get or owned-buffer release failed")
		}
		t.Log("real named Secret get reached one consumer; owned callback bytes cleared")
	})

	t.Run("effective-rbac-denies-unrelated-list-watch", func(t *testing.T) {
		for _, operation := range [][2]string{{"get", "secrets/unselected"}, {"list", "secrets"}, {"watch", "secrets"}} {
			f.assertCanI(t, ctx, identity, false, operation[0], operation[1])
		}
		for _, args := range [][]string{
			{"get", "secret", "unselected", "-o", "json"},
			{"get", "secrets", "-o", "json"},
			{"get", "secrets", "--watch-only", "-o", "json"},
		} {
			attemptCtx, attemptCancel := context.WithTimeout(ctx, credentials.CallTimeout)
			raw, deniedErr := f.command(attemptCtx, nil, append([]string{"--namespace", f.namespace, "--as", identity}, args...)...)
			attemptExpired := attemptCtx.Err()
			attemptCancel()
			clear(raw)
			if deniedErr == nil || attemptExpired != nil {
				t.Fatal("effective RBAC must reject unrelated get/list/watch, rather than expire")
			}
		}
		assertUnavailable(t, bind(unselected))
		t.Log("service account denies unrelated named get, list and watch; bound RBAC failure has zero consumer calls")
	})

	t.Run("host-allowlist-rejects-before-io", func(t *testing.T) {
		beforeGet, beforeConsumer := getterCalls, consumerCalls
		for _, ref := range []credentials.Reference{
			{Resolver: ResolverID, Name: "not-bound", Key: "token"},
			{Resolver: ResolverID, Name: "../selected", Key: "token"},
			{Resolver: ResolverID, Name: "selected", Key: "not-bound"},
			{Resolver: "example.org/credential/unknown", Name: "selected", Key: "token"},
		} {
			if binding, bindErr := registry.Bind(ref); bindErr != credentials.ErrRejected || binding != nil {
				t.Fatal("unbound reference entered a host binding")
			}
			value, resolveErr := resolver.Resolve(ctx, ref)
			clear(value)
			if resolveErr != credentials.ErrRejected {
				t.Fatal("unbound reference entered source resolution")
			}
		}
		if getterCalls != beforeGet || consumerCalls != beforeConsumer {
			t.Fatal("host allowlist rejection performed source or consumer I/O")
		}
		t.Log("malformed, unknown and unauthorized references perform zero source and consumer calls")
	})

	t.Run("wrong-type-and-missing-key", func(t *testing.T) {
		assertUnavailable(t, bind(wrongType))
		f.replaceSecret(t, ctx, "selected", "Opaque", map[string][]byte{"other": first})
		assertUnavailable(t, selectedBinding)
		f.replaceSecret(t, ctx, "selected", "Opaque", map[string][]byte{"token": first})
		t.Log("wrong Secret type and missing selected key fail before consumer execution")
	})

	t.Run("replacement-is-observed-without-mutating-inflight-value", func(t *testing.T) {
		var retained []byte
		useErr := selectedBinding.Use(ctx, func(_ context.Context, value []byte) error {
			consumerCalls++
			retained = value
			if !bytes.Equal(value, first) {
				t.Fatal("initial captured value differs")
			}
			f.replaceSecret(t, ctx, "selected", "Opaque", map[string][]byte{"token": second})
			if !bytes.Equal(value, first) {
				t.Fatal("source update changed the in-flight owned value")
			}
			return nil
		})
		if useErr != nil || !liveBytesCleared(retained) {
			t.Fatal("in-flight use or release failed")
		}
		for range 2 {
			useErr = selectedBinding.Use(ctx, func(_ context.Context, value []byte) error {
				consumerCalls++
				if !bytes.Equal(value, second) {
					t.Fatal("later resolution did not observe replacement")
				}
				return nil
			})
			if useErr != nil {
				t.Fatal("replacement resolution failed")
			}
		}
		t.Log("real Secret replacement is observed on later uses; in-flight bytes remain captured and are cleared")
	})

	t.Run("private-consumer-error-and-canceled-caller", func(t *testing.T) {
		useErr := selectedBinding.Use(ctx, func(context.Context, []byte) error {
			consumerCalls++
			return errors.New(string(second))
		})
		if useErr != credentials.ErrUse {
			t.Fatal("consumer error was not sanitized")
		}
		beforeGet, beforeConsumer := getterCalls, consumerCalls
		canceled, cancelCall := context.WithCancel(ctx)
		cancelCall()
		useErr = selectedBinding.Use(canceled, func(context.Context, []byte) error { consumerCalls++; return nil })
		if useErr != context.Canceled || getterCalls != beforeGet || consumerCalls != beforeConsumer {
			t.Fatal("canceled caller performed source or consumer execution")
		}
		t.Log("private consumer failure is sanitized; canceled caller performs zero source and consumer calls")
	})

	t.Run("production-model-consumer-real-secret-synthetic-http-peer", func(t *testing.T) {
		var httpCalls atomic.Int64
		var badRequest atomic.Bool
		expectedAuthorization := append([]byte("Bearer "), second...)
		defer clear(expectedAuthorization)
		const response = `{"id":"synthetic-response","model":"synthetic-model","choices":[{"message":{"role":"assistant","content":"A synthetic credential-backed answer."}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`
		peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			httpCalls.Add(1)
			body, readErr := io.ReadAll(io.LimitReader(request.Body, 4096))
			defer clear(body)
			if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" ||
				!bytes.Equal([]byte(request.Header.Get("Authorization")), expectedAuthorization) || readErr != nil ||
				liveContainsMaterial(body, first, second) {
				badRequest.Store(true)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, response)
		}))
		defer peer.Close()
		provider, providerErr := modelprovider.New(modelprovider.Config{
			Endpoint: peer.URL + "/v1", Models: map[string]string{"approved-synthetic": "synthetic-model"},
			Credential: selectedBinding, Timeout: credentials.CallTimeout,
		})
		if providerErr != nil {
			t.Fatal("production model consumer construction failed")
		}
		modelRequest := modelprovider.Request{Profile: "approved-synthetic", Prompt: "Return the synthetic campaign answer."}
		positive := func() {
			t.Helper()
			beforeSource, beforeHTTP := getterCalls, httpCalls.Load()
			result, completeErr := provider.Complete(ctx, modelRequest)
			encoded, encodeErr := json.Marshal(result)
			defer clear(encoded)
			if completeErr != nil || encodeErr != nil || result.Text != "A synthetic credential-backed answer." ||
				getterCalls != beforeSource+1 || httpCalls.Load() != beforeHTTP+1 || badRequest.Load() || liveContainsMaterial(encoded, first, second) {
				t.Fatal("production model consumer failed exact real-Secret to synthetic-HTTP wiring or public-result privacy")
			}
		}
		positive()
		f.replaceSecret(t, ctx, "selected", "Opaque", map[string][]byte{"other": second})
		beforeSource, beforeHTTP := getterCalls, httpCalls.Load()
		result, completeErr := provider.Complete(ctx, modelRequest)
		encoded, encodeErr := json.Marshal(result)
		defer clear(encoded)
		if completeErr != credentials.ErrUnavailable || encodeErr != nil || getterCalls != beforeSource+1 || httpCalls.Load() != beforeHTTP ||
			liveContainsMaterial(encoded, first, second) || liveContainsMaterial([]byte(completeErr.Error()), first, second) {
			t.Fatal("missing selected key must stop production model consumer before HTTP with a sanitized result")
		}
		f.replaceSecret(t, ctx, "selected", "Opaque", map[string][]byte{"token": second})
		positive()
		t.Logf("production model consumer resolves real selected Secret through the same immutable binding: synthetic HTTP peer calls=%d; missing-key control adds zero HTTP calls; restored key succeeds; external provider health and installed claim/worker E2E are not claimed", httpCalls.Load())
	})

	t.Run("deleted-selected-source", func(t *testing.T) {
		f.mustCommand(t, ctx, nil, "--namespace", f.namespace, "delete", "secret", "selected", "--wait=true", "--timeout=10s")
		assertUnavailable(t, selectedBinding)
		t.Log("deleted selected Secret fails before consumer execution")
	})
	t.Logf("real campaign complete: source calls=%d consumer calls=%d; values excluded from diagnostics", getterCalls, consumerCalls)
}

type credentialLiveFixture struct {
	kubeContext, namespace, owner, uid string
}

func newCredentialLiveFixture(t *testing.T, ctx context.Context) *credentialLiveFixture {
	t.Helper()
	ownerBytes := make([]byte, 12)
	if _, err := rand.Read(ownerBytes); err != nil {
		t.Fatal("campaign owner generation failed")
	}
	f := &credentialLiveFixture{kubeContext: *credentialContext, namespace: *credentialNamespace, owner: hex.EncodeToString(ownerBytes)}
	clear(ownerBytes)
	data := f.objectJSON(t, map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{
		"name": f.namespace, "labels": map[string]any{"agenova.io/credential-campaign": f.owner},
	}})
	defer clear(data)
	raw, err := f.command(ctx, data, "create", "-f", "-", "-o", "jsonpath={.metadata.uid}")
	defer clear(raw)
	if err != nil || len(raw) == 0 || len(raw) > 128 {
		t.Fatal("exclusive task namespace creation failed; existing namespaces are never adopted")
	}
	f.uid = string(raw)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		metadata, readErr := f.command(cleanupCtx, nil, "get", "namespace", f.namespace, "-o", "json")
		defer clear(metadata)
		var observed struct {
			Metadata struct {
				UID    string            `json:"uid"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		}
		if readErr != nil || json.Unmarshal(metadata, &observed) != nil || observed.Metadata.UID != f.uid || observed.Metadata.Labels["agenova.io/credential-campaign"] != f.owner {
			t.Error("cleanup refused: namespace owner/UID guard could not be confirmed")
			return
		}
		// DeleteOptions is sent through the selected kubeconfig transport. The
		// server-side UID precondition refuses a replacement with the same name.
		options := f.objectJSON(t, map[string]any{"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": map[string]any{"uid": f.uid}})
		defer clear(options)
		out, deleteErr := f.command(cleanupCtx, options, "delete", "--raw", "/api/v1/namespaces/"+f.namespace, "-f", "-")
		clear(out)
		if deleteErr != nil {
			t.Error("exact owned namespace UID-precondition cleanup failed")
			return
		}
		out, waitErr := f.command(cleanupCtx, nil, "wait", "--for=delete", "namespace/"+f.namespace, "--timeout=30s")
		clear(out)
		if waitErr != nil {
			t.Error("owned namespace deletion was not confirmed")
			return
		}
		t.Log("cleanup confirmed for exact namespace with owner guard and server-side UID precondition")
	})
	t.Logf("explicit existing context=%s namespace=%s uid=%s; synthetic-only task resources", f.kubeContext, f.namespace, f.uid)
	return f
}

func syntheticMaterial(t *testing.T) []byte {
	t.Helper()
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		t.Fatal("synthetic material generation failed")
	}
	encoded := make([]byte, hex.EncodedLen(len(value)))
	hex.Encode(encoded, value)
	clear(value)
	return encoded
}

func (f *credentialLiveFixture) objectJSON(t *testing.T, object map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal("synthetic resource encoding failed")
	}
	return data
}

func (f *credentialLiveFixture) createSecret(t *testing.T, ctx context.Context, name, secretType string, values map[string][]byte) {
	t.Helper()
	data := f.objectJSON(t, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": name, "namespace": f.namespace}, "type": secretType, "data": values})
	defer clear(data)
	f.mustCommand(t, ctx, data, "--namespace", f.namespace, "create", "-f", "-")
}

func (f *credentialLiveFixture) replaceSecret(t *testing.T, ctx context.Context, name, secretType string, values map[string][]byte) {
	t.Helper()
	raw, err := f.command(ctx, nil, "--namespace", f.namespace, "get", "secret", name, "-o", "jsonpath={.metadata.resourceVersion}")
	defer clear(raw)
	if err != nil || len(raw) == 0 || len(raw) > 128 {
		t.Fatal("task-owned selected Secret version lookup failed")
	}
	data := f.objectJSON(t, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": name, "namespace": f.namespace, "resourceVersion": string(raw)}, "type": secretType, "data": values})
	defer clear(data)
	f.mustCommand(t, ctx, data, "--namespace", f.namespace, "replace", "-f", "-")
}

func (f *credentialLiveFixture) createReader(t *testing.T, ctx context.Context) {
	t.Helper()
	for _, object := range []map[string]any{
		{"apiVersion": "v1", "kind": "ServiceAccount", "metadata": map[string]any{"name": "credential-reader", "namespace": f.namespace}, "automountServiceAccountToken": false},
		{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "Role", "metadata": map[string]any{"name": "credential-reader", "namespace": f.namespace}, "rules": []map[string]any{{"apiGroups": []string{""}, "resources": []string{"secrets"}, "resourceNames": []string{"selected", "wrong-type"}, "verbs": []string{"get"}}}},
		{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "RoleBinding", "metadata": map[string]any{"name": "credential-reader", "namespace": f.namespace}, "subjects": []map[string]any{{"kind": "ServiceAccount", "name": "credential-reader", "namespace": f.namespace}}, "roleRef": map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": "credential-reader"}},
	} {
		data := f.objectJSON(t, object)
		f.mustCommand(t, ctx, data, "--namespace", f.namespace, "create", "-f", "-")
		clear(data)
	}
}

func (f *credentialLiveFixture) assertCanI(t *testing.T, ctx context.Context, identity string, allowed bool, verb, resource string) {
	t.Helper()
	out, err := f.command(ctx, nil, "--namespace", f.namespace, "--as", identity, "auth", "can-i", verb, resource)
	defer clear(out)
	answer := strings.TrimSpace(string(out))
	if (allowed && (err != nil || answer != "yes")) || (!allowed && (err == nil || answer != "no")) {
		t.Fatal("selected service account effective RBAC differs from exact named-get scope")
	}
}

func (f *credentialLiveFixture) mustCommand(t *testing.T, ctx context.Context, input []byte, args ...string) {
	t.Helper()
	out, err := f.command(ctx, input, args...)
	clear(out)
	if err != nil {
		t.Fatal("task-owned Kubernetes operation failed (raw backend output suppressed)")
	}
}

func (f *credentialLiveFixture) command(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--context", f.kubeContext, "--request-timeout=5s"}, args...)...)
	cmd.WaitDelay = time.Second
	cmd.Stdin = bytes.NewReader(input)
	var stdout boundedBuffer
	defer clear(stdout.data.Bytes())
	cmd.Stdout, cmd.Stderr = &stdout, io.Discard
	if cmd.Run() != nil {
		return bytes.Clone(stdout.data.Bytes()), credentials.ErrUnavailable
	}
	return bytes.Clone(stdout.data.Bytes()), nil
}

func liveBytesCleared(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return len(value) != 0
}

func liveContainsMaterial(data []byte, materials ...[]byte) bool {
	for _, material := range materials {
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(material)))
		base64.StdEncoding.Encode(encoded, material)
		contains := bytes.Contains(data, material) || bytes.Contains(data, encoded)
		clear(encoded)
		if contains {
			return true
		}
	}
	return false
}
