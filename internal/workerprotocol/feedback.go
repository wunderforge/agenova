// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import "strings"

func ValidFeedback(op Operation) bool {
	if op.Profile != "" || op.Tool != "" || op.ResourceScope != "" || op.Prompt != "" || op.Native != nil || len(op.Input) > 512 || strings.ContainsAny(op.Input, "\x00\r") {
		return false
	}
	switch op.DiagnosticCode {
	case "synthetic-label-missing", "schema-invalid", "citation-missing", "evidence-missing", "finish-accepted", "tool-selected", "unrelated-tool", "tool-repeated", "unsupported-finish", "format-retry-limit":
		return true
	}
	return false
}
