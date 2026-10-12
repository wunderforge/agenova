// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';
import { groupWorkerTurns, type WorkerAction } from './worker-activity-model';

export function WorkerActivity({ actions, status, activityHref, failure }: { actions: WorkerAction[]; status: string; activityHref: string; failure?: {reason: string; href: string} }) {
  const [chosen, setChosen] = useState<string>();
  const [followLatest, setFollowLatest] = useState(true);
  const pending = actions.filter(a => a.active);
  const groups = groupWorkerTurns(actions);
  const latest = groups.at(-1)?.id;
  const turn = [...actions].reverse().find(a => a.label === 'Agent turn')?.target;
  const turns = actions.filter(a => a.label === 'Agent turn' && a.state === 'Started').length;
  const observations = actions.filter(a => a.label === 'Agent turn' && a.state === 'Observation received').length;
  const toolsWaiting = pending.some(a => a.label === 'Tool call');
  const blockedActions = actions.filter(a => a.label === 'Tool access' && /deny|approval/i.test(a.state));
  if (!actions.length && !['Running', 'Failed', 'Finishing'].includes(status)) return null;
  const summary = pending.length ? toolsWaiting ? 'Waiting for tool observation' : `Waiting for ${pending.length === 1 ? 'a model response' : `${pending.length} model responses`}`
    : status === 'Running' ? 'Worker running · No active call recorded'
    : status === 'Failed' ? 'Work failed · Inspect the recorded failure'
    : 'No active calls';
  return <section className={`portal-worker ${pending.length ? 'has-active-call' : ''}`} aria-label="Worker activity">
    <div className="portal-section-head"><div><h2>Agent activity</h2><p className="portal-activity-help">{turn ? 'Model calls and tool results, grouped by turn.' : actions.some(a => ['Model request', 'Tool call'].includes(a.label)) ? 'Recorded model and tool calls.' : 'Access checks only; no execution calls recorded.'}</p></div><a href={activityHref}>View records</a></div>
    <p className="portal-worker-current" data-negative={status === 'Failed'} role="status"><span>{turn && `${turn} · `}{summary}</span></p>
    {blockedActions.length > 0 && <div className="portal-agent-failure portal-blocked-attempt" role="status">
      <strong>{blockedActions.length} blocked tool {blockedActions.length === 1 ? 'attempt' : 'attempts'}</strong>
      <p>{blockedActions.map(a => a.target).join(', ')} {blockedActions.length === 1 ? 'was' : 'were'} denied by the Tool Gateway.</p>
      <a href={blockedActions[0].href}>Inspect denial record</a>
    </div>}
    {turns > 0 && <p className="portal-turn-count">{turns} model turns · {observations} tool observations</p>}
    <div className="portal-turns">{groups.map(group => {
      const open = followLatest ? group.id === latest : chosen === group.id;
      const failures = group.calls.filter(a => /failed/i.test(a.state)).length;
      const blocked = group.calls.filter(a => /deny|denied/i.test(a.state)).length;
      const retries = group.calls.filter(a => a.state === 'Retry required').length;
      const callCount = group.calls.length - retries;
      const noun = group.calls.every(a => ['Tool access', 'Model access'].includes(a.label)) && group.calls.length ? 'check' : 'call';
      return <details className="portal-turn" key={group.id} open={open} data-active={group.active} data-negative={failures > 0 || blocked > 0}>
        <summary onClick={event => {
          event.preventDefault();
          setFollowLatest(!open && group.id === latest);
          setChosen(open ? undefined : group.id);
        }}>
          <strong>{group.id}</strong><span>{group.active ? 'In progress' : `${callCount} ${noun}${callCount === 1 ? '' : 's'}`}{group.observations > 0 && ` · ${group.observations} ${group.observations === 1 ? 'observation' : 'observations'}`}{retries > 0 && <span className="portal-turn-retry"> · Format retry</span>}{failures > 0 && <span className="portal-turn-failed"> · {failures} failed</span>}{blocked > 0 && <span className="portal-turn-failed"> · {blocked} blocked</span>}</span>
        </summary>
        <ul className="portal-worker-actions">{group.calls.map(a => <li key={a.id} data-active={a.active} data-retry={a.state === 'Retry required'} data-kind={a.label === 'Tool call' ? 'tool' : a.label === 'Model request' ? 'model' : 'access'} data-negative={/deny|denied|failed|cancelled/i.test(a.state)}>
          <span className="portal-worker-lamp" aria-hidden="true"/>
          <div><a href={a.href}>{a.label}</a><small>{a.target}</small></div>
          <span className={`portal-worker-state ${/deny|denied|failed|cancelled/i.test(a.state) ? 'negative' : /^(succeeded|allowed|allow)$/i.test(a.state) ? 'positive' : ''}`}>{a.state}</span>
        </li>)}</ul>
        {!group.calls.length && <p className="portal-source-note">No execution call recorded for this turn yet.</p>}
      </details>;
    })}</div>
    {failure && <div className="portal-agent-failure" role="alert">
      <strong>Work failed</strong><p>{failure.reason}</p><a href={failure.href}>View failure record</a>
    </div>}
  </section>;
}
