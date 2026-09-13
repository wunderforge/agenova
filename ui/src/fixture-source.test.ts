// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import rows from 'virtual:agenova-fixtures';
import { createFixtureSource, storyboardRows } from './fixture-source';
import { shapeDiagnostics } from './shape-check';
import type { IssuedState } from './contracts.generated';

const source = createFixtureSource(rows);
describe('canonical fixture source', () => {
  it.each(rows)('$id matches the canonical parser and manifest', async row => {
    const result = await source.load(row.id);
    if (row.expected.outcome === 'invalid') {
      expect(result.status).toBe('invalid');
      if (result.status === 'invalid') expect(result.diagnostics[0].category).toBe(row.expected.category);
      expect(row.data).toBeUndefined();
    } else expect(result.status).toBe(row.subject === 'ClaimRequest' ? 'request' : 'issued');
  });
  it('covers 7 requests and 5 issued cases; YAML and JSON normalize equally', async () => {
    expect(rows.filter(row => row.subject === 'ClaimRequest')).toHaveLength(7);
    expect(rows.filter(row => row.subject === 'IssuedState')).toHaveLength(5);
    expect(await source.load('claim-request.valid.team-a-engineer-yaml')).toEqual(await source.load('claim-request.valid.team-a-engineer-json'));
  });
  it('preserves Allow/Running and pre-claim denial without fabricating authority', async () => {
    const allow = await source.load('issued-state.valid.team-a-engineer');
    const deny = await source.load('issued-state.valid.team-b-denial');
    expect(allow.status).toBe('issued'); expect(deny.status).toBe('issued');
    if (allow.status === 'issued' && deny.status === 'issued') {
      expect(allow.data.claim?.phase).toBe('Running');
      expect(allow.data.decision.result).toBe('Allow');
      expect(allow.data.claim?.authorityRef).toBe(allow.data.effectiveAuthority?.id);
      expect(deny.data.decision.result).toBe('Deny');
      expect(deny.data.claim).toBeUndefined(); expect(deny.data.effectiveAuthority).toBeUndefined();
      expect(deny.data.evidence.claimId).toBeUndefined();
    }
  });
  it('does not leak mutation between consumers and reports unknown selections', async () => {
    const first = await source.load('issued-state.valid.team-a-engineer');
    if (first.status === 'issued') first.data.decision.reason = 'changed';
    const second = await source.load('issued-state.valid.team-a-engineer');
    if (second.status === 'issued') expect(second.data.decision.reason).not.toBe('changed');
    expect(await source.load('missing')).toEqual({ status: 'not-found' });
  });
  it('labels in-memory corruptions and reports exact missing/unknown paths', async () => {
    const derived = storyboardRows(rows);
    const source = createFixtureSource(derived);
    expect(derived.at(-1)?.derivedFrom).toBe('issued-state.valid.team-a-engineer');
    expect(await source.load('derived.missing-field')).toEqual({ status: 'invalid', diagnostics: [{ category: 'missing-source-field', fieldPath: 'evidence' }] });
    expect(await source.load('derived.unknown-field')).toEqual({ status: 'invalid', diagnostics: [{ category: 'unknown-field', fieldPath: 'futureField' }] });
  });
});

describe('generated shape validation (not policy evaluation)', () => {
  const original = rows.find(row => row.id === 'issued-state.valid.team-a-engineer')!.data as IssuedState;
  it.each(['decision.result', 'claim.phase'])('rejects an unknown enum at %s', path => {
    const data = structuredClone(original);
    if (path === 'decision.result') Object.assign(data.decision, { result: 'FutureAllow' });
    else Object.assign(data.claim!, { phase: 'FutureRunning' });
    expect(shapeDiagnostics('IssuedState', data)).toContainEqual({ category: 'invalid-value', fieldPath: path });
  });
  it.each(['runtimeEvents', 'toolInvocations', 'modelInvocations'] as const)('distinguishes absent/null %s from recorded empty observations', field => {
    for (const absent of [undefined, null]) {
      const data = structuredClone(original);
      Object.assign(data.evidence, { [field]: absent });
      expect(shapeDiagnostics('IssuedState', data)).toContainEqual({ category: 'missing-source-field', fieldPath: `evidence.${field}` });
    }
    expect(shapeDiagnostics('IssuedState', original)).toEqual([]);
  });
  it('retains arbitrary JSON task input while rejecting unknown contract keys', () => {
    const data = structuredClone(rows.find(row => row.id === 'claim-request.valid.team-a-engineer-json')!.data) as { spec: { task: { input: unknown } } };
    data.spec.task.input = { custom: [null, { nested: true }, 42] };
    expect(shapeDiagnostics('ClaimRequest', data)).toEqual([]);
    Object.assign(data.spec.task, { unknownField: true });
    expect(shapeDiagnostics('ClaimRequest', data)).toContainEqual({ category: 'unknown-field', fieldPath: 'spec.task.unknownField' });
  });
});
