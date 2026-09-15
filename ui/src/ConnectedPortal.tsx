// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import type { ClaimRequest, ClaimRequestedAccess, EffectiveAuthority, Fact as Observation } from './contracts.generated';
import { connectedSource, isTerminal, workStatus, workTitle, type Setup, type View } from './connected-source';
import { RunFlow } from './RunFlow';
import { WorkerActivity, workerActions } from './WorkerActivity';

const link = (path: string) => `#/${path}`;
const workLink = (work: View) => `work/${encodeURIComponent(work.requestRef)}`;
const time = (value?: string) => value
  ? new Date(value).toLocaleTimeString('en-AU', { hour12: false }) : 'Not recorded';
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
const category = (fact: Observation) => fact.operation === 'tool.invoke' ? 'Tool Gateway' : categories[fact.kind] || fact.kind;
function recordTitle(fact: Observation): string {
  if (fact.kind === 'WorkerActivity') return `${fact.target || 'Agent'} · ${({TurnStarted:'Model turn started',ActionReceived:'Action received',ObservationReceived:'Tool observation received',FinalAnswer:'Final answer'} as Record<string,string>)[fact.operation || ''] || 'Recorded'}`;
  if (fact.kind === 'ProviderAttempt') return fact.operation === 'tool.invoke' ? 'Mock tool call started' : 'Model request started';
  if (fact.kind === 'ProviderOutcome') return fact.operation === 'tool.invoke' ? 'Mock tool call finished' : 'Model request finished';
  if (fact.operation) return operationLabels[fact.operation] || fact.operation;
  if (fact.kind === 'RequestReceived') return 'Request received';
  if (fact.kind === 'AuthorityResolved') return 'Access resolved';
  if (fact.kind === 'ProviderAttempt') return 'Model request started';
  if (fact.kind === 'ProviderOutcome') return 'Model request finished';
  return category(fact);
}
const recordReason = (fact: Observation) => fact.reason || fact.decision?.reason || '';

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
function Heading({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  return <div className="portal-head">
    <div><h1>{title}</h1>{subtitle && <p>{subtitle}</p>}</div>{action}
  </div>;
}
function Access({ value, runtime }: {
  value?: ClaimRequestedAccess | EffectiveAuthority | null; runtime?: string;
}) {
  if (!value) return <p>No authority was issued.</p>;
  return <div className="portal-access-fields">
    <Field label="Resource scope" value={<Chips values={value.resourceScopes}/>}/>
    <Field label="Tools" value={<Chips values={value.tools}/>}/>
    <Field label="Model profile" value={value.modelProfile || 'None'}/>
    <Field label="Memory scope" value={<Chips values={value.memoryScopes}/>}/>
    <Field label="Runtime" value={runtime}/>
  </div>;
}

// Polling stops after the final outcome, not merely the terminal claim phase:
// cleanup can still be in progress after authority is revoked.
function useConnection(parts: string[], revision: number) {
  const [setup, setSetup] = useState<Setup>();
  const [works, setWorks] = useState<View[]>([]);
  const [current, setCurrent] = useState<View>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [paused, setPaused] = useState(false);
  let ref = '';
  let routeError = '';
  try {
    // Consume the decoded array: production optimizers can remove an unused
    // decode call, including its validation side effect.
    const decodedParts = parts.map(part => decodeURIComponent(part));
    if (decodedParts[0] === 'work' && decodedParts[1] && decodedParts[1] !== 'new') ref = decodedParts[1];
  } catch {
    routeError = 'Invalid work reference.';
  }

  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;
    const started = Date.now();
    setLoading(true); setError(''); setCurrent(undefined); setPaused(false);
    if (routeError) {
      setLoading(false); setError(routeError);
      return () => controller.abort();
    }

    async function load() {
      try {
        const [nextSetup, list, detail] = await Promise.all([
          connectedSource.setup(controller.signal),
          connectedSource.list(controller.signal),
          ref ? connectedSource.request(ref, controller.signal) : Promise.resolve(undefined),
        ]);
        if (disposed) return;
        setSetup(nextSetup); setWorks(list); setCurrent(detail);
        setLoading(false); setError('');
        if (detail && isTerminal(detail) && detail.outcome) return;
        if (Date.now() - started >= 120_000) { setPaused(true); return; }
        timer = setTimeout(load, 1000);
      } catch (cause) {
        if (disposed) return;
        setLoading(false);
        setError(cause instanceof Error ? cause.message : 'The connection is unavailable.');
      }
    }
    void load();
    return () => {
      disposed = true; controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [ref, routeError, revision]);

  // Guard synchronously: hash navigation can render a new record route with
  // the previous work's state before the effect has reset it.
  return { setup, works, current, loading: routeError ? false : loading, error: routeError || error, paused };
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
        <Badge value={fact.result || fact.providerStatus || fact.decision?.result || 'Recorded'}/>
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
    `${workTitle(work)} ${work.state?.principal.team || ''}`.toLowerCase().includes(search.toLowerCase()));
  return <>
    <Heading title="Work" subtitle="Current-session requests and results."
      action={<a className="portal-button primary" href={link('work/new')}>New work</a>}/>
    <div className="portal-toolbar">
      <Filters values={['All', 'Running', 'Succeeded', 'Denied', 'Failed', 'Pending']}
        selected={filter} choose={setFilter}/>
      <input type="search" aria-label="Search work" placeholder="Search work" value={search}
        onChange={event => setSearch(event.target.value)}/>
    </div>
    <div className="portal-table-wrap"><table className="portal-table">
      <thead><tr><th>Task</th><th>Agent</th><th>Last update</th><th>Status</th></tr></thead>
      <tbody>{shown.map(work =>
        <tr key={work.requestRef}>
          <td><a className="portal-work-title" href={link(workLink(work))}>{workTitle(work)}</a>
            <small>{work.state?.principal.team || 'Identity not recorded'}</small></td>
          <td>{work.request.spec.templateRef}</td><td>{time(work.facts.at(-1)?.timestamp)}</td>
          <td><Badge value={workStatus(work)}/></td>
        </tr>
      )}</tbody>
    </table>{!shown.length && <p className="portal-empty">
      {works.length ? 'No work matches this filter.' : 'No work started in this session.'}
    </p>}</div>
  </>;
}
function WorkDetail({ work }: { work: View }) {
  const state = work.state;
  const runtime = state?.effectiveAuthority?.runtime;
  const status = workStatus(work);
  const ended = !!state?.claim && !['Pending', 'Bound', 'Running'].includes(state.claim.phase);
  const latestFacts = [...work.facts].reverse();
  const lastModel = latestFacts.find(fact => ['ProviderAttempt', 'ProviderOutcome'].includes(fact.kind) && fact.operation !== 'tool.invoke');
  const lastCleanup = latestFacts.find(fact => ['CleanupSucceeded', 'CleanupFailed'].includes(fact.operation || ''));
  const progress = work.facts.filter(fact =>
    !['WorkerActivity', 'ModelDecision', 'ProviderAttempt', 'ProviderOutcome', 'ToolDecision'].includes(fact.kind));
  const summary = status === 'Finishing' ? 'Waiting for the final result and cleanup evidence.'
    : work.outcome?.status === 'Succeeded' ? 'Task completed.'
    : work.outcome?.failure || state?.decision.reason || 'Request received; waiting for authorization.';
  return <>
    <nav className="portal-crumbs" aria-label="Breadcrumb">
      <a href={link('work')}>Work</a> / {workTitle(work)}
    </nav>
    <Heading title={workTitle(work)} subtitle={work.requestRef} action={<Badge value={status}/>}/>
    <div className="portal-top-facts">
      <Field label="Team" value={state?.principal.team}/>
      <Field label="Agent" value={work.request.spec.templateRef}/>
      <Field label="Requested by" value={state?.principal.subject}/>
    </div>
    <RunFlow activityHref={link(`${workLink(work)}/activity`)} evidence={{
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
    }}/>
    <WorkerActivity actions={workerActions(work.facts, status, link(`${workLink(work)}/activity`))} status={status} activityHref={link(`${workLink(work)}/activity`)}/>
    <div className="portal-detail-grid">
      <section>
        <div className="portal-section-head"><h2>Progress</h2>
          <small>{time(work.facts.at(-1)?.timestamp)} last update</small></div>
        <div className={`portal-state ${status.toLowerCase()}`} role="status">
          <strong>{status}</strong><p>{summary}</p>
        </div>
        {work.outcome?.failure && <div className="portal-state failed" role="alert">
          <strong>{work.outcome.status === 'Succeeded'
            ? 'Task completed; cleanup needs attention' : 'Execution needs attention'}</strong>
          <p>{work.outcome.failure}</p>
        </div>}
        <ol className="portal-timeline">{progress.map(fact =>
          <li key={fact.id}>
            <a href={link(`${workLink(work)}/activity/${encodeURIComponent(fact.id)}`)}>{recordTitle(fact)}</a>
            {recordReason(fact) && <p>{recordReason(fact)}</p>}
          </li>
        )}</ol>
      </section>
      <aside className="portal-access-card">
        <h2>{state?.effectiveAuthority
          ? state.claim?.phase === 'Running' ? 'Active access' : 'Issued access' : 'No access issued'}</h2>
        <p>{ended ? 'Authority is inactive after this claim ended.' : state?.decision.reason}</p>
        <Access value={state?.effectiveAuthority}
          runtime={runtime ? `${runtime.profileRef} · ${runtime.timeout}` : undefined}/>
        <div className="portal-card-links">
          <a href={link(`${workLink(work)}/access`)}>Compare requested and granted</a>
          <a href={link('policy')}>View policy</a>
        </div>
      </aside>
    </div>
    {work.outcome?.text && <section className="portal-lower portal-result">
      <h2>Result</h2><pre>{work.outcome.text}</pre>
      {work.outcome.model && <p className="portal-source-note">
        Final model call: {work.outcome.model.model} · {work.outcome.model.inputTokens} input /
        {' '}{work.outcome.model.outputTokens} output tokens
      </p>}
    </section>}
    <section className="portal-lower">
      <div className="portal-section-head"><h2>Recent activity</h2>
        <a href={link(`${workLink(work)}/activity`)}>View all activity</a></div>
      <Records work={work} observations={work.facts.slice(-4)}/>
    </section>
    <details className="portal-details"><summary>Execution details</summary>
      <div className="portal-fact-grid">
        <Field label="Request ID" value={work.requestRef}/>
        <Field label="Claim ID" value={state?.claim?.id}/>
        <Field label="Runtime backend" value={state?.claim?.backendIdentity?.backend}/>
        <Field label="Worker ID" value={state?.claim?.backendIdentity?.workerId}/>
        <Field label="Policy version" value={state ? `${state.policyRef.id} / ${state.policyRef.version}` : undefined}/>
      </div>
    </details>
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
          runtime={runtime ? `${runtime.profileRef} · ${runtime.timeout}` : undefined}/>
      </section>
      <section className="portal-compare-side"><h2>Granted</h2>
        <Access value={effective}
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
        action={<Badge value={fact.result || fact.providerStatus || fact.decision?.result || 'Recorded'}/>}/>
      <section className="portal-panel"><h2>What this record says</h2>
        <p>{recordReason(fact) || recordTitle(fact)}</p>
        <div className="portal-fact-grid">
          <Field label="Kind" value={category(fact)}/><Field label="Target" value={fact.target}/>
          <Field label="Permission" value={fact.result || fact.decision?.result}/>
          <Field label="Provider outcome" value={fact.providerStatus}/>
          <Field label="Claim" value={fact.claimId}/><Field label="Invocation" value={fact.invocationId}/>
          <Field label="Policy" value={policy ? `${policy.id} / ${policy.version}` : undefined}/>
        </div>
        {fact.effectiveAuthority && <><h3>Effective authority at resolution</h3>
          <Access value={fact.effectiveAuthority}
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
          <Badge value={fact.result || fact.providerStatus || fact.decision?.result || 'Recorded'}/>
        </div>
      )}{!records.length && <p className="portal-empty">No governance activity recorded yet.</p>}</div>
    </>;
  }
  const observed = (kinds: string[], key: string) => works.flatMap(work => work.facts)
    .some(fact => kinds.includes(fact.kind) && (key !== 'model' || category(fact) === 'Model Gateway'));
  const capabilities: [string, string, string[]][] = [
    ['Task submission', 'taskSubmission', ['RequestReceived']],
    ['Runtime', 'runtime', ['Runtime']],
    ['Model Gateway', 'model', ['ModelDecision', 'ProviderAttempt', 'ProviderOutcome']],
    ['Tool Gateway', 'tool', ['ToolDecision']],
    ['Memory Interface', 'memory', ['Memory']],
  ];
  return <>
    <Heading title="Platform" subtitle="Configured capabilities and recorded use."/>
    <div className="portal-table-wrap"><table className="portal-table portal-platform-table">
      <thead><tr><th>Capability</th><th>Configuration</th><th>Recorded use</th><th>Open</th></tr></thead>
      <tbody>{capabilities.map(([name, key, kinds]) => <tr key={name}>
        <td>{name}</td>
        <td><Badge value={setup.capabilities[key] === 'notConnected'
          ? 'Not connected' : setup.capabilities[key] || 'Not connected'}/></td>
        <td>{observed(kinds, key) ? 'Activity recorded' : 'No activity recorded'}</td>
        <td><a href={link('platform/activity')}>View activity</a></td>
      </tr>)}</tbody>
    </table></div>
    <p className="portal-source-note">Configuration is not a health check. Activity records show permissions
      and outcomes, not service availability.</p>
  </>;
}
function NewWork({ setup, created }: { setup: Setup; created: (work: View) => void }) {
  const ceiling = setup.template.spec.capabilityCeiling;
  const defaults = setup.template.spec.defaults;
  const [tools, setTools] = useState<string[]>(ceiling?.tools || []);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const objective = String(form.get('objective') || '').trim();
    if (!objective) { setError('Describe the task.'); return; }
    const repository = String(form.get('repository') || '').trim();
    const scopes = (name: string) => String(form.get(name) || '')
      .split(',').map(value => value.trim()).filter(Boolean);
    const request: ClaimRequest = {
      apiVersion: 'agenova.io/v1alpha1', kind: 'ClaimRequest',
      metadata: { name: `work-${crypto.randomUUID()}` },
      spec: {
        templateRef: setup.template.metadata.name,
        projectRef: 'payments',
        task: {
          type: 'repository-change',
          input: {
            objective,
            ...(repository ? { repository, baseBranch: String(form.get('branch') || 'main') } : {}),
          },
        },
        requestedAccess: {
          tools, resourceScopes: scopes('scopes'),
          modelProfile: String(form.get('model') || ''), memoryScopes: scopes('memory'),
        },
        runtime: {
          profileRef: String(form.get('runtime')),
          timeout: `${Number(form.get('timeout'))}m`,
        },
      },
    };
    setBusy(true); setError('');
    try { created(await connectedSource.submit(request)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'The request could not be started.'); }
    finally { setBusy(false); }
  }
  return <>
    <nav className="portal-crumbs"><a href={link('work')}>Work</a> / New work</nav>
    <Heading title="New work"/>
    <form className="portal-form" onSubmit={event => void submit(event)}>
      <div>
        <section>
          <h2>Task</h2>
          <label>Agent<select name="agent" defaultValue={setup.template.metadata.name}>
            <option>{setup.template.metadata.name}</option>
          </select></label>
          <label>What should it do?<textarea name="objective" required/></label>
          <div className="portal-field-row">
            <label>Repository<input name="repository" defaultValue="acme/payments"/></label>
            <label>Base branch<input name="branch" defaultValue="main"/></label>
          </div>
        </section>
        <section>
          <h2>Requested access</h2>
          <fieldset><legend>Tools</legend>{(ceiling?.tools || []).map(tool =>
            <label className="portal-check" key={tool}>
              <input type="checkbox" checked={tools.includes(tool)} onChange={event =>
                setTools(current => event.target.checked
                  ? [...current, tool] : current.filter(value => value !== tool))}/>{tool}
            </label>
          )}</fieldset>
          <label>Resource scopes<input name="scopes" defaultValue="repo:acme/payments"/></label>
          <div className="portal-field-row">
            <label>Model profile<select name="model"
              defaultValue={defaults?.modelProfile || ceiling?.modelProfiles?.[0] || ''}>
              {(ceiling?.modelProfiles || []).map(profile => <option key={profile}>{profile}</option>)}
            </select></label>
            <label>Memory scope<input name="memory" defaultValue={(defaults?.memoryScopes || []).join(', ')}/></label>
          </div>
        </section>
        <section>
          <h2>Runtime request</h2>
          <div className="portal-field-row">
            <label>Runtime profile<select name="runtime">
              {(ceiling?.runtimeProfiles || []).map(profile => <option key={profile}>{profile}</option>)}
            </select></label>
            <label>Time limit (minutes)<input name="timeout" type="number" min="1" max="120"
              defaultValue="20" required/></label>
          </div>
        </section>
        {error && <p role="alert" className="portal-form-error">{error}</p>}
        <div className="portal-form-actions">
          <button type="submit" className="portal-button primary" disabled={busy}>
            {busy ? 'Starting…' : 'Start work'}
          </button>
          <a href={link('work')}>Cancel</a>
        </div>
      </div>
    </form>
  </>;
}
function Templates({ setup }: { setup: Setup }) {
  return <><Heading title="Agents"/>
    <section className="portal-panel">
      <h2>{setup.template.metadata.name}</h2>
      <a className="portal-button" href={link('work/new')}>Start work</a>
      <h3>Template defaults and limits</h3>
      <Field label="Model profiles" value={<Chips values={setup.template.spec.capabilityCeiling?.modelProfiles}/>}/>
      <Field label="Runtime profiles" value={<Chips values={setup.template.spec.capabilityCeiling?.runtimeProfiles}/>}/>
      <Field label="Tool ceiling" value={<Chips values={setup.template.spec.capabilityCeiling?.tools}/>}/>
      <details className="portal-details"><summary>Show template</summary>
        <pre>{JSON.stringify(setup.template, null, 2)}</pre>
      </details>
    </section>
  </>;
}
function Policy({ setup }: { setup: Setup }) {
  return <><Heading title="Policy"/>
    <section className="portal-panel">
      <h2>Active reference policy</h2>
      <div className="portal-fact-grid">
        <Field label="Policy ID" value={setup.policy.ID}/>
        <Field label="Version" value={setup.policy.Version}/>
        <Field label="Default decision" value="Deny"/>
      </div>
      <div className="portal-table-wrap"><table className="portal-table">
        <thead><tr><th>Team</th><th>Action</th><th>Project</th><th>Template</th></tr></thead>
        <tbody>{setup.policy.Rules.map((rule, index) => <tr key={index}>
          <td>{rule.Team}</td><td>{rule.Action}</td><td>{rule.Project}</td><td>{rule.TemplateRef}</td>
        </tr>)}</tbody>
      </table></div>
    </section>
  </>;
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
export function ConnectedPortal({ parts, onDemo }: { parts: string[]; onDemo: () => void }) {
  const [revision, setRevision] = useState(0);
  const data = useConnection(parts, revision);
  const retry = () => setRevision(value => value + 1);
  const section = parts[0] || 'work';
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
  if (section === 'work' && parts[1] === 'new') {
    content = <NewWork setup={setup} created={work => {
      window.location.hash = link(workLink(work)); retry();
    }}/>;
  } else if (section === 'work' && !parts[1]) {
    content = <WorkList works={data.works}/>;
  } else if (section === 'work' && data.current) {
    content = parts[2] === 'access' ? <WorkAccess work={data.current}/>
      : parts[2] === 'activity' ? <Activity work={data.current} selected={parts[3]}/>
      : <WorkDetail work={data.current}/>;
  } else if (section === 'platform') {
    content = <Platform setup={setup} works={data.works} activity={parts[1] === 'activity'}/>;
  } else if (section === 'agents') content = <Templates setup={setup}/>;
  else if (section === 'policy') content = <Policy setup={setup}/>;
  else if (section === 'identity') content = <Identity setup={setup}/>;
  else content = <Heading title="Work not found"/>;
  return <>
    {data.paused && <p role="status" className="portal-source-note">
      Automatic refresh paused. <button className="portal-button" onClick={retry}>Refresh progress</button>
    </p>}
    {content}
  </>;
}
