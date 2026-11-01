// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import type {Fact} from './contracts.generated';
import type {View} from './connected-source';
import {WorkProgress} from './WorkProgress';
import {WorkerActivity} from './WorkerActivity';
import {workerActions} from './worker-activity-model';

afterEach(()=>{cleanup();vi.restoreAllMocks()});
const base={requestRef:'work',request:{metadata:{name:'work'},spec:{}},facts:[]} as unknown as View;
it('shows only recorded progress and quota without fabricating dispatch or completed goal',()=>{
 const view={...base,facts:[{id:'request',sequence:1,kind:'RequestReceived',requestRef:'work',timestamp:'2026-11-01T00:00:00Z'},{id:'budget',sequence:2,kind:'ModelBudgetReservation',modelCallCount:1,modelCallLimit:2,requestRef:'work',timestamp:'2026-11-01T00:00:01Z'}],state:{decision:{result:'Allow'},claim:{phase:'Running'},effectiveAuthority:{runtime:{maxModelCalls:2}}}} as unknown as View;
 const {container}=render(<WorkProgress work={view}/>);
 expect(screen.getByText('Model calls · 1 / 2')).toBeTruthy();expect(container.querySelectorAll('[data-stage-state=done]')).toHaveLength(2);
 expect(screen.getByRole('button',{name:/Result/}).hasAttribute('disabled')).toBe(true);expect(container.textContent).not.toContain('%');
});
it('keeps an actual Gateway refusal visible beside its model action and opens it from navigation',()=>{
 const facts:Fact[]=[{id:'turn',sequence:1,kind:'WorkerActivity',operation:'TurnStarted',target:'Turn 1',requestRef:'work',timestamp:'2026-11-01T00:00:00Z'},
 {id:'action',sequence:2,kind:'WorkerActivity',operation:'ActionValidated',actionKind:'tool',actionTool:'web.read',actionResource:'web:public-pages',turn:1,requestRef:'work',timestamp:'2026-11-01T00:00:01Z'},
 {id:'deny',sequence:3,kind:'ToolDecision',operation:'tool.invoke',result:'Deny',target:'web.read',actionRef:'action',turn:1,reason:'Tool is outside this Work authority.',requestRef:'work',timestamp:'2026-11-01T00:00:02Z'}];
 render(<WorkerActivity actions={workerActions(facts,'Succeeded','#/activity')} status="Succeeded" activityHref="#/activity"/>);
 expect(screen.getByText('tool · web.read · web:public-pages')).toBeTruthy();expect(screen.getAllByText('Tool access').length).toBeGreaterThan(0);expect(screen.getAllByText('Deny').length).toBeGreaterThan(0);
 fireEvent.click(screen.getByRole('button',{name:'Inspect Tool access: web.read'}));expect(screen.getByLabelText('Selected activity').textContent).toContain('Tool is outside this Work authority.');
 fireEvent(window,new CustomEvent('agenova:inspect-fact',{detail:'action'}));expect(screen.getByLabelText('Selected activity').textContent).toContain('Agent action');
 expect(screen.queryByText('Tool call')).toBeNull();
});
