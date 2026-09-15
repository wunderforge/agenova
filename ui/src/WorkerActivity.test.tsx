// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import type { Fact } from './contracts.generated';
import { workerActions, demoWorkerActions } from './WorkerActivity';
const fact=(id:string,sequence:number,kind:Fact['kind'],invocationId?:string,providerStatus?:string):Fact=>({id,sequence,kind,invocationId,providerStatus,requestRef:'work-1',timestamp:'2026-09-15T02:00:00Z'});
describe('recorded worker calls',()=>{
  it('shows actual loop turns and mock tool completion without treating it as model inference',()=>{
    const facts:Fact[]=[{...fact('turn',1,'WorkerActivity'),operation:'TurnStarted',target:'Turn 2'}, {...fact('d',2,'ToolDecision','tool-1'),result:'Allow'}, {...fact('a',3,'ProviderAttempt','tool-1'),operation:'tool.invoke'}, {...fact('o',4,'ProviderOutcome','tool-1','Succeeded'),operation:'tool.invoke'}];
    const actions=workerActions(facts,'Running','#/activity');
    expect(actions[0]).toMatchObject({label:'Agent turn',target:'Turn 2',state:'Started'});
    expect(actions[2]).toMatchObject({label:'Tool call (mock)',active:false,state:'Succeeded'});
  });
  it('only an unresolved correlated attempt is active',()=>{
    const actions=workerActions([fact('a',1,'ProviderAttempt','call-1','Attempted')],'Running','#/work/1/activity');
    expect(actions[0]).toMatchObject({active:true,state:'Waiting for response',label:'Model request'});
  });
  it('matches a response to its own invocation, not the latest event',()=>{
    const facts=[fact('b',2,'ProviderAttempt','call-2'),fact('a',1,'ProviderAttempt','call-1'),fact('o',3,'ProviderOutcome','call-1','Succeeded')];
    expect(workerActions(facts,'Running','#/activity').map(a=>[a.id,a.active,a.state])).toEqual([['a',false,'Succeeded'],['b',true,'Waiting for response']]);
  });
  it.each(['Succeeded','Failed','Cancelled'])('stops motion on %s response',result=>{
    expect(workerActions([fact('a',1,'ProviderAttempt','call-1'),fact('o',2,'ProviderOutcome','call-1',result)],'Running','#/activity')[0]).toMatchObject({active:false,state:result});
  });
  it.each(['Succeeded','Failed','Expired','Cancelled','Finishing','Pending','Denied'])('terminal/non-running %s never pulses an unfinished call',status=>{
    expect(workerActions([fact('a',1,'ProviderAttempt','call-1')],status,'#/activity')[0]).toMatchObject({active:false,state:'No completion recorded'});
  });
  it('missing correlation stays a static record',()=>{
    expect(workerActions([fact('a',1,'ProviderAttempt')],'Running','#/activity')[0].active).toBe(false);
  });
  it('tool and model permission are not executing calls',()=>{
    const facts:Fact[]=[{...fact('t',1,'ToolDecision'),result:'Allow'},{...fact('m',2,'ModelDecision'),result:'Deny'}];
    expect(workerActions(facts,'Running','#/activity').map(a=>[a.label,a.state,a.active])).toEqual([['Tool access','Allow',false],['Model access','Deny',false]]);
  });
  it('an orphan response is retained but never animated',()=>{
    expect(workerActions([fact('o',2,'ProviderOutcome','call-1','Failed')],'Running','#/activity')[0]).toMatchObject({active:false,state:'Failed'});
  });
  it('demo access records do not fabricate provider execution',()=>{
    const actions=demoWorkerActions([{id:'m',kind:'Model',title:'Model request',time:'12:00',result:'Allowed',source:'Model Gateway',target:'coding-standard',description:'Approved'}],'#/activity');
    expect(actions[0]).toMatchObject({label:'Model access',active:false});
  });
});
