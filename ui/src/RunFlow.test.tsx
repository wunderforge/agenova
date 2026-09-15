// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import { flowStages, type FlowEvidence } from './RunFlow';
const base: FlowEvidence = { status:'Pending',received:true,authorized:false,started:false,result:false,cleanup:false };
describe('evidence-driven static lifecycle', () => {
  it('keeps internal model activity out of lifecycle', () => {
    expect(flowStages(base).map(s => s.label)).toEqual(['Request','Access','Worker','Result','Cleanup']);
    expect(flowStages(base).map(s => s.state)).toEqual(['done','active','waiting','waiting','waiting']);
  });
  it('denial does not invent execution', () => {
    const stages=flowStages({...base,status:'Denied'});
    expect(stages[1]).toMatchObject({state:'blocked',detail:'Denied'});
    expect(stages.slice(2).every(s=>s.detail==='Not reached')).toBe(true);
    expect(stages.some(s=>s.state==='active')).toBe(false);
  });
  it('starting is not a recorded worker start', () => {
    expect(flowStages({...base,status:'Starting',authorized:true})[2]).toMatchObject({state:'active',detail:'Starting'});
  });
  it('running only identifies the worker lifecycle stage', () => {
    expect(flowStages({...base,status:'Running',authorized:true,started:true})[2]).toMatchObject({state:'active',detail:'Running'});
  });
  it('success does not imply cleanup confirmation', () => {
    const stages=flowStages({...base,status:'Succeeded',authorized:true,started:true,result:true});
    expect(stages[3].state).toBe('done');
    expect(stages[4]).toMatchObject({state:'waiting',detail:'No record'});
    expect(stages.some(s=>s.state==='active')).toBe(false);
  });
  it('actual result and cleanup complete independent stages', () => {
    expect(flowStages({...base,status:'Succeeded',authorized:true,started:true,result:true,cleanup:true}).every(s=>s.state==='done')).toBe(true);
  });
  it('cleanup failure does not change successful task outcome', () => {
    const stages=flowStages({...base,status:'Succeeded',result:true,failureAt:'Cleanup'});
    expect(stages[3].state).toBe('done');
    expect(stages[4]).toMatchObject({state:'blocked',detail:'Needs attention'});
  });
  it('model failure belongs to worker execution; cleanup can still succeed', () => {
    const stages=flowStages({...base,status:'Failed',started:true,failureAt:'Model',cleanup:true});
    expect(stages[2].state).toBe('blocked');
    expect(stages[3].state).toBe('waiting');
    expect(stages[4].state).toBe('done');
  });
  it('queued cancellation does not suggest a worker ran', () => {
    const stages=flowStages({...base,status:'Cancelled',failureAt:'Request'});
    expect(stages[0]).toMatchObject({state:'blocked',detail:'Cancelled'});
    expect(stages[2].state).toBe('waiting');
  });
});
