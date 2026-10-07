// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package retry

import "fmt"

const MaxAttempts = 3

// ShouldRetry decides whether another attempt may begin within one total
// deadline. This v2.7 implementation is intentionally regressed.
func ShouldRetry(totalDeadlineMillis, elapsedMillis, backoffMillis, attempt int) bool {
	return attempt < MaxAttempts && elapsedMillis < totalDeadlineMillis
}

// IdempotencyKey must stay stable across attempts for the same payment.
func IdempotencyKey(original string, attempt int) string {
	return fmt.Sprintf("%s-%d", original, attempt)
}
