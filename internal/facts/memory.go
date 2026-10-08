// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"encoding/hex"
	"strings"
)

// MemoryMetadata carries only bounded observations, never retrieved content.
type MemoryMetadata struct {
	Status               string   `json:"status"`
	Count                int      `json:"count"`
	DurationMilliseconds int      `json:"durationMilliseconds"`
	Truncated            bool     `json:"truncated"`
	References           []string `json:"references,omitempty"`
}

func ValidMemoryReference(ref string) bool {
	if len(ref) != len("memory:")+32 || !strings.HasPrefix(ref, "memory:") {
		return false
	}
	id := strings.TrimPrefix(ref, "memory:")
	_, err := hex.DecodeString(id)
	return err == nil && strings.ToLower(id) == id
}

// ValidMemoryOutcome is shared by journal writers and evidence readers.
func ValidMemoryOutcome(m *MemoryMetadata, operation, providerStatus string) bool {
	if m == nil || m.Count < 0 || m.Count > 10 || m.DurationMilliseconds < 0 || m.Count != len(m.References) {
		return false
	}
	seen := make(map[string]bool, len(m.References))
	for _, ref := range m.References {
		if !ValidMemoryReference(ref) || seen[ref] {
			return false
		}
		seen[ref] = true
	}
	switch m.Status {
	case "Written":
		return operation == "memory.write" && providerStatus == "Succeeded" && m.Count == 1 && !m.Truncated
	case "Found":
		return operation == "memory.read" && providerStatus == "Succeeded" && (m.Count > 0 || m.Truncated)
	case "Empty":
		return operation == "memory.read" && providerStatus == "Succeeded" && m.Count == 0 && !m.Truncated
	case "Cancelled":
		return providerStatus == "Cancelled" && m.Count == 0 && !m.Truncated
	case "Unsupported", "Unavailable", "Timeout", "Failed", "WriteUncertain":
		return providerStatus == "Failed" && m.Count == 0 && !m.Truncated && (m.Status != "WriteUncertain" || operation == "memory.write")
	default:
		return false
	}
}
