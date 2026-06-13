// Package operator contains reconcile and claim lifecycle logic.
//
// The Phase 0 Runtime is a deterministic, single-threaded local model used to
// pin down lifecycle semantics. It is not safe for concurrent use; add
// locking before any concurrent harness or controller-runtime integration.
package operator
