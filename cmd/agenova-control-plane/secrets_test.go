// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeSecretToken = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// fakeKubectl writes a kubectl stand-in that records its arguments and then
// runs body, so each case controls stdout, stderr and the exit status.
func fakeKubectl(t *testing.T, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >" + args + "\n" + body + "\n"
	path := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path, args
}

func TestKubectlSecretReaderReadsOneKeyOfANamedSecret(t *testing.T) {
	object := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"e16-mcp-token"},"data":{"token":"` + base64.StdEncoding.EncodeToString([]byte(fakeSecretToken)) + `","token.v2":"` + base64.StdEncoding.EncodeToString([]byte("second")) + `"}}`
	path, args := fakeKubectl(t, "echo 'Warning: ignored on stderr' >&2\ncat <<'JSON'\n"+object+"\nJSON")
	reader := kubectlSecretReader{namespace: "agenova-e16-system", kubectl: path}
	value, err := reader.ReadSecretKey(context.Background(), "e16-mcp-token", "token")
	if err != nil || string(value) != fakeSecretToken {
		t.Fatalf("value %q err %v", value, err)
	}
	recorded, _ := os.ReadFile(args)
	if got := strings.Fields(string(recorded)); strings.Join(got, " ") != "get secret e16-mcp-token --namespace agenova-e16-system --request-timeout=5s -o json" {
		t.Fatalf("kubectl arguments %v", got)
	}
	// A key with a dot is looked up in Go, not through a jsonpath expression.
	if value, err := reader.ReadSecretKey(context.Background(), "e16-mcp-token", "token.v2"); err != nil || string(value) != "second" {
		t.Fatalf("dotted key: %q %v", value, err)
	}
}

func TestKubectlSecretReaderFailsWithOneConstantError(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(fakeSecretToken))
	for name, body := range map[string]string{
		"not found":      "echo 'Error from server (NotFound): secrets \"e16-mcp-token-absent\" not found " + fakeSecretToken + "' >&2; exit 1",
		"forbidden":      "echo 'Error from server (Forbidden): cannot get resource secrets' >&2; exit 1",
		"warning stdout": "echo 'W1004 warning'; echo '{\"kind\":\"Secret\",\"data\":{\"token\":\"" + encoded + "\"}}'",
		"missing key":    "echo '{\"kind\":\"Secret\",\"data\":{\"other\":\"" + encoded + "\"}}'",
		"wrong kind":     "echo '{\"kind\":\"ConfigMap\",\"data\":{\"token\":\"" + encoded + "\"}}'",
		"bad base64":     "echo '{\"kind\":\"Secret\",\"data\":{\"token\":\"not base64!\"}}'",
		// Valid, with the right key, but larger than the bound.
		"oversized":         "printf '{\"kind\":\"Secret\",\"data\":{\"token\":\"" + encoded + "\"},\"pad\":\"'; head -c 1048577 /dev/zero | tr '\\0' 'a'; printf '\"}'",
		"exit after output": "echo '{\"kind\":\"Secret\",\"data\":{\"token\":\"" + encoded + "\"}}'; exit 3",
	} {
		t.Run(name, func(t *testing.T) {
			path, _ := fakeKubectl(t, body)
			value, err := kubectlSecretReader{namespace: "agenova-e16-system", kubectl: path}.ReadSecretKey(context.Background(), "e16-mcp-token", "token")
			if err != errTokenSecretUnavailable || value != nil {
				t.Fatalf("value %q err %v", value, err)
			}
		})
	}
	for _, namespace := range []string{"", "default"} {
		path, args := fakeKubectl(t, "exit 0")
		if _, err := (kubectlSecretReader{namespace: namespace, kubectl: path}).ReadSecretKey(context.Background(), "e16-mcp-token", "token"); err != errTokenSecretUnavailable {
			t.Fatalf("namespace %q: %v", namespace, err)
		}
		if _, err := os.Stat(args); err == nil {
			t.Fatalf("namespace %q still ran kubectl", namespace)
		}
	}
}

func TestKubectlSecretReaderStopsAtTheCallDeadline(t *testing.T) {
	path, _ := fakeKubectl(t, "sleep 30")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := (kubectlSecretReader{namespace: "agenova-e16-system", kubectl: path}).ReadSecretKey(ctx, "e16-mcp-token", "token"); err != errTokenSecretUnavailable {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("reader outlived its deadline by %v", elapsed)
	}
}
