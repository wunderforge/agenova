// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

// ReferenceNamespaceRules is the opt-in worker adapter's minimum namespace
// access for the controlled reference path. Kubernetes deployment treats the
// returned objects as opaque; upstream API vocabulary stays in this adapter.
func ReferenceNamespaceRules() []any {
	return []any{
		map[string]any{"apiGroups": []any{""}, "resources": []any{"pods"}, "verbs": []any{"get"}},
		map[string]any{"apiGroups": []any{""}, "resources": []any{"pods/exec"}, "verbs": []any{"create", "get"}},
		map[string]any{"apiGroups": []any{"extensions.agents.x-k8s.io"}, "resources": []any{"sandboxtemplates", "sandboxwarmpools", "sandboxclaims"}, "verbs": []any{"get", "create", "patch", "delete"}},
		map[string]any{"apiGroups": []any{"agents.x-k8s.io"}, "resources": []any{"sandboxes"}, "verbs": []any{"get"}},
	}
}
