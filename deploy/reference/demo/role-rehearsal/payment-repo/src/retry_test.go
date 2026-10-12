// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package retry

import "testing"

func TestBackoffStaysInsideTotalDeadline(t *testing.T) {
	if ShouldRetry(5000, 4000, 2000, 1) {
		t.Fatal("a retry cannot start after its backoff crosses the total deadline")
	}
	if !ShouldRetry(5000, 1000, 2000, 1) {
		t.Fatal("a retry with remaining time should be possible")
	}
	if ShouldRetry(5000, 1000, 100, MaxAttempts) {
		t.Fatal("the attempt count ceiling still applies")
	}
}

func TestIdempotencyKeyIsStable(t *testing.T) {
	if got := IdempotencyKey("payment-42", 1); got != "payment-42" {
		t.Fatalf("first attempt key = %q", got)
	}
	if got := IdempotencyKey("payment-42", 2); got != "payment-42" {
		t.Fatalf("retry attempt key = %q", got)
	}
}
