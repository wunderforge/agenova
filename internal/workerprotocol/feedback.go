// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import "strings"

func ValidFeedback(op Operation) bool {
	if op.Profile != "" || op.Tool != "" || op.ResourceScope != "" || op.Prompt != "" || op.Native != nil || len(op.Input) > 512 || strings.ContainsAny(op.Input, "\x00\r") {
		return false
	}
	return FeedbackReason(op.DiagnosticCode) != ""
}

// FeedbackReason is the canonical public description, not arbitrary Agent text.
// Producers and strict evidence consumers share the same bounded vocabulary.
func FeedbackReason(code string) string {
	switch code {
	case "synthetic-label-missing":
		return "The candidate answer omitted the source's synthetic-data label."
	case "schema-invalid":
		return "The Agent rejected a malformed model action."
	case "citation-missing":
		return "The candidate answer did not cite the observed source."
	case "evidence-missing":
		return "The candidate answer was not supported by available observations."
	case "finish-accepted":
		return "The Agent accepted its final answer and is finishing."
	case "tool-selected":
		return "The Agent accepted the proposed tool request."
	case "unrelated-tool":
		return "The Agent rejected a tool unrelated to this objective."
	case "tool-repeated":
		return "The Agent rejected a repeated successful read."
	case "unsupported-finish":
		return "The Agent rejected inconsistent completion metadata."
	case "format-retry-limit":
		return "The Agent stopped repeated invalid actions."
	}
	return ""
}
