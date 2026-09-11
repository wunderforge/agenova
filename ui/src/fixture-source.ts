// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { ClaimRequest, IssuedState } from './contracts.generated';
import type { Diagnostic, EvidenceResult, EvidenceSource } from './evidence-source';
import { shapeDiagnostics } from './shape-check';

// Input is the canonical-parser output from the build plugin, never raw caller
// data. A future HTTP adapter must establish its own trusted validation boundary.
export interface FixtureRow {
  id: string;
  subject: 'ClaimRequest' | 'IssuedState';
  input: string;
  format: string;
  equivalentTo?: string;
  expected: { outcome: string; category?: string };
  data?: unknown;
  diagnostic?: Diagnostic;
  derivedFrom?: string;
}

export function createFixtureSource(rows: readonly FixtureRow[]): EvidenceSource {
  const byID = new Map(rows.map(row => [row.id, structuredClone(row)]));
  return {
    async load(key): Promise<EvidenceResult> {
      const row = byID.get(key);
      if (!row) return { status: 'not-found' };
      if (row.diagnostic) return { status: 'invalid', diagnostics: [row.diagnostic] };
      const diagnostics = shapeDiagnostics(row.subject, row.data);
      if (diagnostics.length) return { status: 'invalid', diagnostics };
      // Cast only after generated shape validation, on canonical-parser output.
      // Clone prevents one consumer from changing subsequent source responses.
      return row.subject === 'ClaimRequest'
        ? { status: 'request', data: structuredClone(row.data) as ClaimRequest }
        : { status: 'issued', data: structuredClone(row.data) as IssuedState };
    },
  };
}

// Explicit display-corruption scenarios derived in memory from the canonical
// positive case. No parallel payload files or fabricated governance states.
export function storyboardRows(canonical: readonly FixtureRow[]): FixtureRow[] {
  const base = canonical.find(row => row.id === 'issued-state.valid.team-a-engineer');
  if (!base?.data) throw new Error('canonical Team A case is unavailable');
  const missing = structuredClone(base.data) as Record<string, unknown>;
  delete missing.evidence;
  const unknown = { ...structuredClone(base.data) as object, futureField: true };
  return [...canonical, ...[
    { id: 'derived.missing-field', data: missing, input: 'delete evidence' },
    { id: 'derived.unknown-field', data: unknown, input: 'add futureField' },
  ].map(variant => ({ ...base, ...variant, derivedFrom: base.id }))];
}
