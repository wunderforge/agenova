// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { Fact as Observation } from './contracts.generated';
import type { View } from './connected-source';

const categories: Record<string, string> = {
  RequestReceived: 'Request', RequestResolution: 'Request resolution',
  AuthorityResolved: 'Request resolution', ModelDecision: 'Model Gateway',
  ProviderAttempt: 'Model Gateway', ProviderOutcome: 'Model Gateway',
  ToolDecision: 'Tool Gateway', RunOutcome: 'Outcome',
  WorkerActivity: 'Worker',
};
const operationLabels: Record<string, string> = {
  Pending: 'Request received', Bound: 'Worker assigned',
  BackendReady: 'Environment ready', Running: 'Work started',
  Succeeded: 'Work completed', Failed: 'Work failed', Cancelled: 'Work cancelled',
  TerminateSucceeded: 'Work stopped', CleanupSucceeded: 'Environment released',
  TerminateFailed: 'Stop failed', CleanupFailed: 'Cleanup failed',
};
export const category = (fact: Observation) => fact.operation === 'tool.invoke' ? 'Tool Gateway' : categories[fact.kind] || fact.kind;
export function recordTitle(fact: Observation): string {
  if (fact.kind === 'WorkerActivity' && fact.operation === 'ActionValidated') return `${fact.target || 'Agent'} · Action format checked`;
  if (fact.kind === 'WorkerActivity') return `${fact.target || 'Agent'} · ${({TurnStarted:'Model turn started',ActionReceived:'Action received',ObservationReceived:'Tool observation received',FinalAnswer:'Final answer'} as Record<string,string>)[fact.operation || ''] || 'Recorded'}`;
  if (fact.kind === 'ProviderAttempt') return fact.operation === 'tool.invoke' ? 'Mock tool call started' : 'Model request started';
  if (fact.kind === 'ProviderOutcome') return fact.operation === 'tool.invoke' ? 'Mock tool call finished' : 'Model request finished';
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
  const reason = [...work.facts].reverse().find(f => f.kind === 'RunOutcome')?.reason || work.outcome?.failure;
  return reason && !/^Execution or cleanup failed;/.test(reason) ? reason
    : 'No specific failure reason was recorded for this work.';
};
