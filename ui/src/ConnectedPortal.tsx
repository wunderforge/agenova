// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState, type ReactNode } from 'react';
import type { ClaimRequestedAccess, EffectiveAuthority, Fact as Observation, Principal } from './contracts.generated';
import { connectedSource, workStatus, workTitle, type HistoryPage, type Setup, type View } from './connected-source';
import { RunFlow } from './RunFlow';
import { WorkerActivity } from './WorkerActivity';
import { workerActions } from './worker-activity-model';
import { category, recordTitle, recordReason, recordStatus, outcomeReason } from './record-presentation';
import { useConnection } from './useConnection';
import { WorkDetailLayout } from './WorkDetailLayout';
import { TaskInstructions, CopyRequestID } from './TaskInstructions';
import { WorkSubmissionCopy } from './WorkSubmission';
import { ConfigurationContext, WorkComparison, PolicyRules, PolicyHistory } from './PresenterViews';
import { WorkPermissions, WorkContext, WorkNotices } from './WorkAuthority';
import {AgentDirectory,PlatformOverview} from './ProductPages';
import { WorkOverview } from './WorkOverview';
import { WorkActivityChart } from './WorkActivityChart';

const link = (path: string) => `#/${path}`;
const workLink = (work: View) => `work/${encodeURIComponent(work.requestRef)}`;
const time = (value?: string) => value
  ? new Date(value).toLocaleTimeString('en-AU', { hour12: false }) : 'Not recorded';

function Badge({ value }: { value: string }) {
  return <span className={`portal-badge ${value.toLowerCase().replace(/\s+/g, '-')}`}>{value}</span>;
}
function Chips({ values }: { values?: string[] | null }) {
  return values?.length
    ? <div className="portal-chips">{values.map(value => <code key={value}>{value}</code>)}</div>
    : <span className="portal-muted">None</span>;
}
function Field({ label, value }: { label: string; value?: ReactNode }) {
  return <div className="portal-fact"><span>{label}</span><strong>{value ?? 'Not recorded'}</strong></div>;
}
function Heading({ title, subtitle, action, status }: { title: string; subtitle?: string; action?: ReactNode; status?:ReactNode }) {
  return <div className="portal-head">
    <div className="portal-head-info"><div className="portal-title-line"><h1>{title}</h1>{status}</div>{subtitle && <p>{subtitle}</p>}</div>{action&&<div className="portal-head-actions">{action}</div>}
  </div>;
}
function Access({ value, runtime, networkRecorded = false, requested = false }: {
  value?: ClaimRequestedAccess | EffectiveAuthority | null; runtime?: string;
  networkRecorded?: boolean; requested?: boolean;
}) {
  if (!value) return <p>No authority was issued.</p>;
  return <div className="portal-access-fields">
    <Field label="Resource scope" value={<Chips values={value.resourceScopes}/>}/>
    <Field label="Tools" value={<Chips values={value.tools}/>}/>
    <Field label="Model profile" value={value.modelProfile || 'None'}/>
    <Field label="Memory scope" value={<Chips values={value.memoryScopes}/>}/>
    <Field label="Network profiles" value={value.networkProfiles?.length ? <Chips values={value.networkProfiles}/>
      : requested ? 'No public network requested' : networkRecorded ? 'No public network granted' : 'Not recorded; runtime boundary unknown'}/>
    <Field label="Runtime" value={runtime}/>
  </div>;
}

function Records({ work, observations }: { work: View; observations: Observation[] }) {
  return <div className="portal-record-list">
    {observations.length ? observations.map(fact =>
      <div className="portal-record-row" key={fact.id}>
        <time>{time(fact.timestamp)}</time>
        <div>
          <a href={link(`${workLink(work)}/activity/${encodeURIComponent(fact.id)}`)}>{recordTitle(fact)}</a>
          <small>{fact.target || recordReason(fact)}</small>
        </div>
        <span className="portal-record-source">{category(fact)}</span>
        <Badge value={recordStatus(fact)}/>
      </div>
    ) : <p className="portal-empty">No activity recorded yet.</p>}
  </div>;
}
function Filters({ values, selected, choose }: {
  values: string[]; selected: string; choose: (value: string) => void;
}) {
  return <div className="portal-filters">{values.map(value =>
    <button key={value} className={selected === value ? 'selected' : ''}
      onClick={() => choose(value)}>{value}</button>
  )}</div>;
}
function WorkList({ works }: { works: View[] }) {
  const [filter, setFilter] = useState('All');
  const [search, setSearch] = useState('');
  const shown = works.filter(work => (filter === 'All' || workStatus(work) === filter) &&
    `${workTitle(work)} ${work.request.spec.task?.input?.objective || ''} ${work.requestRef} ${work.state?.principal.team || ''}`.toLowerCase().includes(search.toLowerCase()));
  return <>
    <Heading title="Work" subtitle="Requests, outcomes, and the evidence behind them."
      action={<><a className="portal-button" href={link('compare')}>Compare two Work</a><a className="portal-button primary" href={link('work/new')}>New work</a></>}/>
    <a href={link('history')}>Browse durable history</a>
    <WorkOverview statuses={works.map(workStatus)}/>
    <WorkActivityChart entries={works.map(work => ({ timestamp: work.facts.find(fact => ['RequestReceived', 'RequestResolution'].includes(fact.kind))?.timestamp, status: workStatus(work) }))}/>
    <div className="portal-toolbar">
      <Filters values={['All', 'Running', 'Succeeded', 'Denied', 'Failed', 'Pending']}
        selected={filter} choose={setFilter}/>
      <input type="search" aria-label="Search work" placeholder="Search work" value={search}
        onChange={event => setSearch(event.target.value)}/>
    </div>
    <div className="portal-table-wrap"><table className="portal-table">
      <thead><tr><th>Work</th><th>Agent</th><th>Requested by</th><th>Decision</th><th>Last update</th><th>Status</th></tr></thead>
      <tbody>{shown.map(work =>
        <tr key={work.requestRef}>
          <td><a className="portal-work-title" href={link(workLink(work))}>{workTitle(work)}</a>
            <small>{work.state?.principal.team || 'Identity not recorded'}</small></td>
          <td>{work.request.spec.templateRef}</td><td>{work.state?.principal.subject || 'Not recorded'}</td>
          <td><Badge value={work.state?.decision.result || 'Pending'}/></td><td>{time(work.facts.at(-1)?.timestamp)}</td>
          <td><Badge value={workStatus(work)}/></td>
        </tr>
      )}</tbody>
    </table>{!shown.length && <p className="portal-empty">
      {works.length ? 'No work matches this filter.' : 'No work recorded yet.'}
    </p>}</div>
  </>;
}
function History() {
 const [cursor, setCursor] = useState('');
 const [page, setPage] = useState<HistoryPage>();
 const [error, setError] = useState('');
 useEffect(() => {
  const controller = new AbortController(); setPage(undefined); setError('');
  connectedSource.history(cursor, controller.signal).then(setPage).catch(() => { if (!controller.signal.aborted) setError('History is unavailable.'); });
  return () => controller.abort();
 }, [cursor]);
 return <><Heading title="Work history" subtitle="Stored governance evidence"/>
  {error && <p role="alert">{error}</p>}
  <div className="portal-table-wrap"><table className="portal-table"><thead><tr><th>Work</th><th>Status</th><th>Last update</th><th>Diagnosis</th></tr></thead>
   <tbody>{page?.items.map(item => <tr key={item.requestRef}><td><a href={link(`work/${encodeURIComponent(item.requestRef)}`)}>{item.requestRef}</a></td><td>{item.status==='Succeeded'&&item.completion?.status==='incomplete'?'Incomplete':item.status==='Succeeded'&&item.completion?.status==='partial'?'Partial':item.status === 'Deny' ? 'Denied' : item.status}</td><td>{time(item.updated)}</td><td>{item.completion?.reason|| (item.status === 'Succeeded' ? '—' : item.diagnosis?.code || '—')}</td></tr>)}</tbody></table></div>
  {page?.nextCursor && <button className="portal-button" onClick={() => setCursor(page.nextCursor!)}>Next page</button>}
  {cursor && <button className="portal-button" onClick={() => setCursor('')}>Newest work</button>}
 </>;
}
function WorkDetail({ work }: { work: View }) {
  const state = work.state;
  const runtime = state?.effectiveAuthority?.runtime;
  const status = workStatus(work);
  const ended = !!work.recovery || (!!state?.claim && !['Pending', 'Bound', 'Running'].includes(state.claim.phase));
  const latestFacts = [...work.facts].reverse();
  const lastModel = latestFacts.find(fact => ['ProviderAttempt', 'ProviderOutcome'].includes(fact.kind) && fact.operation !== 'tool.invoke');
  const lastCleanup = latestFacts.find(fact => ['CleanupSucceeded', 'CleanupFailed'].includes(fact.operation || ''));
  const progress = work.facts.filter(fact =>
    !['WorkerActivity', 'ModelDecision', 'ProviderAttempt', 'ProviderOutcome', 'ToolDecision'].includes(fact.kind));
  const summary = work.recovery ? 'Execution interrupted by service restart' : status === 'Finishing' ? 'Waiting for the final result and cleanup evidence.'
    : status==='Incomplete'?'Execution finished; task incomplete.':status==='Partial'?'Execution finished; task partially completed.'
    : work.outcome?.status === 'Succeeded' ? 'Task completed.'
    : work.outcome?.failure || (status === 'Running' ? 'Agent is working. Current calls are shown below.' : status === 'Starting' ? 'Starting the agent.' : state?.decision.reason) || 'Request received; waiting for authorization.';
  return <>
    <nav className="portal-crumbs" aria-label="Breadcrumb">
      <a href={link('work')}>Work</a> / Details
    </nav>
    <Heading title={workTitle(work)} status={<Badge value={status}/>} action={<a className="portal-button" href={link(`work/new?copy=${encodeURIComponent(work.requestRef)}`)}>Copy as new Work</a>}/>
    {status!=='Succeeded'&&<WorkNotices work={work}/>}
    <WorkPermissions work={work}/>
    {status==='Succeeded'&&<WorkNotices work={work}/>}
    <WorkContext work={work}/>
    <TaskInstructions text={work.request.spec.task?.input?.objective}/>
    <WorkDetailLayout compact activityHref={link(`${workLink(work)}/activity`)} status={<RunFlow summary={summary} activityHref={link(`${workLink(work)}/activity`)} evidence={{
      status,
      received: work.facts.some(fact => fact.kind === 'RequestReceived'),
      authorized: state?.decision.result === 'Allow' && !!state.effectiveAuthority,
      started: work.facts.some(fact => fact.operation === 'Running'),
      result: work.outcome?.status === 'Succeeded',
      cleanup: lastCleanup?.operation === 'CleanupSucceeded',
      failureAt: lastCleanup?.operation === 'CleanupFailed' ? 'Cleanup'
        : status === 'Failed' && lastModel?.kind === 'ProviderOutcome' && lastModel.providerStatus === 'Failed' ? 'Model'
        : ['Failed', 'Expired', 'Cancelled'].includes(status)
          ? work.facts.some(fact => fact.operation === 'Running') ? 'Worker' : 'Request' : undefined,
    }}/>} activity={<WorkerActivity actions={workerActions(work.facts, status, link(`${workLink(work)}/activity`))} status={status} activityHref={link(`${workLink(work)}/activity`)} failure={status === 'Failed' ? {
          reason: outcomeReason(work),
          href: link(`${workLink(work)}/activity/${encodeURIComponent(latestFacts.find(f => f.kind === 'RunOutcome')?.id || latestFacts.find(f => f.operation === 'Failed')?.id || '')}`),
        } : undefined}/>} access={<aside className="portal-access-card">
        <h2>{state?.effectiveAuthority
          ? !ended && state.claim?.phase === 'Running' ? 'Active access' : 'Issued access' : 'No access issued'}</h2>
        <Access value={state?.effectiveAuthority}
          networkRecorded={work.facts.some(fact => fact.operation === 'NetworkActive')}
          runtime={runtime ? `${runtime.profileRef} · ${runtime.timeout}` : undefined}/>
        <div className="portal-card-links">
          <a href={link(`${workLink(work)}/access`)}>Compare requested and granted</a>
          <a href={link(`policy?project=${encodeURIComponent(work.request.spec.projectRef || '')}&id=${encodeURIComponent(state?.policyRef.id || '')}&version=${encodeURIComponent(state?.policyRef.version || '')}`)}>View policy at authorization</a>
        </div>
      </aside>} result={work.outcome?.text && <section className="portal-result">
      <h2>Result</h2><pre>{work.outcome.text}</pre>
      <details className="portal-details"><summary>Related call evidence</summary>{work.facts.filter(fact=>fact.kind==='ProviderOutcome' && fact.providerStatus==='Succeeded').map(fact=><p key={fact.id}><a href={link(`${workLink(work)}/activity/${encodeURIComponent(fact.id)}`)}>{category(fact)} · {fact.target || fact.invocationId}</a>{fact.resultRef && <code> {fact.resultRef}</code>}</p>)}</details>
      {work.outcome.model && <p className="portal-source-note">
        Final model call: {work.outcome.model.model} · {work.outcome.model.inputTokens} input /
        {' '}{work.outcome.model.outputTokens} output tokens
      </p>}
    </section>} details={<>
      {work.traceId && <div className="portal-trace-reference"><span>Trace ID: <code>{work.traceId}</code></span><button className="portal-button" onClick={() => void navigator.clipboard.writeText(work.traceId!)}>Copy trace ID</button></div>}
      <small className="portal-source-note">{time(work.facts.at(-1)?.timestamp)} last update</small>
      <ol className="portal-timeline">{progress.map(fact =>
        <li key={fact.id} data-negative={recordStatus(fact) === 'Failed'}>
          <a href={link(`${workLink(work)}/activity/${encodeURIComponent(fact.id)}`)}>{recordTitle(fact)}</a>
          {recordStatus(fact) === 'Failed' && <Badge value="Failed"/>}
          {(recordReason(fact) || (fact.kind === 'RunOutcome' && recordStatus(fact) === 'Failed')) && <p>{recordReason(fact) || outcomeReason(work)}</p>}
        </li>
      )}</ol>
      <div className="portal-fact-grid">
        <Field label="Request ID" value={work.requestRef}/>
        <Field label="Claim ID" value={state?.claim?.id}/>
        <Field label="Runtime backend" value={state?.claim?.backendIdentity?.backend}/>
        <Field label="Worker ID" value={state?.claim?.backendIdentity?.workerId}/>
        <Field label="Policy version" value={state ? `${state.policyRef.id} / ${state.policyRef.version}` : undefined}/>
      </div>
      <CopyRequestID value={work.requestRef}/>
    </>}/>
  </>;
}
function WorkAccess({ work }: { work: View }) {
  const runtime = work.request.spec.runtime;
  const effective = work.state?.effectiveAuthority;
  return <>
    <nav className="portal-crumbs"><a href={link(workLink(work))}>{workTitle(work)}</a> / Access</nav>
    <Heading title="Access for this work" action={<Badge value={workStatus(work)}/>}/>
    <div className="portal-comparison">
      <section className="portal-compare-side"><h2>Requested</h2>
        <Access value={work.request.spec.requestedAccess}
          requested
          runtime={runtime ? `${runtime.profileRef} · ${runtime.timeout}` : undefined}/>
      </section>
      <section className="portal-compare-side"><h2>Granted</h2>
        <Access value={effective}
          networkRecorded={work.facts.some(fact => fact.operation === 'NetworkActive')}
          runtime={effective ? `${effective.runtime.profileRef} · ${effective.runtime.timeout}` : undefined}/>
      </section>
    </div>
    <section className="portal-lower"><h2>Request resolution</h2>
      <Records work={work} observations={work.facts.filter(fact => fact.decision || fact.effectiveAuthority)}/>
    </section>
  </>;
}
function Activity({ work, selected }: { work: View; selected?: string }) {
  const [filter, setFilter] = useState('All');
  const fact = selected ? work.facts.find(item => item.id === decodeURIComponent(selected)) : undefined;
  if (selected) {
    if (!fact) return <Heading title="Record not found"/>;
    const policy = fact.policyRef || fact.decision?.policyRef;
    return <>
      <nav className="portal-crumbs"><a href={link(`${workLink(work)}/activity`)}>Activity</a></nav>
      <Heading title={recordTitle(fact)}
        action={<Badge value={recordStatus(fact)}/>}/>
      {recordStatus(fact) === 'Failed' && <div className="portal-state failed" role="alert">
        <strong>{recordTitle(fact)}</strong>
        <p>{recordReason(fact) || (fact.kind === 'RunOutcome' ? outcomeReason(work) : 'No specific failure reason was recorded for this event.')}</p>
      </div>}
      <section className="portal-panel"><h2>What this record says</h2>
        <p>{recordReason(fact) || recordTitle(fact)}</p>
        <div className="portal-fact-grid">
          <Field label="Kind" value={category(fact)}/><Field label="Target" value={fact.target}/>
          <Field label="Permission" value={fact.result || fact.decision?.result}/>
          <Field label="Provider outcome" value={fact.providerStatus}/>
          {fact.resultRef && <Field label="Result reference" value={fact.resultRef}/>}
          {fact.truncated && <Field label="Observation" value="Truncated to the configured observation limit"/>}
          <Field label="Claim" value={fact.claimId}/><Field label="Invocation" value={fact.invocationId}/>
          <Field label="Policy" value={policy ? `${policy.id} / ${policy.version}` : undefined}/>
        </div>
        {fact.effectiveAuthority && <><h3>Effective authority at resolution</h3>
          <Access value={fact.effectiveAuthority}
            networkRecorded={work.facts.some(record => record.operation === 'NetworkActive')}
            runtime={`${fact.effectiveAuthority.runtime.profileRef} · ${fact.effectiveAuthority.runtime.timeout}`}/>
        </>}
        {!!fact.authorityChanges?.length && <div className="portal-table-wrap"><table className="portal-table">
          <thead><tr><th>Field</th><th>Requested</th><th>Effective</th></tr></thead>
          <tbody>{fact.authorityChanges.map((change, index) => <tr key={index}>
            <td>{change.field}</td><td>{change.requested}</td><td>{change.effective}</td>
          </tr>)}</tbody>
        </table></div>}
        <details className="portal-details"><summary>Show record JSON</summary>
          <pre>{JSON.stringify(fact, null, 2)}</pre>
        </details>
      </section>
    </>;
  }
  return <>
    <nav className="portal-crumbs"><a href={link(workLink(work))}>{workTitle(work)}</a> / Activity</nav>
    <Heading title="Activity"/>
    <Filters values={['All', ...new Set(work.facts.map(category))]} selected={filter} choose={setFilter}/>
    <Records work={work}
      observations={filter === 'All' ? work.facts : work.facts.filter(fact => category(fact) === filter)}/>
  </>;
}
function Platform({ setup, works, activity }: { setup: Setup; works: View[]; activity: boolean }) {
  const [filter, setFilter] = useState('All');
  const records = works.flatMap(work => work.facts.map(fact => ({ work, fact })))
    .filter(({ fact }) => filter === 'All' || category(fact) === filter)
    .sort((a, b) => b.fact.timestamp.localeCompare(a.fact.timestamp));
  if (activity) {
    return <>
      <nav className="portal-crumbs"><a href={link('platform')}>Platform</a> / Activity</nav>
      <Heading title="Governance activity"
        subtitle={`${records.length} records across ${new Set(records.map(item => item.work.requestRef)).size} work requests`}/>
      <Filters values={['All', ...new Set(works.flatMap(work => work.facts.map(category)))]}
        selected={filter} choose={setFilter}/>
      <div className="portal-platform-records">{records.map(({ work, fact }) =>
        <div className="portal-platform-row" key={`${work.requestRef}-${fact.id}`}>
          <time>{time(fact.timestamp)}</time>
          <div><small>{category(fact)}</small>
            <a href={link(`${workLink(work)}/activity/${encodeURIComponent(fact.id)}`)}>{recordTitle(fact)}</a>
            {recordReason(fact) && <p>{recordReason(fact)}</p>}
          </div>
          <div><a href={link(workLink(work))}>{workTitle(work)}</a>
            <small>{work.state?.principal.team}</small></div>
          <Badge value={recordStatus(fact)}/>
        </div>
      )}{!records.length && <p className="portal-empty">No governance activity recorded yet.</p>}</div>
    </>;
  }
  return <PlatformOverview setup={setup}/>;
}
function Policy({ setup }: { setup: Setup }) {
 const query=new URLSearchParams(window.location.hash.split('?')[1]||'');
 const historical=query.has('id')||query.has('version');
 if(historical)return <><Heading title="Policy" subtitle="Historical authorization document"/><PolicyHistory setup={setup}/></>;
 return <><Heading title="Policy" subtitle="Versioned rules. Explicit limits. Default deny."/>
 <section className="portal-panel"><div className="portal-head"><div><h2>{setup.policy.ID}</h2><p>Version {setup.policy.Version} · current configuration context</p></div><a className="portal-button" href="/admin#governance">Manage versions</a></div><p>Rules cap a request; they do not grant the template's full capabilities. Unmatched operations are denied.</p></section>
 <PolicyRules setup={setup}/><PolicyHistory setup={setup}/></>;
}
function Identity({ setup }: { setup: Setup }) {
  return <><Heading title="Identity"/><section className="portal-panel">
    <h2>{setup.principal.subject}</h2>
    <div className="portal-fact-grid">
      <Field label="Team" value={setup.principal.team}/>
      <Field label="Authentication context" value={setup.principal.authenticationContext}/>
    </div>
  </section></>;
}
export function ConnectedPortal({ parts, onDemo, onPrincipal }: { parts: string[]; onDemo: () => void; onPrincipal?: (principal: Principal | undefined) => void }) {
  const [revision, setRevision] = useState(0);
  const data = useConnection(parts, revision);
  useEffect(() => { onPrincipal?.(data.error ? undefined : data.setup?.principal); }, [data.error, data.setup, onPrincipal]);
  const retry = () => setRevision(value => value + 1);
  const section = (parts[0] || 'work').split('?')[0];
  if (data.loading) {
    return <div role="status" className="portal-connected-empty"><h2>Connecting…</h2></div>;
  }
  if (data.error || !data.setup) {
    const title = section === 'work' ? parts[1] ? 'Work details' : 'Work'
      : section === 'platform' ? 'Platform' : section === 'agents' ? 'Agents'
      : section === 'policy' ? 'Policy' : 'Identity';
    return <><Heading title={title}/>
      <div className="portal-connected-empty" role="alert">
        <Badge value="Unavailable"/><h2>{data.error || 'Platform setup is unavailable.'}</h2>
        <button className="portal-button" onClick={retry}>Try again</button>
      </div>
      <button className="portal-button" onClick={onDemo}>Explore in Demo</button>
    </>;
  }
  const setup = data.setup;
  let content: ReactNode;
  if (section === 'work' && parts[1]?.split('?')[0] === 'new') {
    const copyRef = new URLSearchParams(window.location.hash.split('?')[1] || '').get('copy');
    content = <WorkSubmissionCopy setup={setup} reference={copyRef} created={work => {
      window.location.hash = link(workLink(work)); retry();
    }}/>;
  } else if (section === 'compare') {
    content = <WorkComparison works={data.works}/>;
  } else if (section === 'work' && !parts[1]) {
    content = <WorkList works={data.works}/>;
  } else if (section === 'work' && data.current) {
    content = parts[2] === 'access' ? <WorkAccess work={data.current}/>
      : parts[2]==='completion'? <><nav className="portal-crumbs"><a href={link(workLink(data.current))}>{workTitle(data.current)}</a> / Completion</nav><Heading title="Task completion"/><section className="portal-panel"><h2>{data.current.outcome?.completion?.status||'Not reported'}</h2><p>{data.current.outcome?.text||'No explanation recorded.'}</p><Field label="Reason" value={data.current.outcome?.completion?.reason}/><Field label="Capability" value={data.current.outcome?.completion?.target}/><Field label="Execution status" value={data.current.outcome?.status}/><small>Agent-reported completion; not a Gateway decision or verified network verdict.</small></section><WorkPermissions work={data.current}/><a href={link(`${workLink(data.current)}/activity`)}>Activity &amp; evidence</a></>
      : parts[2] === 'activity' ? <Activity work={data.current} selected={parts[3]}/>
      : <WorkDetail key={data.current.requestRef} work={data.current}/>;
  } else if (section === 'history') {
 content=<History/>;
 } else if (section === 'platform') {
    content = <Platform setup={setup} works={data.works} activity={parts[1] === 'activity'}/>;
 } else if (section === 'agents') content = <AgentDirectory/>;
  else if (section === 'policy') content = <Policy setup={setup}/>;
  else if (section === 'identity') content = <Identity setup={setup}/>;
  else content = <Heading title="Work not found"/>;
  return <>
    {setup.installation.platform.startsWith('LOCAL ACCEPTANCE') && <p role="status" className="portal-local-warning">Local acceptance · synthetic identity/runtime/model · real Go authorization / PostgreSQL · MCP and source status shown in Platform · not AWS or infrastructure verification</p>}
    {data.paused && <p role="status" className="portal-source-note">
      Automatic refresh paused. <button className="portal-button" onClick={retry}>Refresh progress</button>
    </p>}
    {!['work','history','compare','agents','platform'].includes(section) && <ConfigurationContext setup={setup}/>}
    {content}
  </>;
}
