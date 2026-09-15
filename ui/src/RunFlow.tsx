// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, type CSSProperties } from 'react';

export interface FlowEvidence {
  status: string;
  received: boolean;
  authorized: boolean;
  started: boolean;
  modelRequested: boolean;
  modelFinished: boolean;
  modelActive?: boolean;
  result: boolean;
  cleanup: boolean;
  failureAt?: 'Request' | 'Access' | 'Worker' | 'Model' | 'Result' | 'Cleanup';
}
export interface FlowStage { label: string; state: 'done' | 'active' | 'blocked' | 'waiting'; detail: string }
const labels = ['Request', 'Access', 'Worker', 'Model', 'Result', 'Cleanup'] as const;

// Presence is evidence, not an assumption that all preceding stages completed.
export function flowStages(e: FlowEvidence): FlowStage[] {
  const terminal = ['Succeeded', 'Failed', 'Denied', 'Cancelled', 'Expired', 'Approval required'].includes(e.status);
  const observed = [e.received, e.authorized, e.started, e.modelFinished, e.result, e.cleanup];
  const failureAt = e.status === 'Denied' ? 'Access' : e.failureAt;
  const active = failureAt || terminal ? -1
    : e.status === 'Finishing' ? e.cleanup ? 4 : 5
    : e.status === 'Running' ? (e.modelActive ?? (e.modelRequested && !e.modelFinished)) ? 3 : e.modelFinished ? 4 : 2
    : e.status === 'Starting' ? 2 : e.authorized ? 2 : 1;
  return labels.map((label, i) => ({
    label,
    state: label === failureAt ? 'blocked' : i === active ? 'active' : observed[i] ? 'done' : 'waiting',
    detail: label === failureAt ? e.status === 'Succeeded' ? 'Needs attention' : e.status
      : i === active ? i === 2 && e.status !== 'Running' ? 'Starting' : ['Received', 'Resolving', 'Running', 'Inference', 'Awaiting result', 'Releasing'][i]
      : observed[i] ? ['Received', 'Resolved', 'Started', 'Response recorded', 'Completed', 'Released'][i]
      : i === 3 && e.modelRequested ? 'Request recorded'
      : e.status === 'Denied' ? 'Not reached' : terminal ? 'No record' : 'Waiting',
  }));
}
export function RunFlow({ evidence, activityHref }: { evidence: FlowEvidence; activityHref: string }) {
  const trackRef = useRef<HTMLOListElement>(null);
  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    const size = () => track.style.setProperty('--packet-distance', Math.max(0, (track.firstElementChild?.getBoundingClientRect().width || 38) - 38) + 'px');
    size();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(size);
    observer.observe(track);
    return () => observer.disconnect();
  }, []);
  const stages = flowStages(evidence);
  return <section className="portal-flow" aria-label="Run flow">
    <div className="portal-flow-head"><h2>Run flow</h2><span>{evidence.status}</span></div>
    <ol ref={trackRef} className={`portal-flow-track ${evidence.status === 'Running' && !evidence.failureAt ? 'in-flight' : ''}`}>
      {stages.map((stage, index) => <li key={stage.label} className={`portal-flow-node ${stage.state}`}
        style={{ '--stage-index': index } as CSSProperties}>
        <span className="portal-flow-orb" aria-hidden="true">{stage.state === 'done' ? '✓' : stage.state === 'blocked' ? '×' : index + 1}</span>
        <a href={activityHref}>{stage.label}</a><small>{stage.detail}</small>
      </li>)}
    </ol>
  </section>;
}
