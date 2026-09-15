// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

export interface FlowEvidence {
  status: string;
  received: boolean;
  authorized: boolean;
  started: boolean;
  result: boolean;
  cleanup: boolean;
  failureAt?: 'Request' | 'Access' | 'Worker' | 'Model' | 'Result' | 'Cleanup';
}
export interface FlowStage { label: string; state: 'done' | 'active' | 'blocked' | 'waiting'; detail: string }
const labels = ['Request', 'Access', 'Worker', 'Result', 'Cleanup'] as const;

// Presence is evidence, not an assumption that all preceding stages completed.
export function flowStages(e: FlowEvidence): FlowStage[] {
  const terminal = ['Succeeded', 'Failed', 'Denied', 'Cancelled', 'Expired', 'Approval required'].includes(e.status);
  const observed = [e.received, e.authorized, e.started, e.result, e.cleanup];
  const failureAt = e.status === 'Denied' ? 'Access' : e.failureAt === 'Model' ? 'Worker' : e.failureAt;
  const active = failureAt || terminal ? -1
    : e.status === 'Finishing' ? e.cleanup ? 3 : 4
    : e.status === 'Running' ? 2
    : e.status === 'Starting' ? 2 : e.authorized ? 2 : 1;
  return labels.map((label, i) => ({
    label,
    state: label === failureAt ? 'blocked' : i === active ? 'active' : observed[i] ? 'done' : 'waiting',
    detail: label === failureAt ? e.status === 'Succeeded' ? 'Needs attention' : e.status
      : i === active ? i === 2 && e.status !== 'Running' ? 'Starting' : ['Received', 'Resolving', 'Running', 'Awaiting result', 'Releasing'][i]
      : observed[i] ? ['Received', 'Resolved', 'Started', 'Completed', 'Released'][i]
      : e.status === 'Denied' ? 'Not reached' : terminal ? 'No record' : 'Waiting',
  }));
}
export function RunFlow({ evidence, activityHref }: { evidence: FlowEvidence; activityHref: string }) {
  const stages = flowStages(evidence);
  return <section className="portal-flow" aria-label="Run flow">
    <div className="portal-flow-head"><h2>Run flow</h2><span>{evidence.status}</span></div>
    <ol className="portal-flow-track">
      {stages.map((stage, index) => <li key={stage.label} className={`portal-flow-node ${stage.state}`}>
        <span className="portal-flow-orb" aria-hidden="true">{stage.state === 'done' ? '✓' : stage.state === 'blocked' ? '×' : index + 1}</span>
        <a href={activityHref}>{stage.label}</a><small>{stage.detail}</small>
      </li>)}
    </ol>
  </section>;
}
