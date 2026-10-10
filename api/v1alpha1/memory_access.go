// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "fmt"

const (
	MemoryRead  = "read"
	MemoryWrite = "write"
)

func validateMemoryOperations(path string, operations []string, category ValidationCategory) *ValidationError {
	seen := make(map[string]bool, len(operations))
	for i, operation := range operations {
		field := fmt.Sprintf("%s[%d]", path, i)
		if operation != MemoryRead && operation != MemoryWrite {
			return validationError(category, field, "must be read or write")
		}
		if seen[operation] {
			return validationError(category, field, "duplicate operation")
		}
		seen[operation] = true
	}
	return nil
}
