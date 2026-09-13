// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package gateway defines the shared governed-invocation contract consumed by
// the Tool and Model gateways: typed decision results, stable rejection
// categories, trusted invocation-identity issuance, and pre-adapter request
// validation helpers.
//
// The contract is backend-neutral and carries no provider credential material.
// Capability/resource-scope enforcement, model-profile enforcement, and
// approval storage/resume are layered on top by their own tickets; this
// package only fixes the shapes they agree on.
package gateway
