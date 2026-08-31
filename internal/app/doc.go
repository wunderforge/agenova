// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package app is the CLI composition root.
//
// It constructs application services for the agenova executable, including the
// in-memory reference RuntimeBackend. This package may import a concrete
// adapter constructor; Kubernetes and provider types must still stay out of
// command behavior and shared application contracts.
package app
