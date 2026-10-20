// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, it, vi } from 'vitest';
import { connectedSource, traceURL, workStatus, isTerminal, type View } from './connected-source';
it('keeps interrupted authority inactive even when its last persisted phase was Running', () => {
 const work = { recovery: { code:'service-restarted', cleanup:'Unknown' }, state:{claim:{phase:'Running'}} } as unknown as View;
 expect(workStatus(work)).toBe('Interrupted'); expect(isTerminal(work)).toBe(true);
});
it('accepts only validated trace IDs and loopback Grafana destinations', () => {
 expect(traceURL('http://malicious')).toBeUndefined(); expect(traceURL('abc')).toBeUndefined();
 const target = traceURL('0123456789abcdef0123456789abcdef');
 expect(new URL(target!).hostname).toBe('127.0.0.1'); expect(target).toContain('tempo');
 expect(JSON.parse(new URL(target!).searchParams.get('panes')!).work.queries[0].datasource.uid).toBe('tempo');
});
it('queries bounded history and rejects malformed envelopes', async () => {
 const fetch = vi.fn(async (_input: RequestInfo | URL) => new Response(JSON.stringify({items:[],nextCursor:'opaque'})));
 vi.stubGlobal('fetch',fetch);
 expect((await connectedSource.history('page:2')).nextCursor).toBe('opaque');
 expect(fetch.mock.calls[0]?.[0]).toBe('/api/history?limit=20&cursor=page%3A2');
 vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({items:[{}]}))));
 await expect(connectedSource.history()).rejects.toThrow('incomplete history');
 vi.unstubAllGlobals();
});
