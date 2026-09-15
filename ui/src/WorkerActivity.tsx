// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { Fact } from './contracts.generated';
import type { WorkEvent } from './portal-data';

export interface WorkerAction {
  id: string; label: string; target: string; href: string;
  state: string; active: boolean;
}
// Correlate calls independently. Allow/Deny is not evidence of tool execution.
export function workerActions(facts: Fact[], status: string, activityHref: string): WorkerAction[] {
  const ordered = [...facts].sort((a, b) => a.sequence - b.sequence);
  return ordered.filter(f => ['ProviderAttempt', 'ProviderOutcome', 'ModelDecision', 'ToolDecision'].includes(f.kind))
    .filter(f => f.kind !== 'ProviderOutcome' || !ordered.some(a => a.kind === 'ProviderAttempt' && !!f.invocationId && a.invocationId === f.invocationId && a.sequence < f.sequence))
    .map(f => {
      const outcome = f.kind === 'ProviderAttempt' && f.invocationId
        ? [...ordered].reverse().find(o => o.kind === 'ProviderOutcome' && o.invocationId === f.invocationId && o.sequence > f.sequence) : undefined;
      const active = f.kind === 'ProviderAttempt' && !!f.invocationId && !outcome && status === 'Running';
      const decision = f.kind === 'ModelDecision' || f.kind === 'ToolDecision';
      return { id: f.id, href: `${activityHref}/${encodeURIComponent(outcome?.id || f.id)}`,
        label: decision ? f.kind === 'ToolDecision' ? 'Tool access' : 'Model access' : 'Model request',
        target: f.target || f.invocationId || 'Target not recorded', active,
        state: active ? 'Waiting for response' : decision ? f.result || 'Recorded'
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
export function WorkerActivity({ actions, status, activityHref }: { actions: WorkerAction[]; status: string; activityHref: string }) {
  const pending = actions.filter(a => a.active);
  const visible = [...pending, ...actions.filter(a => !a.active).slice(-4).reverse()];
  if (!actions.length && !['Running', 'Failed', 'Finishing'].includes(status)) return null;
  const summary = pending.length ? `Waiting for ${pending.length === 1 ? 'a model response' : `${pending.length} model responses`}`
    : status === 'Running' ? 'Worker running · No active call recorded'
    : status === 'Failed' ? 'Work failed · Inspect the recorded failure'
    : 'No active calls';
  return <section className={`portal-worker ${pending.length ? 'has-active-call' : ''}`} aria-label="Worker activity">
    <div className="portal-section-head"><h2>Worker activity</h2><a href={activityHref}>View records</a></div>
    <p className="portal-worker-current" role="status"><span className="portal-worker-lamp" aria-hidden="true"/>{summary}</p>
    {visible.length > 0 && <ul className="portal-worker-actions">{visible.map(a => <li key={a.id} data-active={a.active}>
      <div><a href={a.href}>{a.label}</a><small>{a.target}</small></div>
      <span className={`portal-worker-state ${/deny|denied|failed|cancelled/i.test(a.state) ? 'negative' : ''}`}>{a.state}</span>
    </li>)}</ul>}
  </section>;
}
