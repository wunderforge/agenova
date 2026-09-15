// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';
import type { Fact } from './contracts.generated';
import type { WorkEvent } from './portal-data';

export interface WorkerAction {
  id: string; label: string; target: string; href: string;
  state: string; active: boolean; turn?: string;
}
// Correlate calls independently. Allow/Deny is not evidence of tool execution.
export function workerActions(facts: Fact[], status: string, activityHref: string): WorkerAction[] {
  const ordered = [...facts].sort((a, b) => a.sequence - b.sequence);
  let turn: string | undefined;
  return ordered.filter(f => ['WorkerActivity', 'ProviderAttempt', 'ProviderOutcome', 'ModelDecision', 'ToolDecision'].includes(f.kind))
    .filter(f => f.kind !== 'ProviderOutcome' || !ordered.some(a => a.kind === 'ProviderAttempt' && !!f.invocationId && a.invocationId === f.invocationId && a.sequence < f.sequence))
    .map(f => {
      if (f.kind === 'WorkerActivity' && f.target) turn = f.target;
      const outcome = f.kind === 'ProviderAttempt' && f.invocationId
        ? [...ordered].reverse().find(o => o.kind === 'ProviderOutcome' && o.invocationId === f.invocationId && o.sequence > f.sequence) : undefined;
      const active = f.kind === 'ProviderAttempt' && !!f.invocationId && !outcome && status === 'Running';
      const decision = f.kind === 'ModelDecision' || f.kind === 'ToolDecision';
      const toolCall = f.operation === 'tool.invoke';
      const step = f.kind === 'WorkerActivity';
      return { id: f.id, turn, href: `${activityHref}/${encodeURIComponent(outcome?.id || f.id)}`,
        label: step ? 'Agent turn' : decision ? f.kind === 'ToolDecision' ? 'Tool access' : 'Model access' : toolCall ? 'Tool call (mock)' : 'Model request',
        target: f.target || f.invocationId || 'Target not recorded', active,
        state: step ? ({TurnStarted:'Started',ActionReceived:'Action received',ObservationReceived:'Observation received',FinalAnswer:'Final answer'} as Record<string,string>)[f.operation || ''] || 'Recorded' : active ? 'Waiting for response' : decision ? f.result || 'Recorded'
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
export function WorkerActivity({ actions, status, activityHref }: { actions: WorkerAction[]; status: string; activityHref: string }) {
  const [chosen, setChosen] = useState<string>();
  const [followLatest, setFollowLatest] = useState(true);
  const pending = actions.filter(a => a.active);
  const groups = groupWorkerTurns(actions);
  const latest = groups.at(-1)?.id;
  const turn = [...actions].reverse().find(a => a.label === 'Agent turn')?.target;
  const turns = actions.filter(a => a.label === 'Agent turn' && a.state === 'Started').length;
  const observations = actions.filter(a => a.label === 'Agent turn' && a.state === 'Observation received').length;
  const toolsWaiting = pending.some(a => a.label === 'Tool call (mock)');
  if (!actions.length && !['Running', 'Failed', 'Finishing'].includes(status)) return null;
  const summary = pending.length ? toolsWaiting ? 'Waiting for tool observation' : `Waiting for ${pending.length === 1 ? 'a model response' : `${pending.length} model responses`}`
    : status === 'Running' ? 'Worker running · No active call recorded'
    : status === 'Failed' ? 'Work failed · Inspect the recorded failure'
    : 'No active calls';
  return <section className={`portal-worker ${pending.length ? 'has-active-call' : ''}`} aria-label="Worker activity">
    <div className="portal-section-head"><div><h2>Agent activity</h2><p className="portal-activity-help">{turn ? 'Model calls and tool results, grouped by turn.' : actions.some(a => ['Model request', 'Tool call (mock)'].includes(a.label)) ? 'Recorded model and tool calls.' : 'Access checks only; no execution calls recorded.'}</p></div><a href={activityHref}>View records</a></div>
    <p className="portal-worker-current" role="status"><span className="portal-worker-lamp" aria-hidden="true"/><span>{turn && `${turn} · `}{summary}</span></p>
    {turns > 0 && <p className="portal-turn-count">{turns} model turns · {observations} tool observations</p>}
    <div className="portal-turns">{groups.map(group => {
      const open = followLatest ? group.id === latest : chosen === group.id;
      const failures = group.calls.filter(a => /failed/i.test(a.state)).length;
      const blocked = group.calls.filter(a => /deny|denied/i.test(a.state)).length;
      const noun = group.calls.every(a => ['Tool access', 'Model access'].includes(a.label)) && group.calls.length ? 'check' : 'call';
      return <details className="portal-turn" key={group.id} open={open} data-active={group.active} data-negative={failures > 0 || blocked > 0}>
        <summary onClick={event => {
          event.preventDefault();
          setFollowLatest(!open && group.id === latest);
          setChosen(open ? undefined : group.id);
        }}>
          <strong>{group.id}</strong><span>{group.active ? 'In progress' : `${group.calls.length} ${noun}${group.calls.length === 1 ? '' : 's'}`}{group.observations > 0 && ` · ${group.observations} ${group.observations === 1 ? 'observation' : 'observations'}`}{failures > 0 && <span className="portal-turn-failed"> · {failures} failed</span>}{blocked > 0 && <span className="portal-turn-failed"> · {blocked} blocked</span>}</span>
        </summary>
        <ul className="portal-worker-actions">{group.calls.map(a => <li key={a.id} data-active={a.active} data-kind={a.label === 'Tool call (mock)' ? 'tool' : a.label === 'Model request' ? 'model' : 'access'} data-negative={/deny|denied|failed|cancelled/i.test(a.state)}>
          <div><a href={a.href}>{a.label}</a><small>{a.target}</small></div>
          <span className={`portal-worker-state ${/deny|denied|failed|cancelled/i.test(a.state) ? 'negative' : /^(succeeded|allowed|allow)$/i.test(a.state) ? 'positive' : ''}`}>{a.state}</span>
        </li>)}</ul>
        {!group.calls.length && <p className="portal-source-note">No execution call recorded for this turn yet.</p>}
      </details>;
    })}</div>
  </section>;
}
