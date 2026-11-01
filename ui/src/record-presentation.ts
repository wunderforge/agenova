// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { Fact as Observation } from './contracts.generated';
import type { View } from './connected-source';

const categories: Record<string, string> = {
  RequestReceived: 'Request', RequestResolution: 'Request resolution',
  AuthorityResolved: 'Request resolution', ModelDecision: 'Model Gateway', ModelBudgetReservation: 'Model Gateway',
  ProviderAttempt: 'Model Gateway', ProviderOutcome: 'Model Gateway',
  ToolDecision: 'Tool Gateway', RunOutcome: 'Outcome',
  WorkerActivity: 'Worker',
};
const operationLabels: Record<string, string> = {
  Pending: 'Request received', Bound: 'Worker assigned',
  BackendReady: 'Environment ready', Running: 'Work started',
  Succeeded: 'Execution finished', Failed: 'Work failed', Cancelled: 'Work cancelled',
  TerminateSucceeded: 'Work stopped', CleanupSucceeded: 'Environment released',
  TerminateFailed: 'Stop failed', CleanupFailed: 'Cleanup failed',
  NetworkActivationAttempt: 'Network activation requested', NetworkActive: 'Network rules activated',
  NetworkActivationFailed: 'Network activation failed', NetworkRevocationAttempt: 'Network withdrawal requested',
  NetworkRevoked: 'Network rules withdrawn', NetworkRevocationFailed: 'Network withdrawal failed',
};
export const category = (fact: Observation) => fact.operation === 'tool.invoke' ? 'Tool Gateway' : categories[fact.kind] || fact.kind;
export function recordTitle(fact: Observation): string {
  if(fact.operation==='NativeHTTPSAttempt')return 'Direct HTTPS · attempt started';
  if(fact.operation==='NativeHTTPSObservation')return 'Direct HTTPS · worker observation';
  if(fact.kind==='ModelBudgetReservation')return 'Model call reserved';
  if (fact.kind === 'WorkerActivity' && fact.operation === 'ActionValidated') return fact.actionKind?`${fact.target||'Agent'} · ${fact.actionKind==='tool'?`Selected ${fact.actionTool}`:fact.actionKind==='finish'?'Finish selected':'Direct HTTPS selected'}`:`${fact.target || 'Agent'} · Action format checked`;
  if (fact.kind === 'WorkerActivity') return `${fact.target || 'Agent'} · ${({TurnStarted:'Model turn started',ActionReceived:'Action received',ObservationReceived:'Tool observation received',FinalAnswer:'Final answer'} as Record<string,string>)[fact.operation || ''] || 'Recorded'}`;
  // Only the synthetic adapter records mock-* reason codes; a configured
  // provider call must never be presented as mock.
  const tool = fact.operation === 'tool.invoke' ? (fact.reasonCode?.startsWith('mock-') ? 'Mock tool call' : 'Tool call') : '';
  if (fact.kind === 'ProviderAttempt') return tool ? `${tool} started` : 'Model request started';
  if (fact.kind === 'ProviderOutcome') return tool ? `${tool} finished` : 'Model request finished';
  if (fact.operation) return operationLabels[fact.operation] || fact.operation;
  if (fact.kind === 'RequestReceived') return 'Request received';
  if (fact.kind === 'AuthorityResolved') return 'Access resolved';
  return category(fact);
}
export const recordReason = (fact: Observation) => fact.reason || fact.decision?.reason || '';
export const recordStatus = (fact: Observation) => fact.result || fact.providerStatus || fact.decision?.result
  || (fact.operation === 'ActionValidated' && fact.reasonCode === 'agent-action-invalid' ? 'Retry required'
    : ['Runtime', 'RunOutcome'].includes(fact.kind) && /Failed$/.test(fact.operation || '') ? 'Failed'
    : fact.kind === 'RunOutcome' && ['Succeeded', 'Cancelled', 'Expired'].includes(fact.operation || '') ? fact.operation! : 'Recorded');
export const outcomeReason = (work: View) => {
  const action=terminalActionFailure(work);
  if(action?.reason)return action.reason;
  const reason = [...work.facts].reverse().find(f => f.kind === 'RunOutcome')?.reason || work.outcome?.failure;
  return reason && !/^Execution or cleanup failed;/.test(reason) ? reason
    : 'No specific failure reason was recorded for this work.';
};

// Derived from recorded host validation, never arbitrary Agent feedback or
// reconstructed model text. A later valid action/provider failure supersedes it.
export function terminalActionFailure(work:View):Observation|undefined {
 if(work.outcome?.status!=='Failed')return;
 const code=work.diagnosis?.code||[...work.facts].reverse().find(f=>f.kind==='RunOutcome')?.reasonCode;
 if(!['agent-no-final-result','agent-invalid-action','agent-invalid-action-limit'].includes(code||''))return;
 for(const f of [...work.facts].reverse()){
  if(f.kind==='ProviderOutcome'&&f.providerStatus==='Failed'||f.result==='Deny'||f.decision?.result==='Deny')return;
  if(f.kind==='WorkerActivity'&&['ObservationReceived','FinalAnswer'].includes(f.operation||''))return;
  if(f.kind==='WorkerActivity'&&f.operation==='ActionValidated')return f.reasonCode==='agent-action-invalid'?f:undefined;
 }
}
