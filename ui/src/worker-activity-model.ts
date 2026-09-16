// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { Fact } from './contracts.generated';
import type { WorkEvent } from './portal-data';

export interface WorkerAction {
  id: string; label: string; target: string; href: string;
  state: string; active: boolean; turn?: string;
}
// Correlate calls independently. Allow/Deny is not evidence of tool execution.
export function workerActions(facts: Fact[], status: string, activityHref: string): WorkerAction[] {
  const ordered = [...facts].sort((a, b) => a.sequence - b.sequence);
  const firstAttempt = new Map<string, number>();
  const outcomes = new Map<string, Fact>();
  for (const f of ordered) {
    if (!f.invocationId) continue;
    if (f.kind === 'ProviderAttempt' && !firstAttempt.has(f.invocationId)) firstAttempt.set(f.invocationId, f.sequence);
    if (f.kind === 'ProviderOutcome') outcomes.set(f.invocationId, f);
  }
  let turn: string | undefined;
  return ordered.filter(f => ['WorkerActivity', 'ProviderAttempt', 'ProviderOutcome', 'ModelDecision', 'ToolDecision'].includes(f.kind))
    .filter(f => f.kind !== 'ProviderOutcome' || !f.invocationId || (firstAttempt.get(f.invocationId) ?? Infinity) >= f.sequence)
    .map(f => {
      if (f.kind === 'WorkerActivity' && f.target) turn = f.target;
      const candidate = f.kind === 'ProviderAttempt' && f.invocationId ? outcomes.get(f.invocationId) : undefined;
      const outcome = candidate && candidate.sequence > f.sequence ? candidate : undefined;
      const active = f.kind === 'ProviderAttempt' && !!f.invocationId && !outcome && status === 'Running';
      const decision = f.kind === 'ModelDecision' || f.kind === 'ToolDecision';
      const toolCall = f.operation === 'tool.invoke';
      const step = f.kind === 'WorkerActivity';
      return { id: f.id, turn, href: `${activityHref}/${encodeURIComponent(outcome?.id || f.id)}`,
        label: step && f.operation === 'ActionValidated' && f.reasonCode === 'agent-action-invalid' ? 'Action check' : step ? 'Agent turn' : decision ? f.kind === 'ToolDecision' ? 'Tool access' : 'Model access' : toolCall ? 'Tool call (mock)' : 'Model request',
        target: f.operation === 'ActionValidated' ? f.reason || 'Action format checked' : f.target || f.invocationId || 'Target not recorded', active,
        state: step ? f.operation === 'ActionValidated' && f.reasonCode === 'agent-action-invalid' ? 'Retry required' : ({TurnStarted:'Started',ActionReceived:'Action received',ObservationReceived:'Observation received',FinalAnswer:'Final answer'} as Record<string,string>)[f.operation || ''] || 'Recorded' : active ? 'Waiting for response' : decision ? f.result || 'Recorded'
          : outcome?.providerStatus || (f.kind === 'ProviderOutcome' ? f.providerStatus : undefined) || 'No completion recorded',
      };
    });
}
export function demoWorkerActions(events: WorkEvent[], activityHref: string): WorkerAction[] {
  return events.filter(e => e.kind === 'Model' || e.kind === 'Tool').map(e => ({
    id: e.id, label: e.kind === 'Tool' ? 'Tool access' : 'Model access', target: e.target,
    href: `${activityHref}/${e.id}`, state: e.result, active: false,
  }));
}
export interface WorkerTurn { id: string; calls: WorkerAction[]; observations: number; active: boolean }
export function groupWorkerTurns(actions: WorkerAction[]): WorkerTurn[] {
  const recordedTurns = actions.some(a => a.turn);
  const recordedCalls = actions.some(a => ['Model request', 'Tool call (mock)'].includes(a.label));
  const groups = new Map<string, WorkerTurn>();
  for (const action of actions) {
    // Access decisions remain inspectable records, not redundant execution rows.
    if ((recordedTurns || recordedCalls) && ['Tool access', 'Model access'].includes(action.label)) continue;
    const id = action.turn || (recordedTurns ? 'Earlier calls' : recordedCalls ? 'Recorded calls' : 'Recorded access checks');
    const group = groups.get(id) || {id, calls: [], observations: 0, active: false};
    if (action.label !== 'Agent turn') group.calls.push(action);
    if (action.state === 'Observation received') group.observations++;
    group.active ||= action.active;
    groups.set(id, group);
  }
  return [...groups.values()];
}
