// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package toolbackend

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSetRetainsOnlyCanonicalStructuredDiagnostic(t *testing.T) {
	set, err := NewSet([]Binding{{Descriptor: descriptor(), Backend: "docs", MaxObservationBytes: 128, MaxConcurrentCalls: 1, Provider: providerFunc(func(context.Context, Invocation) (Result, error) {
		return Result{}, &DiagnosticError{Detail: Diagnostic{Code: "source-unavailable", HTTPStatus: 404, Message: "password=DO-NOT-ECHO", Stage: "private"}}
	})}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = set.Invoke(context.Background(), Invocation{ID: "call", ClaimID: "claim", Operation: "repo.read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "README.md"}})
	d := ErrorDiagnostic(err)
	if d == nil || d.HTTPStatus != 404 || d.Stage != "response" || strings.Contains(err.Error(), "DO-NOT-ECHO") {
		t.Fatal("canonical cause lost or unsafe provider text retained", err)
	}
}

func TestDiagnosticKeepsCauseButNeverRemoteSecrets(t *testing.T) {
	custom := &DiagnosticError{Detail: Diagnostic{Code: "dns-failed", Message: "password=DO-NOT-ECHO", Stage: "untrusted"}}
	if strings.Contains(custom.Error(), "DO-NOT-ECHO") || ErrorDiagnostic(custom).Stage != "network" {
		t.Fatal("noncanonical provider message escaped")
	}
	err := ParseDiagnostic(`{"agenovaError":{"code":"source-unavailable","httpStatus":503,"message":"password=FAKE-secret","actualBytes":999999999}}`)
	d := ErrorDiagnostic(err)
	if !errors.Is(err, ErrProvider) || d == nil || d.HTTPStatus != 503 || !d.Retryable || d.ActualBytes != 0 || strings.Contains(err.Error(), "FAKE-secret") {
		t.Fatal("unsafe or missing diagnostic")
	}
	for _, text := range []string{`{"agenovaError":{"code":"password=FAKE-secret"}}`, `<html>upstream password</html>`, strings.Repeat("x", 2049)} {
		if ErrorDiagnostic(ParseDiagnostic(text)) != nil {
			t.Fatal("unrecognized text became a diagnostic")
		}
	}
}
