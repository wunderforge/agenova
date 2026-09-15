// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, waitFor, act } from '@testing-library/react';
import rows from 'virtual:agenova-console-fixtures';
import { createConsoleFixtureSource, fixtureConsoleKeys } from './console-fixture-source';
import { consoleHref, parseConsoleRoute, type ConsoleRoute, type Scenario } from './console-route';
import { loadConsole as resolveConsole } from './console-load';
import { ClaimConsole as ConsolePage, ConsolePanel as Panel } from './ClaimConsole';
import type { EvidenceResult, EvidenceSource } from './evidence-source';

afterEach(cleanup);
const loadConsole = (source: EvidenceSource, route: ConsoleRoute) => resolveConsole(source, route, fixtureConsoleKeys);
const ClaimConsole = (props: { source: EvidenceSource; route: ConsoleRoute }) => <ConsolePage {...props} keys={fixtureConsoleKeys}/>;
const ConsolePanel = (props: { source: EvidenceSource; route: ConsoleRoute }) => <Panel {...props} keys={fixtureConsoleKeys}/>;
const route: ConsoleRoute = { kind: 'requests', reference: 'fix-payment-timeout', scenario: 'canonical' };
const source = createConsoleFixtureSource(rows, 'canonical');

describe('presentation routes and fixture references', () => {
  it.each(['request with spaces', 'claim:a/b', 'request:中文'])('round-trips opaque reference %s', reference => {
    const url = new URL(consoleHref('claims', reference, 'narrowed'), 'http://local');
    expect(parseConsoleRoute(url.pathname, url.search)).toEqual({ status: 'route', route: { kind: 'claims', reference, scenario: 'narrowed' } });
  });
  it.each(['/console/requests/%ZZ', '/console/requests/%00', '/console/requests/%20', '/console/projects/id', '/console/claims/', '/console/claims/a/b'])('rejects %s without a fallback', pathname => {
    expect(parseConsoleRoute(pathname, '')).toEqual({ status: 'malformed-route' });
  });
  it.each(['?scenario=live', '?poll=1', '?scenario=canonical&scenario=narrowed'])('rejects unsupported query %s', query => {
    expect(parseConsoleRoute('/console/requests/x', query).status).toBe('malformed-route');
  });
  it('loads the same assignment by request and claim, without borrowing denial request data', async () => {
    const byRequest = await loadConsole(source, route);
    const byClaim = await loadConsole(source, { ...route, kind: 'claims', reference: 'claim:fix-payment-timeout:1' });
    expect(byRequest).toEqual(byClaim);
    expect(byRequest.status).toBe('ready');
    const deny = await loadConsole(source, { ...route, reference: 'fix-payment-timeout-team-b' });
    if (deny.status !== 'ready') throw new Error('denial missing');
    expect(deny.request).toBeUndefined(); expect(deny.issued.claim).toBeUndefined();
    expect(deny.requestGap).toBe('Matching request document not supplied.');
  });
  it('returns not-found for unknown and wrong-kind references', async () => {
    expect(await loadConsole(source, { ...route, reference: 'absent' })).toEqual({ status: 'not-found' });
    expect(await loadConsole(source, { ...route, kind: 'claims' })).toEqual({ status: 'not-found' });
  });
  it('narrows only the labeled derived scenario and leaves canonical results unchanged', async () => {
    const derived = await loadConsole(createConsoleFixtureSource(rows, 'narrowed'), route);
    const canonical = await loadConsole(source, route);
    if (derived.status !== 'ready' || canonical.status !== 'ready') throw new Error('fixtures missing');
    expect(derived.issued.effectiveAuthority?.tools).toEqual(['git.read', 'git.write']);
    expect(derived.request?.spec.requestedAccess?.tools).toContain('github.pull-request');
    expect(canonical.issued.effectiveAuthority?.tools).toContain('github.pull-request');
    expect(await loadConsole(createConsoleFixtureSource(rows, 'narrowed'), { ...route, reference: 'fix-payment-timeout-team-b' })).toEqual({ status: 'not-found' });
  });
});

describe('correlation and source boundary', () => {
  it('replaces fixture keys with unrelated opaque keys through injected resolution', async () => {
    const issued = await source.load('requests:fix-payment-timeout');
    const request = await source.load('request-document:fix-payment-timeout');
    const observed: string[] = [];
    const replacement: EvidenceSource = { async load(key) {
      observed.push(key);
      if (key === 'opaque-A') return issued;
      if (key === 'opaque-B') return request;
      throw new Error('fixture key escaped into replacement source');
    } };
    const keys = { issued: () => 'opaque-A', request: () => 'opaque-B' };
    render(<ConsolePage source={replacement} keys={keys} route={route}/>);
    expect(await screen.findByText('Allow', { exact: true })).toBeTruthy();
    expect(screen.getAllByText('Same recorded access')).toHaveLength(4);
    expect(observed).toEqual(['opaque-A', 'opaque-B']);
  });
  it.each(['name', 'template'])('withholds request comparison for mismatched %s', async kind => {
    const replacement: EvidenceSource = { async load(key) {
      const result = await source.load(key);
      if (result.status === 'request') {
        if (kind === 'name') result.data.metadata.name = 'unrelated';
        else result.data.spec.templateRef = 'unrelated';
      }
      return result;
    } };
    const result = await loadConsole(replacement, route);
    if (result.status !== 'ready') throw new Error('expected valid issued state');
    expect(result.request).toBeUndefined(); expect(result.requestGap).toContain('correlation failed');
  });
  it.each(['requestRef', 'claimReference', 'claimTemplate', 'evidenceReference'])('rejects mismatched issued %s', async kind => {
    const replacement: EvidenceSource = { async load(key) {
      const result = await source.load(key);
      if (result.status === 'issued') {
        if (kind === 'requestRef') result.data.requestRef = 'unrelated';
        if (kind === 'claimReference') result.data.claim!.requestRef = 'unrelated';
        if (kind === 'claimTemplate') result.data.claim!.templateRef = 'unrelated';
        if (kind === 'evidenceReference') result.data.evidence.requestRef = 'unrelated';
      }
      return result;
    } };
    expect((await loadConsole(replacement, route)).status).toBe('invalid');
  });
  it('keeps valid issued evidence but exposes unavailable request transport', async () => {
    const replacement: EvidenceSource = { load(key) { if (key.startsWith('request-document:')) throw new Error('private'); return source.load(key); } };
    const result = await loadConsole(replacement, route);
    expect(result.status).toBe('ready');
    if (result.status === 'ready') expect(result.requestGap).toBe('Matching request document unavailable.');
  });
  it('renders a rejected source without leaking its error detail', async () => {
    render(<ConsolePanel source={{ load: async () => { throw new Error('private-detail'); } }} route={route}/>);
    expect((await screen.findByRole('alert')).textContent).toContain('Evidence unavailable');
    expect(screen.queryByText('private-detail')).toBeNull();
  });
});

describe('single-claim page', () => {
  it.each([
    ['canonical', 'Request & principal'], ['narrowed', 'Requested vs effective authority'],
    ['loading', 'Loading evidence'], ['not-found', 'Reference not found'],
    ['malformed', 'Malformed evidence'], ['unavailable', 'Evidence unavailable'],
  ] as [Scenario, string][])('renders %s', async (scenario, heading) => {
    render(<ClaimConsole source={createConsoleFixtureSource(rows, scenario)} route={{ ...route, scenario }}/>);
    expect(await screen.findByRole('heading', { name: heading })).toBeTruthy();
    if (scenario === 'narrowed') {
      expect(screen.getByText(/Derived fixture demonstration/)).toBeTruthy();
      expect(screen.getByText('Requested, not granted: github.pull-request')).toBeTruthy();
    }
    if (scenario === 'canonical') {
      expect(screen.getAllByText('Same recorded access')).toHaveLength(4);
      expect(screen.getByText('Agent outcome not supplied.')).toBeTruthy();
      expect(screen.getByText('Revocation evidence not supplied.')).toBeTruthy();
      expect(screen.getByText('reference', { exact: true })).toBeTruthy();
      expect(screen.getByText(/Per-call allow\/deny decisions and results are not supplied/)).toBeTruthy();
    }
  });
  it('shows pre-claim denial without granting or pairing Team A data', async () => {
    render(<ClaimConsole source={source} route={{ ...route, reference: 'fix-payment-timeout-team-b' }}/>);
    expect(await screen.findByText('Deny', { exact: true })).toBeTruthy();
    expect(screen.getByText('No claim issued.')).toBeTruthy();
    expect(screen.getByText('No backend allocation in this snapshot.')).toBeTruthy();
    expect(screen.getByText('Matching request document not supplied.')).toBeTruthy();
    expect(screen.queryByText('user:team-a-engineer')).toBeNull();
    expect(screen.queryByText('Running', { exact: true })).toBeNull();
  });
  it('clears earlier evidence during loading and ignores late Allow after Deny', async () => {
    let resolve!: (value: EvidenceResult) => void;
    const delayed: EvidenceSource = { load: () => new Promise(done => { resolve = done; }) };
    const view = render(<ConsolePanel source={source} route={route}/>);
    await screen.findByText('Allow', { exact: true });
    view.rerender(<ConsolePanel source={delayed} route={{ ...route }}/>);
    expect(screen.queryByText('Allow', { exact: true })).toBeNull();
    await waitFor(() => expect(resolve).toBeTypeOf('function'));
    view.rerender(<ConsolePanel source={source} route={{ ...route, reference: 'fix-payment-timeout-team-b' }}/>);
    await screen.findByText('Deny', { exact: true });
    await act(async () => { resolve(await source.load('requests:fix-payment-timeout')); });
    expect(screen.queryByText('Allow', { exact: true })).toBeNull();
    expect(screen.getByText('Deny', { exact: true })).toBeTruthy();
  });
});
