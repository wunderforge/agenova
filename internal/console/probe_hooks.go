// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build agenovaprobe

package console

import (
	"errors"

	"github.com/wunderforge/agenova/internal/facts"
)

// WrapToolAppendForProbe lets an acceptance probe inject journal failures on
// the tool path of s. It is compiled only into binaries built with the
// agenovaprobe tag, never into the installed service. Call it before any
// Work starts on s; wrap receives the real append and returns its
// replacement, so facts the probe does not fail still reach the journal.
func WrapToolAppendForProbe(s *Service, wrap func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error)) error {
	if s == nil || wrap == nil {
		return errors.New("probe append wrapper requires a service and a wrapper")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) != 0 {
		return errors.New("probe append wrapper must be installed before any Work")
	}
	next := wrap(s.appendTool)
	if next == nil {
		return errors.New("probe append wrapper returned no append")
	}
	s.appendTool = next
	return nil
}
