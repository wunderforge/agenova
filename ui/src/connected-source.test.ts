// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it, vi } from 'vitest';
import { connectedSource, validPolicyRule, suggestedProjects, SourceError, type Setup } from './connected-source';

afterEach(() => vi.unstubAllGlobals());

it('accepts Go nil-slice caps as explicit empty sets, not unrestricted access',()=>{
 const rule={team:'team-a',project:'payments',action:'claim.create',templateRef:'engineer',constraints:{tools:{values:null}}};
 expect(validPolicyRule(rule)).toBe(true);
 expect(validPolicyRule({...rule,constraints:{tools:{values:'all'}}})).toBe(false);
});

it('never uses inspection projects to grant a viewer submission access',()=>{
 expect(suggestedProjects({submissionProjects:[],projects:['payments']} as unknown as Setup)).toEqual([]);
});
it.each([400,401,403,422,429])('uses a safe actionable message for definite HTTP %s rejection',async(status)=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response('private upstream stack and credentials',{status})));
 try{await connectedSource.submit({} as Parameters<typeof connectedSource.submit>[0]);throw new Error('expected rejection');}catch(error){expect(error).toBeInstanceOf(SourceError);expect((error as SourceError).uncertain).toBe(false);expect((error as Error).message).not.toContain('private');}
});
it.each([409,500,503])('retains uncertainty on HTTP %s so a retry does not generate new intent',async(status)=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response('{}',{status})));
 await expect(connectedSource.submit({} as Parameters<typeof connectedSource.submit>[0])).rejects.toMatchObject({uncertain:true});
});
it('an incomplete success response remains an uncertain submission',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response('{}',{status:202})));
 await expect(connectedSource.submit({} as Parameters<typeof connectedSource.submit>[0])).rejects.toMatchObject({uncertain:true});
});
it('automatic submission preserves uncertainty for generic failure and hides unknown server details',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({code:'advisor_unavailable',message:'The result is not confirmed.'}),{status:503})));
 await expect(connectedSource.autoSubmit({} as Parameters<typeof connectedSource.autoSubmit>[0])).rejects.toMatchObject({uncertain:true});
 vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({code:'raw_provider_error',message:'private upstream credential'}),{status:503})));
 try{await connectedSource.autoSubmit({} as Parameters<typeof connectedSource.autoSubmit>[0]);throw new Error('expected failure');}catch(error){expect((error as SourceError).uncertain).toBe(true);expect((error as Error).message).not.toContain('private');}
});
it('a definite preflight model failure does not pretend a Work may have started',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({code:'advisor_failed',message:'Advisor did not return a valid result; no task was started.'}),{status:503})));
 await expect(connectedSource.autoSubmit({} as Parameters<typeof connectedSource.autoSubmit>[0])).rejects.toMatchObject({uncertain:false});
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
