// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeSecretToken = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// fakeKubectlEnv makes the test binary act as kubectl, so the reader runs a
// real child process the same way on every OS (no shell script).
const fakeKubectlEnv = "AGENOVA_FAKE_KUBECTL_DIR"

func TestMain(m *testing.M) {
	if dir := os.Getenv(fakeKubectlEnv); dir != "" {
		os.Exit(runFakeKubectl(dir))
	}
	os.Exit(m.Run())
}

// fakeKubectlCase is what the stand-in writes and how it exits.
type fakeKubectlCase struct {
	Stdout, Stderr string
	Exit           int
	Sleep          time.Duration
}

// runFakeKubectl records its arguments and then replays the case.
func runFakeKubectl(dir string) int {
	if os.WriteFile(filepath.Join(dir, "args"), []byte(strings.Join(os.Args[1:], "\n")), 0o600) != nil {
		return 125
	}
	var c fakeKubectlCase
	data, err := os.ReadFile(filepath.Join(dir, "case.json"))
	if err != nil || json.Unmarshal(data, &c) != nil {
		return 125
	}
	time.Sleep(c.Sleep)
	fmt.Fprint(os.Stderr, c.Stderr)
	fmt.Fprint(os.Stdout, c.Stdout)
	return c.Exit
}

// fakeKubectl returns a kubectl path that replays c and the file where it
// records its arguments.
func fakeKubectl(t *testing.T, c fakeKubectlCase) (string, string) {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeKubectlEnv, dir)
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return path, filepath.Join(dir, "args")
}

func TestKubectlSecretReaderReadsOneKeyOfANamedSecret(t *testing.T) {
	object := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"e16-mcp-token"},"data":{"token":"` + base64.StdEncoding.EncodeToString([]byte(fakeSecretToken)) + `","token.v2":"` + base64.StdEncoding.EncodeToString([]byte("second")) + `"}}`
	path, args := fakeKubectl(t, fakeKubectlCase{Stderr: "Warning: ignored on stderr\n", Stdout: object + "\n"})
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
	secret := func(data string) string { return `{"kind":"Secret","data":{` + data + `}}` + "\n" }
	for name, c := range map[string]fakeKubectlCase{
		"not found":      {Stderr: `Error from server (NotFound): secrets "e16-mcp-token-absent" not found ` + fakeSecretToken + "\n", Exit: 1},
		"forbidden":      {Stderr: "Error from server (Forbidden): cannot get resource secrets\n", Exit: 1},
		"warning stdout": {Stdout: "W1004 warning\n" + secret(`"token":"`+encoded+`"`)},
		"missing key":    {Stdout: secret(`"other":"` + encoded + `"`)},
		"wrong kind":     {Stdout: `{"kind":"ConfigMap","data":{"token":"` + encoded + `"}}` + "\n"},
		"bad base64":     {Stdout: secret(`"token":"not base64!"`)},
		// Valid, with the right key, but larger than the bound.
		"oversized":         {Stdout: `{"kind":"Secret","data":{"token":"` + encoded + `"},"pad":"` + strings.Repeat("a", maxSecretObjectBytes+1) + `"}`},
		"exit after output": {Stdout: secret(`"token":"` + encoded + `"`), Exit: 3},
	} {
		t.Run(name, func(t *testing.T) {
			path, args := fakeKubectl(t, c)
			value, err := kubectlSecretReader{namespace: "agenova-e16-system", kubectl: path}.ReadSecretKey(context.Background(), "e16-mcp-token", "token")
			if err != errTokenSecretUnavailable || value != nil {
				t.Fatalf("value %q err %v", value, err)
			}
			// The stand-in ran, so the refusal came from the reader, not from
			// a fixture the OS could not execute.
			if _, err := os.Stat(args); err != nil {
				t.Fatalf("kubectl stand-in did not run: %v", err)
			}
		})
	}
	for _, namespace := range []string{"", "default"} {
		path, args := fakeKubectl(t, fakeKubectlCase{})
		if _, err := (kubectlSecretReader{namespace: namespace, kubectl: path}).ReadSecretKey(context.Background(), "e16-mcp-token", "token"); err != errTokenSecretUnavailable {
			t.Fatalf("namespace %q: %v", namespace, err)
		}
		if _, err := os.Stat(args); err == nil {
			t.Fatalf("namespace %q still ran kubectl", namespace)
		}
	}
}

func TestKubectlSecretReaderStopsAtTheCallDeadline(t *testing.T) {
	path, args := fakeKubectl(t, fakeKubectlCase{Sleep: 30 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	if _, err := (kubectlSecretReader{namespace: "agenova-e16-system", kubectl: path}).ReadSecretKey(ctx, "e16-mcp-token", "token"); err != errTokenSecretUnavailable {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("reader outlived its deadline by %v", elapsed)
	}
	if _, err := os.Stat(args); err != nil {
		t.Fatalf("kubectl stand-in did not run: %v", err)
	}
}
