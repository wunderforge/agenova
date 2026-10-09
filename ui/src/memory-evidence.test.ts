// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it, vi } from 'vitest';
import vectors from '../../work/0179-scoped-memory/memory-reader-vectors.json';
import admissionTraces from '../../work/0179-scoped-memory/memory-admission-traces.json';
import { connectedSource, type View } from './connected-source';
import { validMemoryEvidence } from './memory-evidence';
import { shapeDiagnostics } from './shape-check';
import type { ClaimPhase, Fact } from './contracts.generated';

const base = vectors.view as View;
afterEach(() => vi.unstubAllGlobals());
it.each(admissionTraces.map((view, index) => ({view: view as View, index})))('accepts lifecycle/Memory producer trace $index', async ({view}) => {
  expect(shapeDiagnostics('View', view)).toEqual([]);
  expect(validMemoryEvidence(view)).toBe(true);
  vi.stubGlobal('fetch', vi.fn(async (url: string) => new Response(JSON.stringify(url === '/api/requests' ? [view] : view))));
  expect((await connectedSource.request(view.requestRef)).outcome?.status).toBe(view.outcome?.status);
  expect(await connectedSource.list()).toHaveLength(1);
});
it('accepts the same public Memory view as the CLI reader', async () => {
  expect(shapeDiagnostics('View', base)).toEqual([]);
  vi.stubGlobal('fetch', vi.fn(async (_url: string, init: RequestInit) => new Response(JSON.stringify(init.method === 'POST' ? base : [base]))));
  expect(await connectedSource.list()).toHaveLength(1);
  expect((await connectedSource.submit(base.request)).facts[8].memory?.status).toBe('Written');
});
it('requires explicit empty evidence arrays, not null or missing histories', () => {
  expect(shapeDiagnostics('Evidence', base.state!.evidence)).toEqual([]);
  const {modelInvocations: _models, ...missing} = base.state!.evidence;
  expect(shapeDiagnostics('Evidence', missing)).toContainEqual({category:'missing-source-field',fieldPath:'modelInvocations'});
  expect(shapeDiagnostics('Evidence', {...base.state!.evidence,modelInvocations:null})).toContainEqual({category:'missing-source-field',fieldPath:'modelInvocations'});
});
it.each(vectors.corruptions)('rejects shared corruption: $name', async ({path, value}) => {
  const view = structuredClone(base);
  let current: any = view;
  for (const part of path.slice(0, -1)) current = current[part];
  current[path.at(-1)!] = value;
  vi.stubGlobal('fetch', vi.fn(async (url: string) => new Response(JSON.stringify(url === '/api/requests' ? [view] : view))));
  await expect(connectedSource.request(base.requestRef)).rejects.toMatchObject({status: 502});
  await expect(connectedSource.list()).rejects.toMatchObject({status: 502});
});
it.each(vectors.denialLifecycles)('requires prior Running for denial: $name', async ({phase, events, accept}) => {
  const view = structuredClone(base), deny = view.facts[12];
  view.state!.claim!.phase = phase as ClaimPhase;
  view.state!.evidence.runtimeEvents = [];
  view.facts = view.facts.slice(0,3);
  for (const event of events) {
    const fact: Fact = event === 'Deny' ? {...deny} : {kind:'Runtime',operation:event,
      requestRef:view.requestRef,claimId:view.state!.claim!.id,id:'',sequence:0,timestamp:deny.timestamp};
    if (event !== 'Deny') {
      view.state!.evidence.runtimeEvents.push({kind:event});
      if (event === 'Bound') fact.backendIdentity = view.state!.claim!.backendIdentity;
    }
    fact.id = `fact:lifecycle:${view.facts.length + 1}`; fact.sequence = view.facts.length + 1;
    view.facts.push(fact);
  }
  expect(shapeDiagnostics('View', view)).toEqual([]);
  expect(validMemoryEvidence(view)).toBe(accept);
  vi.stubGlobal('fetch', vi.fn(async (url: string) => new Response(JSON.stringify(url === '/api/requests' ? [view] : view))));
  if (accept) {
    expect((await connectedSource.request(view.requestRef)).facts.at(-1)?.result).toBe('Deny');
    expect(await connectedSource.list()).toHaveLength(1);
  } else {
    await expect(connectedSource.request(view.requestRef)).rejects.toMatchObject({status:502});
    await expect(connectedSource.list()).rejects.toMatchObject({status:502});
  }
});
it.each(['Empty','Unavailable','Unsupported','Timeout','Cancelled','Failed','WriteUncertain'])('keeps %s distinct', status => {
  const view = structuredClone(base), f = view.facts[8];
  if (status === 'Empty') for (const fact of view.facts.slice(6,9)) fact.operation = 'memory.read';
  f.providerStatus = status === 'Empty' ? 'Succeeded' : status === 'Cancelled' ? 'Cancelled' : 'Failed';
  f.reasonCode = status === 'Empty' ? 'memory-completed' : status === 'WriteUncertain' ? 'memory-write-uncertain' : `memory-${status.toLowerCase()}`;
  f.memory = {status,count:0,durationMilliseconds:2,truncated:false};
  expect(validMemoryEvidence(view)).toBe(true);
});
it('accepts in-progress calls but rejects terminal dangling invocations and list identity reuse', async () => {
  const active = structuredClone(base);
  active.facts.splice(8);
  expect(validMemoryEvidence(active)).toBe(true);
  active.outcome = {status:'Succeeded'};
  expect(validMemoryEvidence(active)).toBe(false);
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify([base,base]))));
  await expect(connectedSource.list()).rejects.toMatchObject({status:502});
});
it.each(['Written','WriteUncertain','Cancelled','Timeout','Found'])('preserves post-cancellation %s semantics', status => {
  const view = structuredClone(base), result = structuredClone(view.facts[8]);
  view.facts = view.facts.slice(0,8);
  view.state!.claim!.phase = 'Failed';
  view.state!.evidence.runtimeEvents!.push({kind:'Cancelled'});
  view.facts.push({id:'fact:cancelled',sequence:9,timestamp:result.timestamp,kind:'Runtime',requestRef:view.requestRef,claimId:view.state!.claim!.id,operation:'Cancelled'});
  result.sequence = 10;
  result.memory = {status,count:status === 'Written' || status === 'Found' ? 1 : 0,durationMilliseconds:1,truncated:false,
    references:status === 'Written' || status === 'Found' ? ['memory:00000000000000000000000000000001'] : []};
  result.providerStatus = status === 'Written' || status === 'Found' ? 'Succeeded' : status === 'Cancelled' ? 'Cancelled' : 'Failed';
  result.reasonCode = status === 'Written' || status === 'Found' ? 'memory-completed' : status === 'WriteUncertain' ? 'memory-write-uncertain' : `memory-${status.toLowerCase()}`;
  if (['Cancelled','Timeout','Found'].includes(status)) for (const f of [...view.facts.slice(6,8),result]) f.operation = 'memory.read';
  view.facts.push(result);
  expect(validMemoryEvidence(view)).toBe(status !== 'Found');
  if (status === 'Found') return;
  const deny: Fact = {...view.facts[6],id:'fact:late-deny',sequence:11,invocationId:'inv:late-deny',result:'Deny',reasonCode:'memory-claim-inactive'};
  view.facts.push(deny);
  expect(validMemoryEvidence(view)).toBe(true);
  deny.result = 'Allow'; deny.reasonCode = 'memory-allowed';
  expect(validMemoryEvidence(view)).toBe(false);
});
it('rejects mismatched requested identity and reused Memory invocation across independent Work', async () => {
  expect(validMemoryEvidence(base,'other')).toBe(false);
  const other = structuredClone(base);
  other.requestRef = other.request.metadata.name = other.state!.requestRef = other.state!.claim!.requestRef = other.state!.evidence.requestRef = 'other';
  other.state!.claim!.id = 'claim:other';
  other.state!.claim!.authorityRef = other.state!.effectiveAuthority!.id = 'authority:other';
  other.state!.claim!.backendIdentity!.workerId = 'worker:other';
  for (const f of other.facts) {
    f.requestRef = 'other'; f.id = `other:${f.id}`; f.sequence += 100;
    if (f.claimId) f.claimId = 'claim:other';
    if (f.effectiveAuthority) f.effectiveAuthority.id = 'authority:other';
    if (f.backendIdentity) f.backendIdentity.workerId = 'worker:other';
  }
  expect(validMemoryEvidence(other)).toBe(true);
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify([base,other]))));
  await expect(connectedSource.list()).rejects.toMatchObject({status:502});
  for (const f of other.facts) if (f.invocationId) f.invocationId = `other:${f.invocationId}`;
  other.state!.claim!.backendIdentity = {workerId:base.state!.claim!.backendIdentity!.workerId,backend:base.state!.claim!.backendIdentity!.backend};
  other.facts[3].backendIdentity = structuredClone(other.state!.claim!.backendIdentity);
  expect(validMemoryEvidence(other)).toBe(true);
  await expect(connectedSource.list()).rejects.toMatchObject({status:502});
});
