// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolclient

// FixtureModeLabel is printed by consumers before any mock invocation so no
// run output can be mistaken for live Agenova governance enforcement.
const FixtureModeLabel = "[FIXTURE MODE] tool calls are mocked — not live Agenova governance"

// Invocation records one mocked tool call in the order it was made.
type Invocation struct {
	Tool  string
	Scope string
}

// Mock is the fixture-mode ToolClient. It records every invocation and
// returns the configured result for the tool, or success when none is
// configured. It holds no credentials and reaches no external system.
type Mock struct {
	// Results maps a tool name to the error Invoke returns for it. A missing
	// entry means the mocked call succeeds.
	Results map[string]error

	invocations []Invocation
}

var _ ToolClient = (*Mock)(nil)

func (m *Mock) Invoke(tool, scope string) error {
	m.invocations = append(m.invocations, Invocation{Tool: tool, Scope: scope})
	if m.Results == nil {
		return nil
	}
	return m.Results[tool]
}

// Invocations returns a copy of the recorded calls in invocation order.
func (m *Mock) Invocations() []Invocation {
	return append([]Invocation(nil), m.invocations...)
}
