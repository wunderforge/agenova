// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"sort"

	"github.com/wunderforge/agenova/internal/platform"
)

// runtimeCredentialSecretNames is the complete name-scoped Secret allowlist
// required by the installed control plane. Values and keys never enter RBAC.
func runtimeCredentialSecretNames(resolved *platform.ResolvedPlatform) []string {
	names := mcpTokenSecretNames(resolved)
	seen := make(map[string]bool, len(names)+1)
	for _, name := range names {
		seen[name] = true
	}
	if name, _, ok := modelTokenSecret(resolved); ok && !seen[name] {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
