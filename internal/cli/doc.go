// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package cli implements backend-neutral command behavior for the agenova executable.
//
// Command parsing, help, version, and usage errors stay here. Kubernetes and
// other provider types must not be imported by this package. Runtime backends
// are supplied by a RuntimeFactory from the composition root.
package cli
