// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package runtime

// ClaimReader is the narrow compatibility view the Tool and Model gateways
// need to preserve their existing Running-only Authorize behavior.
//
// It is implemented by the existing claim state owner (the in-memory
// reference runtime). It is NOT part of the RuntimeBackend contract and is not
// a promise that backend observation supplies governance authority. Ticket #31
// owns the authoritative run-service view and Ticket #32 binds gateway
// eligibility to that lifecycle; both replace or adapt this reader.
type ClaimReader interface {
	Claim(name string) (BackendClaim, bool)
}
