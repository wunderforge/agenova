// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { createFixtureSource, type FixtureRow } from './fixture-source';
import type { EvidenceResult, EvidenceSource } from './evidence-source';
import type { Scenario } from './console-route';
import type { ConsoleReferenceKeys } from './console-load';

// Fixture-only key convention stays in the adapter and is injected at composition.
export const fixtureConsoleKeys: ConsoleReferenceKeys = {
  issued: route => `${route.kind}:${route.reference}`,
  request: reference => `request-document:${reference}`,
};

export function createConsoleFixtureSource(rows: readonly FixtureRow[], scenario: Scenario): EvidenceSource {
  const base = createFixtureSource(rows);
  return { async load(key): Promise<EvidenceResult> {
    for (const row of rows) {
      if (row.derivedFrom && !(scenario === 'narrowed' && row.id === 'derived.console.narrowed')) continue;
      if (scenario === 'narrowed' && row.subject === 'IssuedState' && !row.derivedFrom) continue;
      const result = await base.load(row.id);
      if (result.status === 'request' && key === fixtureConsoleKeys.request(result.data.metadata.name)) return result;
      if (result.status !== 'issued') continue;
      const state = result.data;
      if (key !== `requests:${state.requestRef}` && (!state.claim || key !== `claims:${state.claim.id}`)) continue;
      if (scenario === 'loading') return new Promise<EvidenceResult>(() => {}); // deliberate fixture, no timer/polling
      if (scenario === 'not-found') return { status: 'not-found' };
      if (scenario === 'unavailable') return { status: 'unavailable' };
      if (scenario === 'malformed') {
        const broken = structuredClone(state) as unknown as Record<string, unknown>;
        delete broken.evidence;
        return createFixtureSource([{ ...row, data: broken }]).load(row.id);
      }
      return result;
    }
    return { status: 'not-found' };
  } };
}
