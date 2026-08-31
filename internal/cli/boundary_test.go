// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionSourcesStayBackendNeutral(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	// Command behavior and shared contracts must stay free of provider
	// SDKs and adapter packages. cmd/agenova and internal/app are the
	// composition edge and may import a concrete adapter constructor.
	targets := []struct {
		rel       string
		forbidden []string
	}{
		{rel: "api/v1alpha1", forbidden: providerImports()},
		{rel: "internal/cli", forbidden: append(providerImports(),
			"github.com/wunderforge/agenova/internal/operator",
			"github.com/wunderforge/agenova/internal/app",
			"github.com/wunderforge/agenova/internal/sandbox",
		)},
	}

	for _, target := range targets {
		dir := filepath.Join(root, filepath.FromSlash(target.rel))
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			imports := parseImports(t, path)
			for _, imp := range imports {
				for _, forbidden := range target.forbidden {
					if imp == forbidden || strings.HasPrefix(imp, forbidden) {
						t.Errorf("%s imports %q", filepath.ToSlash(path[len(root)+1:]), imp)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", target.rel, err)
		}
	}
}

func providerImports() []string {
	return []string{
		"k8s.io/",
		"sigs.k8s.io/",
		"github.com/kubernetes-sigs/",
		"github.com/wunderforge/agenova/internal/runtime/agentsandbox",
	}
}

func parseImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var imports []string
	for _, spec := range file.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
