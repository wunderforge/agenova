// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import type { Fact } from './contracts.generated';
import { workerActions, demoWorkerActions, groupWorkerTurns } from './worker-activity-model';
const fact=(id:string,sequence:number,kind:Fact['kind'],invocationId?:string,providerStatus?:string):Fact=>({id,sequence,kind,invocationId,providerStatus,requestRef:'work-1',timestamp:'2026-09-15T02:00:00Z'});
describe('recorded worker calls',()=>{
  it('retains format-retry evidence without marking successful inference as failed',()=>{
    const facts:Fact[]=[
      {...fact('turn',1,'WorkerActivity'),operation:'TurnStarted',target:'Turn 4'},
      fact('attempt',2,'ProviderAttempt','m'),
      fact('outcome',3,'ProviderOutcome','m','Succeeded'),
      {...fact('invalid',4,'WorkerActivity'),operation:'ActionValidated',target:'Turn 4',reasonCode:'agent-action-invalid',reason:'Final-answer tool/input must be empty.'},
    ];
    const group=groupWorkerTurns(workerActions(facts,'Failed','#/activity'))[0];
    expect(group.calls.map(a=>a.state)).toEqual(['Succeeded','Retry required']);
    expect(group.calls[1]).toMatchObject({active:false,target:'Final-answer tool/input must be empty.',href:'#/activity/invalid'});
  });
  it('groups actual turns, retains failed observations and excludes permission checks from execution',()=>{
    const facts:Fact[]=[
      {...fact('turn1',1,'WorkerActivity'),operation:'TurnStarted',target:'Turn 1'},
      {...fact('decision',2,'ToolDecision','t1'),result:'Allow'},
      {...fact('attempt',3,'ProviderAttempt','t1'),operation:'tool.invoke'},
      {...fact('failed',4,'ProviderOutcome','t1','Failed'),operation:'tool.invoke'},
      {...fact('observation',5,'WorkerActivity'),operation:'ObservationReceived',target:'Turn 1'},
      {...fact('turn2',6,'WorkerActivity'),operation:'TurnStarted',target:'Turn 2'},
      fact('retry',7,'ProviderAttempt','m2'),
    ];
    const groups=groupWorkerTurns(workerActions(facts,'Running','#/activity'));
    expect(groups.map(g=>[g.id,g.calls.length,g.observations,g.active])).toEqual([['Turn 1',1,1,false],['Turn 2',1,0,true]]);
    expect(groups[0].calls[0]).toMatchObject({state:'Failed',href:'#/activity/failed'});
  });
  it('does not invent turns or call permission decisions execution',()=>{
    expect(groupWorkerTurns(workerActions([fact('a',1,'ProviderAttempt','m')],'Running','#/activity'))[0].id).toBe('Recorded calls');
    expect(groupWorkerTurns(workerActions([fact('d',1,'ToolDecision')],'Running','#/activity'))[0].id).toBe('Recorded access checks');
    const mixed=groupWorkerTurns(workerActions([fact('d',1,'ModelDecision'),fact('a',2,'ProviderAttempt','m')],'Running','#/activity'));
    expect(mixed[0].calls.map(a=>a.label)).toEqual(['Model request']);
  });
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
