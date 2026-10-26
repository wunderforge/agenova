// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {describe,it,expect} from 'vitest';
import type {View} from './connected-source';
import {workPermissions} from './work-permissions';
function view():View{
 const request=JSON.parse(readFileSync(resolve(process.cwd(),'../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json'),'utf8'));
 const state=JSON.parse(readFileSync(resolve(process.cwd(),'../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json'),'utf8'));
 request.spec.requestedAccess={tools:['web.read'],resourceScopes:['web:public-pages'],modelProfile:'approved-coding-model'};
 state.effectiveAuthority.tools=['web.read','crm.read'];state.effectiveAuthority.resourceScopes=['web:public-pages','crm:customer-retention'];
 return {version:'agenova.evidence/v0',requestRef:request.metadata.name,request,state,facts:[]};
}
describe('historical requested-only permission comparison',()=>{
 it('lists only actual requests, not unused template/effective capabilities',()=>{
  const rows=workPermissions(view());expect(rows.map(r=>r.label)).toEqual(['Tools','Resources','Model','Runtime','Time limit']);expect(JSON.stringify(rows)).not.toMatch(/crm.read|Memory|Network/);
 });
 it('marks only requested but ungranted values as not granted',()=>{
  const w=view();w.state!.effectiveAuthority!.tools=[];const rows=workPermissions(w);expect(rows[0]).toMatchObject({requested:'web.read',granted:'Not granted',state:'not-granted'});expect(rows[1].state).toBe('granted');
 });
 it('does not convert failed execution into a permission denial',()=>{
  const w=view();w.outcome={status:'Failed',failure:'provider unavailable'};expect(workPermissions(w)[0].state).toBe('granted');
 });
 it('distinguishes pending, unknown and denied issuance',()=>{
  const w=view();delete w.state;expect(workPermissions(w).every(r=>r.state==='pending'&&r.granted==='Pending')).toBe(true);
  const unknown=view();delete unknown.state!.effectiveAuthority;expect(workPermissions(unknown).every(r=>r.state==='unknown')).toBe(true);
  unknown.state!.decision.result='Deny';expect(workPermissions(unknown).every(r=>r.state==='not-granted')).toBe(true);
 });
 it('includes each requested network/memory value and exact historical runtime',()=>{
  const w=view();w.request.spec.requestedAccess!.networkProfiles=['public-web'];w.request.spec.requestedAccess!.memoryScopes=['team-docs'];
  expect(workPermissions(w).filter(r=>['Network','Memory'].includes(r.label))).toHaveLength(2);
  w.request.spec.runtime!.timeout='40m';w.state!.effectiveAuthority!.runtime.timeout='30m';expect(workPermissions(w).find(r=>r.id==='timeout')).toMatchObject({requested:'40m',granted:'30m',state:'granted'});
 });
});
