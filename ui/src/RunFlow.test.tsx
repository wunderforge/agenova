// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import { flowStages, type FlowEvidence } from './RunFlow';
const base: FlowEvidence = { status: 'Pending', received: true, authorized: false,
  started: false, modelRequested: false, modelFinished: false, result: false, cleanup: false };
describe('evidence-driven flow presentation', () => {
  it('pending and denial never invent workers or models', () => {
    const pending = flowStages(base);
    expect(pending.map(s => s.state)).toEqual(['done','active','waiting','waiting','waiting','waiting']);
    const denied = flowStages({...base, status:'Denied'});
    expect(denied[1]).toMatchObject({state:'blocked',detail:'Denied'});
    expect(denied.slice(2).every(s => s.detail === 'Not reached')).toBe(true);
    expect(denied.some(s => s.state === 'active')).toBe(false);
  });
  it('ready/starting is not a recorded worker start', () => {
    const stage = flowStages({...base,status:'Starting',authorized:true})[2];
    expect(stage).toMatchObject({state:'active',detail:'Starting'});
  });
  it('permission alone is not a provider response', () => {
    const stage = flowStages({...base,status:'Running',authorized:true,started:true})[3];
    expect(stage.state).toBe('waiting');
    expect(flowStages({...base,status:'Running',authorized:true,started:true,modelRequested:true})[3].state).toBe('active');
  });
  it('success does not imply optional model use or confirmed cleanup', () => {
    const stages = flowStages({...base,status:'Succeeded',authorized:true,started:true,result:true});
    expect(stages[3]).toMatchObject({state:'waiting',detail:'No record'});
    expect(stages[4].state).toBe('done');
    expect(stages[5]).toMatchObject({state:'waiting',detail:'No record'});
    expect(stages.some(s => s.state === 'active')).toBe(false);
  });
  it('a finished unsuccessful provider attempt does not stay in inference', () => {
    const stages = flowStages({...base,status:'Running',authorized:true,started:true,
      modelRequested:true,modelActive:false});
    expect(stages[3]).toMatchObject({state:'waiting',detail:'Request recorded'});
    expect(stages[2].state).toBe('active');
  });
  it('provider completion and cleanup confirmation complete their own stages', () => {
    const stages = flowStages({...base,status:'Succeeded',authorized:true,started:true,
      modelRequested:true,modelFinished:true,result:true,cleanup:true});
    expect(stages.every(s => s.state === 'done')).toBe(true);
  });
  it('cleanup failure never rewrites a successful task outcome', () => {
    const stages = flowStages({...base,status:'Succeeded',authorized:true,started:true,
      result:true,failureAt:'Cleanup'});
    expect(stages[4].state).toBe('done');
    expect(stages[5]).toMatchObject({state:'blocked',detail:'Needs attention'});
  });
  it('failure may still have independent successful cleanup', () => {
    const stages = flowStages({...base,status:'Failed',authorized:true,started:true,
      modelRequested:true,failureAt:'Model',cleanup:true});
    expect(stages[3].state).toBe('blocked');
    expect(stages[4].state).toBe('waiting');
    expect(stages[5].state).toBe('done');
  });
  it('queued cancellation does not suggest a worker ran', () => {
    const stages = flowStages({...base,status:'Cancelled',failureAt:'Request'});
    expect(stages[0]).toMatchObject({state:'blocked',detail:'Cancelled'});
    expect(stages[2].state).toBe('waiting');
  });
});
