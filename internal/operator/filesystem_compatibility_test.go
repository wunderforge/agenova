// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureExportLimit = 1 << 20

// TestFilesystemLocalCompatibility uses real git and Go commands inside a
// controlled temporary directory. It proves ordinary-tool compatibility, not
// isolation from a hostile process; the reusable model suite records boundary
// denials separately.
func TestFilesystemLocalCompatibility(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	collector := filepath.Join(root, "collector")
	outside := filepath.Join(root, "outside-sentinel.txt")
	for _, directory := range []string{workspace, collector, filepath.Join(workspace, ".home"), filepath.Join(workspace, ".cache"), filepath.Join(workspace, ".tmp")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixtureFile(t, outside, []byte("outside-unchanged\n"))
	writeFixtureFile(t, filepath.Join(workspace, "go.mod"), []byte("module example.local/fsfixture\n\ngo 1.22\n"))
	writeFixtureFile(t, filepath.Join(workspace, "value.go"), []byte("package fsfixture\n\nfunc Value() int { return 1 }\n"))
	writeFixtureFile(t, filepath.Join(workspace, "value_test.go"), []byte("package fsfixture\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value() != 2 { t.Fatalf(\"Value = %d\", Value()) } }\n"))

	environment := fixtureEnvironment(workspace)
	gitVersion := strings.TrimSpace(string(runFixtureCommand(t, workspace, environment, "git", "--version")))
	goVersion := strings.TrimSpace(string(runFixtureCommand(t, workspace, environment, "go", "version")))
	runFixtureCommand(t, workspace, environment, "git", "init", "--quiet")
	runFixtureCommand(t, workspace, environment, "git", "-c", "user.name=Agenova Fixture", "-c", "user.email=fixture@invalid.example", "add", ".")
	runFixtureCommand(t, workspace, environment, "git", "-c", "user.name=Agenova Fixture", "-c", "user.email=fixture@invalid.example", "commit", "--quiet", "-m", "fixture baseline")

	writeFixtureFile(t, filepath.Join(workspace, "value.go"), []byte("package fsfixture\n\nfunc Value() int { return 2 }\n"))
	diff := runFixtureCommand(t, workspace, environment, "git", "diff", "--", "value.go")
	if !bytes.Contains(diff, []byte("return 2")) {
		t.Fatalf("git diff did not observe workspace edit:\n%s", diff)
	}
	runFixtureCommand(t, workspace, environment, "go", "test", "./...")

	writeFixtureFile(t, filepath.Join(workspace, "result.patch"), diff)
	outputCollector := &fixtureCollector{workspace: workspace, destination: collector, limit: fixtureExportLimit, active: true}
	exported, err := outputCollector.collect("result.patch")
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(diff)
	gotDigest := sha256.Sum256(exported)
	if gotDigest != wantDigest {
		t.Fatalf("export digest = %x, want %x", gotDigest, wantDigest)
	}
	if got := mustReadFile(t, outside); string(got) != "outside-unchanged\n" {
		t.Fatalf("outside sentinel changed: %q", got)
	}
	writeFixtureFile(t, filepath.Join(workspace, "unexported.txt"), []byte("must-not-export"))
	outputCollector.close()
	if _, err := outputCollector.collect("unexported.txt"); err == nil {
		t.Fatal("collector accepted output after closure")
	}
	if _, err := os.Stat(filepath.Join(collector, "unexported.txt")); !os.IsNotExist(err) {
		t.Fatalf("output attempted after collector closure reached destination: %v", err)
	}

	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace retained after cleanup: %v", err)
	}
	if got := mustReadFile(t, filepath.Join(collector, "result.patch")); !bytes.Equal(got, diff) {
		t.Fatal("pre-termination export did not survive workspace cleanup")
	}
	t.Logf("FS-P1/FS-P2 git=%q go=%q cwd=%s git_diff_bytes=%d command_exits=0 export_sha256=%x workspace_retained=false evidence=local-compatibility-not-isolation", gitVersion, goVersion, workspace, len(diff), gotDigest)
}

type fixtureCollector struct {
	workspace   string
	destination string
	limit       int64
	active      bool
}

func (c *fixtureCollector) close() { c.active = false }

func (c *fixtureCollector) collect(relative string) ([]byte, error) {
	if !c.active {
		return nil, fmt.Errorf("fixture output collector is closed")
	}
	return collectFixtureOutput(c.workspace, c.destination, relative, c.limit)
}

func collectFixtureOutput(workspace, collector, relative string, limit int64) ([]byte, error) {
	cleaned := filepath.Clean(relative)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("fixture output must be a relative path inside the workspace")
	}
	source := filepath.Join(workspace, cleaned)
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("fixture output must be a bounded regular non-link file")
	}
	links, err := regularFileLinkCount(source, info)
	if err != nil {
		return nil, err
	}
	if links != 1 {
		return nil, fmt.Errorf("fixture output must not be hard-linked")
	}
	if err := rejectFixtureAliases(workspace, cleaned); err != nil {
		return nil, err
	}
	resolvedRoot, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	resolvedSource, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedSource)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("fixture output escaped the workspace")
	}
	data, err := os.ReadFile(resolvedSource)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(collector, filepath.Base(cleaned)), data, 0o600); err != nil {
		return nil, err
	}
	return data, nil
}

func rejectFixtureAliases(workspace, relative string) error {
	current := workspace
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture output path contains a link")
		}
	}
	return nil
}

func fixtureEnvironment(workspace string) []string {
	home := filepath.Join(workspace, ".home")
	cache := filepath.Join(workspace, ".cache")
	temp := filepath.Join(workspace, ".tmp")
	environment := make([]string, 0, 16)
	for _, key := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "SystemRoot", "COMSPEC", "ComSpec", "WINDIR", "windir"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return append(environment,
		"HOME="+home,
		"USERPROFILE="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"GOCACHE="+filepath.Join(cache, "go-build"),
		"GOMODCACHE="+filepath.Join(cache, "go-mod"),
		"GOENV=off",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
		"TMPDIR="+temp,
		"TEMP="+temp,
		"TMP="+temp,
	)
}

func runFixtureCommand(t *testing.T, directory string, environment []string, name string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
	t.Logf("command cwd=%s argv=%q exit=0 output=%q", directory, append([]string{name}, arguments...), output)
	return output
}

func writeFixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCollectFixtureOutputRejectsEscapeLinkAndOversize(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	collector := filepath.Join(root, "collector")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(collector, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(root, "outside.txt"), []byte("outside"))
	writeFixtureFile(t, filepath.Join(workspace, "large.bin"), bytes.Repeat([]byte{'x'}, 9))
	if err := os.Link(filepath.Join(root, "outside.txt"), filepath.Join(workspace, "hardlink.txt")); err != nil {
		t.Fatalf("create hard-link fixture: %v", err)
	}

	assertCollectorRejects(t, "escape", func() error { _, err := collectFixtureOutput(workspace, collector, "../outside.txt", 8); return err })
	assertCollectorRejects(t, "oversize", func() error { _, err := collectFixtureOutput(workspace, collector, "large.bin", 8); return err })
	assertCollectorRejects(t, "hardlink", func() error { _, err := collectFixtureOutput(workspace, collector, "hardlink.txt", 8); return err })
	if err := os.Symlink(filepath.Join(root, "outside.txt"), filepath.Join(workspace, "link.txt")); err != nil {
		t.Fatalf("create symbolic-link fixture: %v", err)
	}
	assertCollectorRejects(t, "symlink", func() error { _, err := collectFixtureOutput(workspace, collector, "link.txt", 8); return err })
	if entries, err := os.ReadDir(collector); err != nil || len(entries) != 0 {
		t.Fatalf("rejected export wrote collector data: entries=%v error=%v", entries, err)
	}
}

func TestFixtureEnvironmentExcludesCredentialVariables(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "synthetic-must-not-propagate")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "synthetic-must-not-propagate")
	t.Setenv("SSH_AUTH_SOCK", "synthetic-must-not-propagate")
	t.Setenv("GIT_ASKPASS", "synthetic-must-not-propagate")
	for _, entry := range fixtureEnvironment(t.TempDir()) {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "SSH_AUTH_SOCK", "GIT_ASKPASS":
			t.Fatalf("credential-bearing variable propagated: %s", key)
		}
	}
}

func assertCollectorRejects(t *testing.T, name string, fn func() error) {
	t.Helper()
	if err := fn(); err == nil {
		t.Fatalf("%s: expected rejection", name)
	}
}
