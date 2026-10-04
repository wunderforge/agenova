// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// scanTokens returns synthetic token values, never real credentials.
func scanTokens() (valid, wrong string) {
	v := sha256.Sum256([]byte("e16 scan test: valid token"))
	w := sha256.Sum256([]byte("e16 scan test: wrong token"))
	return hex.EncodeToString(v[:]), hex.EncodeToString(w[:])
}

func tokenLines(fixture, valid, wrong string) string {
	return "valid-fixture " + fixture + "\nvalid " + valid + "\nwrong " + wrong + "\n"
}

// runScan runs scan mode and fails the test if its output holds any pattern
// of either test value or any of secrets. The failure never prints them.
func runScan(t *testing.T, stdin string, secrets []string, roots ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := scanMain(roots, strings.NewReader(stdin), &out, &errOut)
	checkNoSecrets(t, out.String()+errOut.String(), secrets)
	return code, out.String(), errOut.String()
}

func checkNoSecrets(t *testing.T, output string, secrets []string) {
	t.Helper()
	valid, wrong := scanTokens()
	for _, p := range newMatcher([]string{valid, wrong}).patterns {
		secrets = append(secrets, string(p))
	}
	for i, s := range secrets {
		if strings.Contains(output, s) {
			t.Fatalf("scan output holds secret %d or part of a token value", i)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// zipOf builds an archive whose entries use method (zip.Deflate or zip.Store).
func zipOf(t *testing.T, method uint16, entries map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestScanCleanTreeCounts(t *testing.T) {
	valid, wrong := scanTokens()
	a, b := t.TempDir(), t.TempDir()
	files := map[string][]byte{
		filepath.Join(a, "notes.txt"):                 []byte("nothing secret here\n"),
		filepath.Join(a, "logs", "control-plane.log"): []byte(`{"msg":"tool call failed","reasonCode":"tool-credential-rejected"}` + "\n"),
		filepath.Join(a, "empty"):                     nil,
		filepath.Join(b, "portal", "trace.zip"): zipOf(t, zip.Deflate, map[string]string{
			"trace/network.txt": "GET /healthz 200\n", "resources/page.html": "<html></html>\n",
		}),
	}
	total := 0
	for path, data := range files {
		writeFile(t, path, data)
		total += len(data)
	}
	code, out, errOut := runScan(t, tokenLines(valid, valid, wrong), nil, a, b)
	if code != 0 {
		t.Fatalf("clean tree exited %d: %s%s", code, out, errOut)
	}
	if want := fmt.Sprintf("scanned files=4 bytes=%d zip-entries=2 patterns=12 roots=2\n", total); out != want {
		t.Fatalf("stdout %q, want %q", out, want)
	}
}

// Every form of either value is found wherever it is planted, and named by
// file only.
func TestScanFindsEveryForm(t *testing.T) {
	valid, wrong := scanTokens()
	dir := t.TempDir()
	filler := strings.Repeat("evidence ", 40)
	// encoded returns a larger base64 blob in which value starts at a byte
	// offset congruent to offset modulo 3, after non-zero bytes (the
	// patterns are built with zero bytes before the value).
	encoded := func(enc *base64.Encoding, value string, offset int) string {
		blob := append(bytes.Repeat([]byte{0x5a, 0x7f, 0xc3}, 40+offset)[:120+offset], value...)
		return enc.EncodeToString(append(blob, bytes.Repeat([]byte{0xee}, 77)...))
	}
	// The read boundary falls inside the base64 form, which holds neither the
	// value nor a half of it, so only the overlap window can find it.
	whole := base64.StdEncoding.EncodeToString([]byte(valid))
	boundary := append(bytes.Repeat([]byte{'x'}, scanChunk-40), whole+strings.Repeat("x", 100)...)
	archive := zipOf(t, zip.Deflate, map[string]string{"resources/network.txt": filler + "Authorization: Bearer " + wrong + "\n" + filler, "resources/page.html": "<html></html>"})
	for _, p := range newMatcher([]string{valid, wrong}).patterns {
		if bytes.Contains(archive, p) {
			t.Fatal("the deflated archive holds a pattern in its raw bytes; the entry test would prove nothing")
		}
	}
	planted := map[string][]byte{
		"raw.log":                     []byte("Authorization: Bearer " + valid + "\n"),
		"cut.txt":                     []byte(filler + wrong[:32] + "...\n"),
		"tail.txt":                    []byte(filler + "..." + valid[32:] + "\n"),
		"b64-offset-0.json":           []byte(`{"body":"` + encoded(base64.StdEncoding, valid, 0) + `"}`),
		"b64-offset-1.json":           []byte(`{"body":"` + encoded(base64.StdEncoding, wrong, 1) + `"}`),
		"b64-offset-2.json":           []byte(`{"body":"` + encoded(base64.StdEncoding, valid, 2) + `"}`),
		"b64url-offset-1.json":        []byte(`{"body":"` + encoded(base64.URLEncoding, valid, 1) + `"}`),
		"large/boundary.bin":          boundary,
		"portal/trace.zip":            archive,
		"dump-" + wrong[:32] + ".txt": []byte("clean content, leaky name\n"),
		"clean.txt":                   []byte(filler + "\n"),
	}
	total := 0
	for name, data := range planted {
		writeFile(t, filepath.Join(dir, name), data)
		total += len(data)
	}
	code, out, errOut := runScan(t, tokenLines(valid, valid, wrong), nil, dir)
	if code != 1 {
		t.Fatalf("exited %d, want 1: %s%s", code, out, errOut)
	}
	var want []string
	for _, name := range []string{"raw.log", "cut.txt", "tail.txt", "b64-offset-0.json", "b64-offset-1.json", "b64-offset-2.json", "b64url-offset-1.json", "large/boundary.bin"} {
		want = append(want, "leak "+filepath.Join(dir, name))
	}
	want = append(want, "leak "+filepath.Join(dir, "portal/trace.zip")+"!resources/network.txt", "leak [name withheld: it contains a token pattern]")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	summary := lines[len(lines)-1]
	got := lines[:len(lines)-1]
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("leak lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if wantSummary := fmt.Sprintf("scanned files=%d bytes=%d zip-entries=2 patterns=12 roots=1", len(planted), total); summary != wantSummary {
		t.Fatalf("summary %q, want %q", summary, wantSummary)
	}
	if !strings.Contains(errOut, "[fail] scan: 10 token leaks") {
		t.Fatalf("stderr %q", errOut)
	}
}

// A link, an unreadable file or directory, or an archive that cannot be read
// would leave evidence unscanned, so each one stops the scan.
func TestScanRefusesWhatItCannotScan(t *testing.T) {
	valid, wrong := scanTokens()
	input := tokenLines(valid, valid, wrong)
	unprivileged := func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("file modes do not deny reads here")
		}
	}
	for name, tc := range map[string]struct {
		want  string
		setup func(t *testing.T, dir string) string // returns the root to scan
	}{
		"symbolic link in the tree": {"is a symbolic link", func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "a.txt"), []byte("a"))
			if err := os.Symlink(filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")); err != nil {
				t.Skip("cannot create a symbolic link here:", err)
			}
			return dir
		}},
		"root is a symbolic link": {"is not a directory", func(t *testing.T, dir string) string {
			if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
				t.Skip("cannot create a symbolic link here:", err)
			}
			return filepath.Join(dir, "link")
		}},
		"missing root": {"root ", func(t *testing.T, dir string) string { return filepath.Join(dir, "absent") }},
		"root is a file": {"is not a directory", func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "f"), nil)
			return filepath.Join(dir, "f")
		}},
		"unreadable zip": {"unreadable zip archive", func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "x.bin"), []byte("PK\x03\x04 not really"))
			return dir
		}},
		"corrupt zip entry": {"checksum error", func(t *testing.T, dir string) string {
			archive := zipOf(t, zip.Store, map[string]string{"log.txt": "stored entry content"})
			archive = bytes.Replace(archive, []byte("stored entry content"), []byte("stored entry CONTENT"), 1)
			writeFile(t, filepath.Join(dir, "trace.zip"), archive)
			return dir
		}},
		"unreadable file": {"open: permission denied", func(t *testing.T, dir string) string {
			unprivileged(t)
			writeFile(t, filepath.Join(dir, "secret.log"), []byte("x"))
			if err := os.Chmod(filepath.Join(dir, "secret.log"), 0); err != nil {
				t.Fatal(err)
			}
			return dir
		}},
		"unreadable directory": {"permission denied", func(t *testing.T, dir string) string {
			unprivileged(t)
			writeFile(t, filepath.Join(dir, "locked", "a.txt"), []byte("x"))
			if err := os.Chmod(filepath.Join(dir, "locked"), 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "locked"), 0o755) })
			return dir
		}},
	} {
		t.Run(name, func(t *testing.T) {
			root := tc.setup(t, t.TempDir())
			code, out, errOut := runScan(t, input, nil, root)
			if code != 2 || !strings.Contains(errOut, tc.want) {
				t.Fatalf("exited %d with %q, want 2 with %q", code, errOut, tc.want)
			}
			if strings.Contains(out, "scanned ") {
				t.Fatalf("an incomplete scan printed a summary: %q", out)
			}
		})
	}
}

// Malformed token input stops the scan before any file is read, and no
// message repeats what the input held.
func TestScanRejectsBadTokenInput(t *testing.T) {
	valid, wrong := scanTokens()
	other := strings.Repeat("ab", 32)
	upper := strings.ToUpper(valid)
	secrets := []string{valid, wrong, other, upper, valid[:63]}
	root := t.TempDir()
	for name, tc := range map[string]struct{ stdin, want string }{
		"empty input":                    {"", "token input is empty"},
		"blank line":                     {"valid-fixture " + valid + "\n\nvalid " + valid + "\nwrong " + wrong + "\n", "token input line 2 is blank"},
		"missing label":                  {"valid-fixture " + valid + "\nvalid " + valid + "\n", "token input has no wrong line"},
		"repeated label":                 {tokenLines(valid, valid, wrong) + "valid " + valid + "\n", "token input repeats valid"},
		"value without a label":          {valid + "\n", "token input line 1 is not"},
		"unknown label":                  {"token " + valid + "\n", "token input line 1 is not"},
		"uppercase hex":                  {tokenLines(upper, valid, wrong), "token input valid-fixture is not 64 lowercase hex characters"},
		"short value":                    {tokenLines(valid, valid, wrong[:63]), "token input wrong is not 64 lowercase hex characters"},
		"valid differs from the fixture": {tokenLines(valid, other, wrong), "valid differs from valid-fixture"},
		"wrong equals valid":             {tokenLines(valid, valid, valid), "token input: wrong equals valid"},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := runScan(t, tc.stdin, secrets, root)
			if code != 2 || out != "" || !strings.Contains(errOut, tc.want) {
				t.Fatalf("exited %d with stdout %q and stderr %q, want 2 with %q", code, out, errOut, tc.want)
			}
		})
	}
	if code, _, errOut := runScan(t, tokenLines(valid, valid, wrong), secrets); code != 2 || !strings.Contains(errOut, "usage: evidence scan") {
		t.Fatalf("no root exited %d: %q", code, errOut)
	}
}

// The self-test refuses a matcher that would miss a form, or that matches
// anything.
func TestScanSelfTestCatchesABrokenMatcher(t *testing.T) {
	valid, wrong := scanTokens()
	values := []string{valid, wrong}
	if err := newMatcher(values).selfTest(values); err != nil {
		t.Fatalf("the real matcher: %v", err)
	}
	without := func(drop func(value string) string) *matcher {
		m := newMatcher(values)
		kept := m.patterns[:0]
		for _, p := range m.patterns {
			if string(p) != drop(valid) && string(p) != drop(wrong) {
				kept = append(kept, p)
			}
		}
		m.patterns = kept
		return m
	}
	for name, tc := range map[string]struct {
		m    *matcher
		want string
	}{
		"no base64 form at offset 1": {without(func(v string) string { return base64Core(v, 1) }), "missed the standard base64 at offset 1"},
		"no first half":              {without(func(v string) string { return v[:32] }), "missed the first half"},
		"matches anything":           {&matcher{patterns: [][]byte{[]byte("-")}, longest: 1}, "a sample without any value matched"},
	} {
		if err := tc.m.selfTest(values); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
}
