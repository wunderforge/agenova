// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import type { Fact } from './contracts.generated';
import { category, recordStatus, recordTitle } from './record-presentation';
const fact: Fact = {id:'action',sequence:1,timestamp:'2026-09-16T02:00:00Z',kind:'WorkerActivity',requestRef:'work-1',target:'Turn 4',operation:'ActionValidated',reasonCode:'agent-action-invalid'};
describe('evidence presentation independent of React and transport',()=>{
  it('distinguishes successful inference, format retry and failed work',()=>{
    expect(recordStatus({...fact,kind:'ProviderOutcome',providerStatus:'Succeeded',operation:'model.invoke'})).toBe('Succeeded');
    expect(recordStatus(fact)).toBe('Retry required');
    expect(recordStatus({...fact,kind:'RunOutcome',operation:'Failed'})).toBe('Failed');
    expect(recordTitle(fact)).toBe('Turn 4 · Action format checked');
  });
  it('uses recorded operation to distinguish tool from model provider',()=>{
    expect(category({...fact,kind:'ProviderOutcome',operation:'tool.invoke'})).toBe('Tool Gateway');
    expect(category({...fact,kind:'ProviderOutcome',operation:'model.invoke'})).toBe('Model Gateway');
  });
  it('labels only the synthetic adapter as mock',()=>{
    expect(recordTitle({...fact,kind:'ProviderAttempt',operation:'tool.invoke',reasonCode:'mock-tool'})).toBe('Mock tool call started');
    expect(recordTitle({...fact,kind:'ProviderOutcome',operation:'tool.invoke',reasonCode:'mock-artifact-not-found'})).toBe('Mock tool call finished');
    expect(recordTitle({...fact,kind:'ProviderAttempt',operation:'tool.invoke',reasonCode:'configured-tool'})).toBe('Tool call started');
    expect(recordTitle({...fact,kind:'ProviderOutcome',operation:'tool.invoke',reasonCode:'tool-timeout'})).toBe('Tool call finished');
    expect(recordTitle({...fact,kind:'ProviderOutcome',operation:'tool.invoke',reasonCode:undefined})).toBe('Tool call finished');
  });
});
