// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it, vi } from 'vitest';
import { connectedSource, suggestedProjects, type Setup, type View } from './connected-source';

afterEach(() => vi.unstubAllGlobals());

it('accepts only explicit empty projections for Memory-requesting Work', async () => {
  const record: View = {
    version: 'agenova.evidence/v0', requestRef: 'private-test',
    request: {
      apiVersion: 'agenova.io/v1alpha1', kind: 'ClaimRequest', metadata: { name: 'private-test' },
      spec: {
        templateRef: 'engineer', projectRef: 'payments', task: { type: 'investigation', input: {} },
        requestedAccess: { memoryOperations: ['read'] }, runtime: { profileRef: 'standard-isolated', timeout: '1m' },
      },
    },
    facts: [], outcome: { status: 'Succeeded' },
    contentRedactions: ['request.spec.task.input', 'outcome.text'],
  };
  let response = record;
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(response), { status: 200 })));
  expect((await connectedSource.request('private-test')).request.spec.task?.input).toEqual({});
  for (const mutate of [
    (v: View) => { v.contentRedactions = undefined; },
    (v: View) => { v.contentRedactions = ['request.spec.task.input']; },
    (v: View) => { v.contentRedactions = ['request.spec.task.input', 'unknown.path']; },
    (v: View) => { v.contentRedactions = ['outcome.text', 'outcome.text']; },
    (v: View) => { v.request.spec.task!.input = { objective: 'private-input-sentinel' }; },
    (v: View) => { v.request.spec.task!.input = undefined; },
    (v: View) => { v.outcome!.text = 'private-result-sentinel'; },
    (v: View) => { v.request.spec.requestedAccess = {}; },
  ]) {
    response = structuredClone(record);
    mutate(response);
    await expect(connectedSource.request('private-test')).rejects.toMatchObject({ status: 502 });
    await expect(connectedSource.list()).rejects.toMatchObject({ status: 502 });
  }
  response = structuredClone(record);
  response.request.spec.requestedAccess = { memoryScopes: ['team-docs'] };
  expect((await connectedSource.request('private-test')).contentRedactions).toHaveLength(2);
});

it('rejects unrelated Memory metadata and unbound decisions on legacy evidence', async () => {
  const record: View = {
    version: 'agenova.evidence/v0', requestRef: 'synthetic',
    request: {
      apiVersion: 'agenova.io/v1alpha1', kind: 'ClaimRequest', metadata: { name: 'synthetic' },
      spec: {
        templateRef: 'engineer', projectRef: 'payments', task: { type: 'investigation', input: {} },
        requestedAccess: {}, runtime: { profileRef: 'standard-isolated', timeout: '1m' },
      },
    },
    facts: [{ id: 'fact:1', sequence: 1, timestamp: '2026-10-08T00:00:00Z', kind: 'RequestReceived', requestRef: 'synthetic' }],
  };
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(record), { status: 200 })));
  expect((await connectedSource.request('synthetic')).requestRef).toBe('synthetic');
  record.facts[0].memory = { status: 'Empty', count: 0, durationMilliseconds: 0, truncated: false };
  await expect(connectedSource.request('synthetic')).rejects.toMatchObject({ status: 502 });
  record.facts[0].memory = undefined;
  record.facts[0].kind = 'MemoryDecision';
  await expect(connectedSource.request('synthetic')).rejects.toMatchObject({ status: 502 });
});

it('accepts the installed API policy rule field names', async () => {
  const setup = {
    installation: { kind: 'installed', platform: 'reference-kind', revision: 'sha256:' + 'a'.repeat(64) },
    principal: { subject: 'operator', team: 'team-a', authenticationContext: 'local-reference' },
    template: {
      apiVersion: 'agenova.io/v1alpha1', kind: 'AgentTemplate', metadata: { name: 'engineer' },
      spec: { artifact: { image: 'example/agent:local' }, entrypoint: { command: ['/agent'] }, capabilityCeiling: {} },
    },
    policy: { ID: 'reference-default-deny', Version: '1', Rules: [{ team: 'team-a', action: 'claim.create', project: 'payments', templateRef: 'engineer' }] },
    capabilities: { runtime: 'available' },
  };
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(setup), { status: 200 })));
  expect((await connectedSource.setup()).policy.Rules[0].team).toBe('team-a');
});

it('suggests only projects for the current principal and registered template', () => {
  const setup = {
    principal: { team: 'team-a' }, template: { metadata: { name: 'engineer' } },
    policy: { Rules: [
      { team: 'team-a', action: 'claim.create', project: 'billing', templateRef: 'engineer' },
      { team: 'team-a', action: 'claim.create', project: 'billing', templateRef: 'engineer' },
      { team: 'team-b', action: 'claim.create', project: 'identity', templateRef: 'engineer' },
      { team: 'team-a', action: 'claim.delete', project: 'ops', templateRef: 'engineer' },
    ] },
  } as Setup;
  expect(suggestedProjects(setup)).toEqual(['billing']);
});

it('reserves the installed setup window for submission without extending reads', async () => {
  const timeout = vi.spyOn(AbortSignal, 'timeout');
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 503 })));
  await expect(connectedSource.submit({} as Parameters<typeof connectedSource.submit>[0])).rejects.toThrow();
  expect(timeout).toHaveBeenCalledWith(240_000);
  await expect(connectedSource.setup()).rejects.toThrow();
  expect(timeout).toHaveBeenLastCalledWith(8_000);
  timeout.mockRestore();
});
