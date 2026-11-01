// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';
import { groupWorkerTurns, toolCallLabels, type WorkerAction } from './worker-activity-model';
import {ActivityContent} from './ActivityContent';

export function WorkerActivity({ actions, status, activityHref, failure }: { actions: WorkerAction[]; status: string; activityHref: string; failure?: {reason: string; href: string} }) {
  const [chosen, setChosen] = useState<string>();
  const [followLatest, setFollowLatest] = useState(true);
  const [selected, setSelected] = useState<string>();
  const pending = actions.filter(a => a.active);
  const groups = groupWorkerTurns(actions);
  const latest = groups.at(-1)?.id;
  useEffect(()=>{const inspect=(event:Event)=>{const id=(event as CustomEvent<string>).detail;const group=groupWorkerTurns(actions).find(g=>g.calls.some(a=>a.id===id));if(group){setFollowLatest(false);setChosen(group.id);setSelected(id);}};window.addEventListener('agenova:inspect-fact',inspect);return()=>window.removeEventListener('agenova:inspect-fact',inspect);},[actions]);

  const turn = [...actions].reverse().find(a => a.label === 'Agent turn')?.target;
  const turns = actions.filter(a => a.label === 'Agent turn' && a.state === 'Started').length;
  const observations = actions.filter(a => a.label === 'Agent turn' && a.state === 'Observation received').length;
  const toolsWaiting = pending.some(a => toolCallLabels.includes(a.label));
  const inspectable = groups.flatMap(group => group.calls);
  const inspected = inspectable.find(action => action.id === selected) || inspectable.filter(a=>a.label==='Model request'||toolCallLabels.includes(a.label)).at(-1) || inspectable.at(-1);
  const toolFailures=actions.filter(a=>toolCallLabels.includes(a.label)&&a.state==='Failed').length;
  if (!actions.length && !['Running', 'Failed', 'Finishing'].includes(status)) return null;
  const summary = pending.length ? toolsWaiting ? 'Waiting for tool observation' : `Waiting for ${pending.length === 1 ? 'a model response' : `${pending.length} model responses`}`
    : status === 'Running' ? 'Worker running · No active call recorded'
    : status === 'Failed' ? 'Work failed · Inspect the recorded failure'
    : toolFailures>0 ? `Finished · ${toolFailures} tool ${toolFailures===1?'call failed':'calls failed'}`
    : 'No active calls';
  return <section className={`portal-worker ${pending.length ? 'has-active-call' : ''}`} aria-label="Worker activity">
    <div className="portal-section-head"><div><h2>Agent activity</h2><p className="portal-activity-help">{turn ? 'Model calls and tool results, grouped by turn.' : actions.some(a => ['Model request', ...toolCallLabels].includes(a.label)) ? 'Recorded model and tool calls.' : 'Access checks only; no execution calls recorded.'}</p></div><a href={activityHref}>View records</a></div>
    <p className="portal-worker-current" data-negative={status === 'Failed'} role="status"><span>{turn && `${turn} · `}{summary}</span></p>
    {turns > 0 && <p className="portal-turn-count">{turns} turns · {observations} tool observations</p>}
    <div className="portal-trace-layout"><div className="portal-turns">{groups.map(group => {
      const open = followLatest ? group.id === latest : chosen === group.id;
      const failures = group.calls.filter(a => /failed/i.test(a.state)).length;
      const blocked = group.calls.filter(a => /deny|denied/i.test(a.state)).length;
      const retries = group.calls.filter(a => a.state === 'Retry required').length;
      const retryLabel=group.calls.some(a=>a.label==='Agent feedback'&&a.state==='Retry required')?'Agent retry':'Format retry';
      return <details className="portal-turn" key={group.id} open={open} data-active={group.active} data-negative={failures > 0 || blocked > 0}>
        <summary onClick={event => {
          event.preventDefault();
          setFollowLatest(!open && group.id === latest);
          setChosen(open ? undefined : group.id);
        }}>
          <strong>{group.id}</strong><span>{group.active ? 'In progress' : `${group.calls.length} records`}{group.observations > 0 && ` · ${group.observations} ${group.observations === 1 ? 'observation' : 'observations'}`}{retries > 0 && <span className="portal-turn-retry"> · {retryLabel}</span>}{failures > 0 && <span className="portal-turn-failed"> · {failures} failed</span>}{blocked > 0 && <span className="portal-turn-blocked"> · {blocked} blocked</span>}</span>
        </summary>
        <ul className="portal-worker-actions">{group.calls.map(a => <li id={`work-fact-${a.id}`} key={a.id} data-active={a.active} data-retry={a.state === 'Retry required'} data-denied={/deny|denied/i.test(a.state)} data-kind={toolCallLabels.includes(a.label) ? 'tool' : a.label === 'Model request' ? 'model' : 'access'} data-negative={/deny|denied|failed|cancelled/i.test(a.state)}>
          <span className="portal-worker-lamp" aria-hidden="true"/>
          <div><a href={a.href}>{a.label}</a><small>{a.target}{a.elapsedMs!==undefined&&Number.isFinite(a.elapsedMs)&&<span className="portal-call-duration"> · {(a.elapsedMs/1000).toFixed(1)} s</span>}</small></div>
          <span className={`portal-worker-state ${/deny|denied|failed|cancelled/i.test(a.state) ? 'negative' : /^(succeeded|allowed|allow)$/i.test(a.state) ? 'positive' : ''}`}>{a.state}</span>
          <button type="button" className="portal-inspect-button" aria-label={`Inspect ${a.label}: ${a.target}`} aria-pressed={inspected?.id === a.id} onClick={() => setSelected(a.id)}>Inspect</button>
        </li>)}</ul>
        {!group.calls.length && <p className="portal-source-note">No execution call recorded for this turn yet.</p>}
      </details>;
    })}</div>{inspected && <aside className="portal-call-inspector" aria-label="Selected activity">
      <span className="portal-inspector-eyebrow">SELECTED RECORD</span><h3>{inspected.label}</h3>
      <dl><dt>Target</dt><dd>{inspected.target}</dd><dt>Recorded status</dt><dd>{inspected.state}</dd><dt>Turn</dt><dd>{inspected.turn || 'Not reported'}</dd>{inspected.reason&&<><dt>Reason</dt><dd>{inspected.reason}</dd></>}</dl>
      <a href={inspected.href}>Evidence</a>
      <ActivityContent work={inspected.workRef} reference={inspected.contentRef}/>
    </aside>}</div>
    {failure && <div className="portal-agent-failure" role="alert">
      <strong>Work failed</strong><p>{failure.reason}</p><a href={failure.href}>View failure record</a>
    </div>}
  </section>;
}
