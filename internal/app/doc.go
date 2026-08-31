// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package app is the backend-neutral CLI composition root.
//
// It constructs application services for the agenova executable, including the
// in-memory reference RuntimeBackend. Provider adapters stay outside this
// package so command wiring cannot import Kubernetes types.
package app
