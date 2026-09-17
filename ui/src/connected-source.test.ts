// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it, vi } from 'vitest';
import { connectedSource, suggestedProjects, type Setup } from './connected-source';

afterEach(() => vi.unstubAllGlobals());

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
