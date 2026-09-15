// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package toolclient is the only tool-access boundary for the example
// engineer worker. The worker never imports provider SDKs or gateway
// packages; every tool interaction goes through ToolClient so the boundary
// can later be rebound to the real Tool Gateway (E4-T2) without changing
// worker logic.
package toolclient

// ToolClient is the sole path through which the engineer worker invokes a
// tool against a resource scope. Implementations decide whether the call is
// mocked or governed; the worker cannot tell the difference and never holds
// credentials either way.
type ToolClient interface {
	Invoke(tool, scope string) error
}
