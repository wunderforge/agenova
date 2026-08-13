// Package operator is the in-memory reference backend for the RuntimeBackend contract.
//
// Runtime is the reference implementation: deterministic, single-threaded, and
// used to pin down claim lifecycle semantics and run the backend-neutral
// contract test suite. It is not safe for concurrent use; add locking before
// any concurrent harness or controller-runtime integration.
//
// Swapping to a different backend (e.g. an Agent Sandbox adapter) must be
// invisible to application agents and must not require changes to
// application-facing Agenova APIs.
package operator

import "github.com/donozhang1992/agenova/internal/runtime"

// Compile-time assertion: Runtime must satisfy the RuntimeBackend contract.
var _ runtime.RuntimeBackend = (*Runtime)(nil)
