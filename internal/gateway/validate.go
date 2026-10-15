// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
)

// NormalizeParameterKey applies the single documented normalization rule:
// lowercase, then remove "-" and "_".
func NormalizeParameterKey(key string) string {
	return v1alpha1.NormalizeCredentialFieldName(key)
}

// ReservedCredentialKey reports whether a request parameter key is one of the
// documented reserved credential names (for example "githubToken", matching
// the frozen claim-request.invalid.secret-value fixture).
func ReservedCredentialKey(key string) bool {
	return v1alpha1.ReservedCredentialFieldName(key)
}

// FindReservedCredentialKey returns the reserved credential key that a
// rejection reports. Go map iteration is unordered, so a request carrying more
// than one reserved key would otherwise name a different key — and produce a
// different Reason — on identical input. The matching keys are sorted and the
// first is returned, making the rejection deterministic.
func FindReservedCredentialKey(params map[string]string) (string, bool) {
	return v1alpha1.FindReservedCredentialFieldName(params)
}

// AmbiguousResourceScope reports whether a resource scope fails to name one
// bounded resource: empty scopes and wildcard scopes cannot be authorized
// against an exact grant.
func AmbiguousResourceScope(scope string) bool {
	trimmed := strings.TrimSpace(scope)
	return trimmed == "" || strings.Contains(trimmed, "*")
}

// ResourceScope is one exact backend-neutral resource identity. It carries no
// provider endpoint, credential, query, or SDK object. Provider adapters map
// this logical identity to their own request shapes behind the Gateway.
type ResourceScope struct {
	Kind     string
	Segments []string
}

// String returns the canonical external representation of a ResourceScope.
func (s ResourceScope) String() string {
	return s.Kind + ":" + strings.Join(s.Segments, "/")
}

// ParseResourceScope validates the closed logical-resource vocabulary used by
// governed tool calls. Capability ceilings may contain a terminal wildcard,
// but one invocation must always name one exact canonical resource.
func ParseResourceScope(value string) (ResourceScope, error) {
	if AmbiguousResourceScope(value) {
		return ResourceScope{}, fmt.Errorf("resource scope must name one exact resource")
	}
	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "\\?#") {
		return ResourceScope{}, fmt.Errorf("resource scope is not canonical")
	}
	kind, path, found := strings.Cut(value, ":")
	if !found || kind == "" || path == "" || strings.Contains(path, ":") {
		return ResourceScope{}, fmt.Errorf("resource scope must use kind:path form")
	}
	segments := strings.Split(path, "/")
	for _, segment := range segments {
		if !validResourceSegment(segment) {
			return ResourceScope{}, fmt.Errorf("resource scope contains an invalid segment")
		}
	}

	scope := ResourceScope{Kind: kind, Segments: append([]string(nil), segments...)}
	if err := validateResourceShape(scope); err != nil {
		return ResourceScope{}, err
	}
	if scope.String() != value {
		return ResourceScope{}, fmt.Errorf("resource scope is not canonical")
	}
	return scope, nil
}

func validResourceSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	for _, r := range segment {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateResourceShape(scope ResourceScope) error {
	wantSegments := func(count int) error {
		if len(scope.Segments) != count {
			return fmt.Errorf("resource kind %q requires %d path segments", scope.Kind, count)
		}
		return nil
	}

	switch scope.Kind {
	case "workitem":
		return wantSegments(3)
	case "repo":
		return wantSegments(2)
	case "repo-object":
		if len(scope.Segments) < 4 {
			return fmt.Errorf("resource kind %q requires owner, repository, object kind, and object reference", scope.Kind)
		}
		switch scope.Segments[2] {
		case "path", "commit", "merge-request":
			return nil
		default:
			return fmt.Errorf("unsupported repository object kind %q", scope.Segments[2])
		}
	case "kubernetes":
		if len(scope.Segments) != 3 && len(scope.Segments) != 5 {
			return fmt.Errorf("resource kind %q requires an exact namespace or namespaced object", scope.Kind)
		}
		if scope.Segments[1] != "namespace" {
			return fmt.Errorf("kubernetes resource must use the namespace boundary")
		}
		return nil
	case "metrics", "dashboard":
		return wantSegments(2)
	case "gitops":
		return wantSegments(3)
	default:
		return fmt.Errorf("unsupported resource kind %q", scope.Kind)
	}
}

// SupportedToolOperation reports whether operation is part of the bounded
// read-only vocabulary currently supported at the governed Tool Gateway.
// Mutation and provider-specific operations require a separately accepted
// contract; they fail closed here rather than reaching policy or an adapter.
func SupportedToolOperation(operation string) bool {
	switch operation {
	case "workitem.read",
		"repo.read", "repository.read", "git.read", "file.read", "commit.read", "mergerequest.read",
		"kubernetes.get", "kubernetes.list", "kubernetes.logs",
		"metrics.query", "dashboard.read", "gitopsapplication.read":
		return true
	default:
		return false
	}
}

// ForbiddenToolOperation identifies high-impact mutation operations that the
// read-only E21 slice must never dispatch, even if a malformed or overly broad
// authority snapshot contains the same string. Other legacy/custom operations
// remain governed by exact issued authority for backward compatibility.
func ForbiddenToolOperation(operation string) bool {
	switch operation {
	case "git.write", "kubernetes.delete", "kubernetes.patch",
		"gitopsapplication.sync", "terraform.apply":
		return true
	default:
		return false
	}
}

// ValidateToolOperationResource prevents independently granted operations and
// resources from being recombined into a nonsensical or unsafe provider call.
// It assumes the ResourceScope has already passed ParseResourceScope.
func ValidateToolOperationResource(operation string, scope ResourceScope) error {
	if !SupportedToolOperation(operation) {
		return fmt.Errorf("unsupported governed tool operation %q", operation)
	}
	kindIs := func(want string) error {
		if scope.Kind != want {
			return fmt.Errorf("operation %q requires resource kind %q", operation, want)
		}
		return nil
	}
	repoObjectIs := func(want string) error {
		if scope.Kind != "repo-object" || len(scope.Segments) < 4 || scope.Segments[2] != want {
			return fmt.Errorf("operation %q requires repository object kind %q", operation, want)
		}
		return nil
	}

	switch operation {
	case "workitem.read":
		return kindIs("workitem")
	case "repo.read", "repository.read":
		return kindIs("repo")
	case "git.read":
		if scope.Kind != "repo" && scope.Kind != "repo-object" {
			return fmt.Errorf("operation %q requires a repository resource", operation)
		}
		return nil
	case "file.read":
		if scope.Kind == "repo" {
			return nil
		}
		return repoObjectIs("path")
	case "commit.read":
		return repoObjectIs("commit")
	case "mergerequest.read":
		return repoObjectIs("merge-request")
	case "kubernetes.get":
		return kindIs("kubernetes")
	case "kubernetes.list":
		if err := kindIs("kubernetes"); err != nil {
			return err
		}
		if len(scope.Segments) != 3 {
			return fmt.Errorf("operation %q requires a namespace boundary", operation)
		}
		return nil
	case "kubernetes.logs":
		if err := kindIs("kubernetes"); err != nil {
			return err
		}
		if len(scope.Segments) != 5 || scope.Segments[3] != "pod" {
			return fmt.Errorf("operation %q requires one exact Pod", operation)
		}
		return nil
	case "metrics.query":
		return kindIs("metrics")
	case "dashboard.read":
		return kindIs("dashboard")
	case "gitopsapplication.read":
		return kindIs("gitops")
	default:
		return fmt.Errorf("unsupported governed tool operation %q", operation)
	}
}
