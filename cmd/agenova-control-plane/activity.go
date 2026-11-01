// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"github.com/wunderforge/agenova/internal/console"
)

// Host configuration explicitly opts into bounded, volatile response capture.
// Credentials are redacted by the store; current Work read access gates every GET.
// Restart/expiry makes old bodies unavailable, never reconstructed or fabricated.
func configureActivityCapture(options *console.Options, mode string) error {
	switch mode {
	case "", "disabled":
		return nil
	case "memory":
		if options.Access == nil {
			return errors.New("activity capture requires authenticated member access")
		}
		options.ActivityStore = console.NewMemoryActivityStore(16 << 20)
		return nil
	default:
		return errors.New("unsupported AGENOVA_ACTIVITY_CAPTURE mode")
	}
}
