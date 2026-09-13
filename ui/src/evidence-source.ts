// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { ClaimRequest, IssuedState } from './contracts.generated';

export interface Diagnostic { category: string; fieldPath: string }
export type EvidenceResult =
  | { status: 'request'; data: ClaimRequest }
  | { status: 'issued'; data: IssuedState }
  | { status: 'invalid'; diagnostics: Diagnostic[] }
  | { status: 'not-found' }
  | { status: 'unavailable' };

// Selection keys are opaque. The future source owns how to resolve them.
// This interface carries canonical data, not another governance representation.
export interface EvidenceSource { load(key: string): Promise<EvidenceResult> }
