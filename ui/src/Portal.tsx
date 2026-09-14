// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { agents, demoIdentity, demoPolicy, exampleWorks, type AccessSet, type AgentSummary, type EventKind, type WorkEvent, type WorkItem, type WorkStatus } from './portal-data';
import './portal.css';

const workFilters = ['All', 'Running', 'Succeeded', 'Denied', 'Failed', 'Pending'] as const;
const workActivityKinds = ['All', 'Request', 'Decision', 'Claim', 'Runtime', 'Tool', 'Model'] as const;
const platformActivityKinds = ['All', 'Decision', 'Tool', 'Model', 'Runtime'] as const;
const kindLabel = (kind: string) => kind === 'Decision' ? 'Request resolution' : kind;
const href = (path: string) => `#/${path}`;
function route() {
  const raw = window.location.hash.replace(/^#\/?/, '');
  const [path, query = ''] = raw.split('?');
  return { parts: path.split('/').filter(Boolean), query: new URLSearchParams(query) };
}
function Badge({ value }: { value: string }) { return <span className={`portal-badge ${value.toLowerCase().replace(/\s+/g, '-')}`}>{value}</span>; }
function Chips({ values }: { values?: string[] }) { return values?.length ? <div className="portal-chips">{values.map(value => <code key={value}>{value}</code>)}</div> : <span className="portal-muted">None</span>; }
function Crumbs({ items }: { items: { label: string; link?: string }[] }) {
  return <nav className="portal-crumbs" aria-label="Breadcrumb">{items.map((item, index) => <span key={index}>{index > 0 && <span aria-hidden="true"> / </span>}{item.link ? <a href={href(item.link)}>{item.label}</a> : item.label}</span>)}</nav>;
}
function Head({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  return <div className="portal-head"><div><h1>{title}</h1>{subtitle && <p>{subtitle}</p>}</div>{action}</div>;
}
function Fact({ label, value }: { label: string; value?: ReactNode }) { return <div className="portal-fact"><span>{label}</span><strong>{value ?? 'Not available'}</strong></div>; }
function Records({ work, events }: { work: WorkItem; events: WorkEvent[] }) {
  return <div className="portal-record-list">{events.length ? events.map(event => <div className="portal-record-row" key={event.id}>
    <time>{event.time}</time><div><a href={href(`work/${work.id}/activity/${event.id}`)}>{event.title}</a><small>{event.target}</small></div>
    <span className="portal-record-source">{event.source}</span><Badge value={event.result}/>
  </div>) : <p className="portal-empty">No records in this category.</p>}</div>;
}
function AccessFields({ access, runtime }: { access?: AccessSet; runtime?: string }) {
  if (!access) return <p>No authority was issued.</p>;
  return <div className="portal-access-fields"><Fact label="Resource scope" value={<Chips values={access.resourceScopes}/>}/><Fact label="Tools" value={<Chips values={access.tools}/>}/>
    <Fact label="Model profile" value={access.model || 'None'}/><Fact label="Memory scope" value={<Chips values={access.memory}/>}/><Fact label="Runtime" value={runtime}/></div>;
}
function WorkList({ works }: { works: WorkItem[] }) {
  const [filter, setFilter] = useState<(typeof workFilters)[number]>('All');
  const [search, setSearch] = useState('');
  const shown = works.filter(work => (filter === 'All' || work.status === filter) &&
    `${work.title} ${work.team} ${work.agent} ${work.context?.value ?? ''}`.toLowerCase().includes(search.toLowerCase()));
  return <><Head title="Work" subtitle="Requests and agent runs you can inspect." action={<a className="portal-button primary" href={href('work/new')}>New work</a>}/>
    <div className="portal-toolbar"><div className="portal-filters" role="group" aria-label="Filter work">{workFilters.map(item => <button key={item} type="button" className={filter === item ? 'selected' : ''} onClick={() => setFilter(item)}>{item}</button>)}</div>
      <input aria-label="Search work" placeholder="Search work" type="search" value={search} onChange={event => setSearch(event.target.value)}/></div>
    <div className="portal-table-wrap"><table className="portal-table"><thead><tr><th>Task</th><th>Agent</th><th>Last update</th><th>Status</th></tr></thead><tbody>
      {shown.map(work => <tr key={work.id}><td><a className="portal-work-title" href={href(`work/${work.id}`)}>{work.title}</a><small>{work.team}{work.context && <> · {work.context.label}: {work.context.value}</>}</small></td>
        <td>{work.agent}</td><td>{work.updated}</td><td><Badge value={work.status}/></td></tr>)}
    </tbody></table>{!shown.length && <p className="portal-empty">No work matches this filter.</p>}</div>
    <p className="portal-source-note">Illustrative work history. The live evidence list and persistence are not connected.</p></>;
}
function WorkDetail({ work }: { work: WorkItem }) {
  const message = work.status === 'Running' ? ['Worker running', 'No outcome has been recorded yet.']
    : work.status === 'Succeeded' ? ['Work completed', work.outcome]
    : work.status === 'Failed' ? ['Work failed', work.outcome]
    : work.status === 'Denied' ? ['Request denied', 'No claim was issued and no worker started.']
    : ['Request received', 'No authorization or worker allocation has been recorded.'];
  return <><Crumbs items={[{ label: 'Work', link: 'work' }, { label: work.title }]}/>
    <Head title={work.title} subtitle={`Submitted ${work.submitted} · ${work.requestRef}`} action={<Badge value={work.status}/>}/>
    <div className="portal-top-facts">{work.context && <Fact label={work.context.label} value={work.context.value}/>}<Fact label={work.branch ? 'Base branch' : 'Team'} value={work.branch || work.team}/><Fact label="Agent" value={work.agent}/><Fact label="Requested by" value={work.principal}/></div>
    <div className="portal-detail-grid"><section><div className="portal-section-head"><h2>Progress</h2><small>{work.updated} last update</small></div>
      <div className={`portal-state ${work.status.toLowerCase()}`} role="status"><strong>{message[0]}</strong><p>{message[1]}</p></div>
      <ol className="portal-timeline">{work.events.filter(event => ['Request', 'Decision', 'Claim', 'Runtime'].includes(event.kind)).map(event =>
        <li key={event.id}><a href={href(`work/${work.id}/activity/${event.id}`)}>{event.title}</a><p>{event.description}</p></li>)}</ol>
    </section><aside className="portal-access-card"><h2>{work.granted ? work.status === 'Running' ? 'Active access' : 'Issued access' : work.status === 'Denied' ? 'Access denied' : 'Request pending'}</h2>
      <p>{work.granted ? work.status === 'Running' ? 'Effective authority for this running claim.' : 'Authority is inactive after this claim ended.' : 'No authority has been issued.'}</p>
      {work.granted ? <AccessFields access={work.granted} runtime={work.effectiveRuntime || work.requestedRuntime}/> : <><Fact label="Policy decision" value={work.status === 'Pending' ? 'Not evaluated' : work.decisionReason}/><Fact label="Requested tools" value={<Chips values={work.requested.tools}/>}/></>}
      <div className="portal-card-links"><a href={href(`work/${work.id}/access`)}>{work.granted ? 'Compare requested and granted' : 'View full request'}</a><a href={href('policy')}>View policy</a></div>
    </aside></div>
    <section className="portal-lower"><div className="portal-section-head"><h2>Recent activity</h2><a href={href(`work/${work.id}/activity`)}>View all activity</a></div><Records work={work} events={work.events.slice(-4)}/></section>
    <details className="portal-details"><summary>Execution details</summary><p>IDs linking the request, claim, and worker.</p><div className="portal-fact-grid"><Fact label="Request ID" value={work.requestRef}/><Fact label="Claim ID" value={work.claimId}/><Fact label="Runtime backend" value={work.backend}/><Fact label="Worker ID" value={work.worker}/><Fact label="Policy version" value={work.policy}/><Fact label="Decision" value={work.decision}/></div></details></>;
}
function FilterButtons<T extends string>({ values, selected, onSelect, label }: { values: readonly T[]; selected: T; onSelect: (value: T) => void; label: string }) {
  return <div className="portal-filters" role="group" aria-label={label}>{values.map(value => <button key={value} type="button" className={selected === value ? 'selected' : ''} onClick={() => onSelect(value)}>{kindLabel(value)}</button>)}</div>;
}
function WorkActivity({ work, initial }: { work: WorkItem; initial: string }) {
  const [filter, setFilter] = useState<(typeof workActivityKinds)[number]>(workActivityKinds.includes(initial as (typeof workActivityKinds)[number]) ? initial as (typeof workActivityKinds)[number] : 'All');
  useEffect(() => setFilter(workActivityKinds.includes(initial as (typeof workActivityKinds)[number]) ? initial as (typeof workActivityKinds)[number] : 'All'), [initial]);
  return <><Crumbs items={[{ label: 'Work', link: 'work' }, { label: work.title, link: `work/${work.id}` }, { label: 'Activity' }]}/>
    <Head title="Activity" subtitle={`${work.title}${work.context ? ` · ${work.context.label}: ${work.context.value}` : ''}`} action={<Badge value={work.status}/>}/>
    <FilterButtons values={workActivityKinds} selected={filter} onSelect={setFilter} label="Filter work activity"/>
    <Records work={work} events={filter === 'All' ? work.events : work.events.filter(event => event.kind === filter)}/>
    <p className="portal-source-note">Only records for this work appear here. Open a row for its details.</p></>;
}
function EventDetail({ work, event }: { work: WorkItem; event: WorkEvent }) {
  const hasClaim = !['Request', 'Decision'].includes(event.kind) && !!work.claimId;
  return <><Crumbs items={[{ label: 'Work', link: 'work' }, { label: work.title, link: `work/${work.id}` }, { label: 'Activity', link: `work/${work.id}/activity` }, { label: event.title }]}/>
    <Head title={event.title} subtitle={`${work.title} · ${event.time}`} action={<Badge value={event.result}/>}/>
    <section className="portal-panel"><h2>What this record says</h2><p>{event.description}</p><div className="portal-fact-grid">
      <Fact label="Source" value={event.source}/><Fact label="Action" value={event.action || kindLabel(event.kind)}/><Fact label="Target" value={event.target}/><Fact label="Result" value={event.result}/>
      <Fact label="Policy" value={event.policy || 'Not applicable'}/><Fact label="Claim" value={hasClaim ? work.claimId : 'Not issued yet'}/>{event.externalCall === false && <Fact label="External call" value="Not made"/>}
    </div>{event.effective && <div className="portal-record-effective"><h3>Effective authority at resolution</h3><AccessFields access={event.effective} runtime={event.effective.runtime}/><a href={href(`work/${work.id}/access`)}>Compare request, template, and grant</a></div>}
      <details className="portal-details"><summary>Show illustrative record JSON</summary><pre>{JSON.stringify({ ...event, requestRef: work.requestRef, claimId: hasClaim ? work.claimId : null }, null, 2)}</pre></details></section>
    <p><a className="portal-button" href={href(`work/${work.id}/activity`)}>Back to activity</a></p></>;
}
function AccessSide({ title, access, runtime }: { title: string; access?: AccessSet; runtime?: string }) {
  return <section className="portal-compare-side"><h2>{title}</h2><AccessFields access={access} runtime={runtime}/></section>;
}
function WorkAccess({ work }: { work: WorkItem }) {
  return <><Crumbs items={[{ label: 'Work', link: 'work' }, { label: work.title, link: `work/${work.id}` }, { label: 'Access' }]}/>
    <Head title="Access for this work" subtitle={work.title} action={<Badge value={work.status}/>}/>
    <div className="portal-state"><strong>{work.granted ? 'Assignment allowed; authority resolved.' : work.status === 'Pending' ? 'Authorization pending.' : 'No access was granted.'}</strong>
      <p>{work.granted ? 'The request does not grant access by itself; limits determine the issued authority.' : work.status === 'Pending' ? 'No policy decision or claim has been recorded.' : 'The request was denied before claim creation.'}</p></div>
    {work.ceiling && <div className="portal-resolution"><strong>Why the grant is narrower</strong><p><code>shell.exec</code> is outside the Engineer template ceiling. The requested 45 minutes is capped at 30 minutes.</p></div>}
    <div className="portal-comparison"><AccessSide title="Requested" access={work.requested} runtime={work.requestedRuntime}/>{work.ceiling && <AccessSide title="Engineer template ceiling" access={work.ceiling} runtime={work.ceiling.runtime}/>}<AccessSide title="Granted" access={work.granted} runtime={work.granted ? work.effectiveRuntime || work.requestedRuntime : undefined}/></div>
    <details className="portal-details"><summary>Policy decision</summary><div className="portal-fact-grid"><Fact label="Decision" value={work.decision}/><Fact label="Policy" value={work.policy}/><Fact label="Reason" value={work.decisionReason}/><Fact label="Principal" value={work.principal}/></div></details></>;
}
function AgentsPage() { return <><Head title="Agents" subtitle="Reusable roles available for work requests."/><div className="portal-agent-list">{agents.map(agent => <div key={agent.id} className="portal-agent-row"><div><h2><a href={href(`agents/${agent.id}`)}>{agent.name}</a></h2><p>{agent.purpose}</p></div><a className="portal-button" href={href(`work/new?agent=${agent.id}`)}>Start work</a></div>)}</div></>; }
function AgentPage({ agent }: { agent: AgentSummary }) { return <><Crumbs items={[{ label: 'Agents', link: 'agents' }, { label: agent.name }]}/><Head title={`${agent.name} agent`} subtitle={agent.purpose} action={<a className="portal-button primary" href={href(`work/new?agent=${agent.id}`)}>Start work</a>}/>
  <section className="portal-panel"><h2>Template defaults and limits</h2><div className="portal-fact-grid"><Fact label="Model profile · Model Gateway" value={agent.model}/><Fact label="Memory scope · future interface" value={agent.memory}/><Fact label="Runtime profile · RuntimeBackend" value={agent.runtime}/><Fact label="Tool ceiling · Tool Gateway" value={<Chips values={agent.tools}/>}/></div><p className="portal-source-note">These template settings do not grant every request this access.</p></section></>; }
function PolicyPage() { return <><Head title="Policy"/><section className="portal-panel"><h2>Reference policy example</h2><div className="portal-fact-grid"><Fact label="Policy ID" value={demoPolicy.id}/><Fact label="Version" value={demoPolicy.version}/><Fact label="Default decision" value="Deny"/><Fact label="Source" value="Illustrative portal data"/></div></section>
  <section className="portal-lower"><div className="portal-section-head"><h2>Assignment rules</h2><a href={href('work/review-checkout-change')}>View denied request</a></div><div className="portal-table-wrap"><table className="portal-table"><thead><tr><th>Team</th><th>Can start</th><th>Decision</th></tr></thead><tbody>{demoPolicy.rules.map((rule, index) => <tr key={index}><td>{rule.team}</td><td>{rule.scope}<small>claim.create</small></td><td><Badge value={rule.result}/></td></tr>)}<tr><td>All others</td><td>Unmatched requests</td><td><Badge value="Denied"/></td></tr></tbody></table></div></section></>; }
function PlatformPage() {
  const capabilities = [
    ['Agent templates', 'Engineer, Reviewer, Researcher', 'Reusable roles with defaults and ceilings', 'Example', 'agents', 'View agents'],
    ['Policy', `${demoPolicy.id} / v${demoPolicy.version}`, 'Default-deny assignment policy', 'Example', 'policy', 'View policy'],
    ['Tool Gateway', 'Git, GitHub, Web', 'Claim-scoped tool access', 'Example', 'platform/activity?type=Tool', 'View tool activity'],
    ['Model Gateway', 'coding-standard, research-standard', 'Logical model profiles', 'Example', 'platform/activity?type=Model', 'View model activity'],
    ['Memory Interface', 'team-docs in example requests', 'No memory calls connected', 'Planned', '', ''],
    ['RuntimeBackend', 'reference', 'Backend identity recorded per claim', 'Example', 'platform/activity?type=Runtime', 'View runtime activity'],
    ['Evidence', 'Request resolution, tool, model, runtime', 'Cross-work governance activity', 'Example', 'platform/activity', 'View activity'],
  ];
  return <><Head title="Platform" subtitle="Service bindings used by these example work requests."/>
    <div className="portal-table-wrap"><table className="portal-table portal-platform-table"><thead><tr><th>Capability</th><th>Binding in this example</th><th>Demo status</th><th>Open</th></tr></thead><tbody>{capabilities.map(([name, binding, description, status, link, action]) => <tr key={name}><td><strong>{name}</strong></td><td>{binding}<small>{description}</small></td><td><Badge value={status}/></td><td>{link ? <a href={href(link)}>{action}</a> : 'Not connected'}</td></tr>)}</tbody></table></div>
    <section className="portal-implementation"><h2>What is connected today?</h2><div className="portal-implementation-grid"><div><Badge value="Done"/><h3>Contract fixture reader</h3><p>Canonical Go-parsed request and issued-state fixtures are validated and displayed.</p><a href="/fixtures">Open contract cases</a></div><div><Badge value="Fixture"/><h3>Single-claim console</h3><p>Read-only request, authority, and evidence display using validated fixtures.</p><a href="/console/requests/fix-payment-timeout">Open fixture console</a></div><div><Badge value="Mock"/><h3>Portal journey</h3><p>Work history, submission, gateway records, policy and platform activity are illustrative here.</p></div><div><Badge value="Todo"/><h3>Live connection</h3><p>Evidence API, bounded polling and live gateway records are not connected to this portal.</p></div></div></section></>;
}
function PlatformActivity({ works, initial }: { works: WorkItem[]; initial: string }) {
  const [filter, setFilter] = useState<(typeof platformActivityKinds)[number]>(platformActivityKinds.includes(initial as (typeof platformActivityKinds)[number]) ? initial as (typeof platformActivityKinds)[number] : 'All');
  useEffect(() => setFilter(platformActivityKinds.includes(initial as (typeof platformActivityKinds)[number]) ? initial as (typeof platformActivityKinds)[number] : 'All'), [initial]);
  const records = works.flatMap(work => work.events.filter(event => ['Decision', 'Tool', 'Model', 'Runtime'].includes(event.kind)).map(event => ({ work, event })))
    .filter(({ event }) => filter === 'All' || event.kind === filter)
    .sort((a, b) => `${b.work.submitted.slice(0, 11)} ${b.event.time}`.localeCompare(`${a.work.submitted.slice(0, 11)} ${a.event.time}`));
  return <><Crumbs items={[{ label: 'Platform', link: 'platform' }, { label: 'Activity' }]}/><Head title="Governance activity" subtitle={`${records.length} example records across ${new Set(records.map(({ work }) => work.id)).size} work requests · ${records.filter(({ event }) => event.result === 'Denied').length} denied`}/>
    <FilterButtons values={platformActivityKinds} selected={filter} onSelect={setFilter} label="Filter platform activity"/>
    <div className="portal-platform-records">{records.length ? records.map(({ work, event }) => <div className="portal-platform-row" key={`${work.id}-${event.id}`}><time>{work.submitted.slice(0, 11)}<br/>{event.time}</time>
      <div><small>{kindLabel(event.kind)}{event.action && event.kind !== 'Decision' ? ` · ${event.action}` : ''}</small><a href={href(`work/${work.id}/activity/${event.id}`)}>{event.title}</a><p>{event.description}</p></div>
      <div><a href={href(`work/${work.id}`)}>{work.title}</a><small>{work.team}</small></div><Badge value={event.result}/></div>) : <p className="portal-empty">No records for this filter.</p>}</div>
    <p className="portal-source-note">Illustrative cross-work records, not observed traffic or gateway health metrics.</p></>;
}
function IdentityPage() { return <><Head title="Demo identity" subtitle="Fixed requester for new example work."/><section className="portal-panel"><h2>{demoIdentity.displayName}</h2><div className="portal-fact-grid"><Fact label="Subject" value={demoIdentity.subject}/><Fact label="Team" value={demoIdentity.team}/><Fact label="Authentication context" value={demoIdentity.authenticationContext}/></div></section></>; }
function NewWork({ addWork, selectedAgent }: { addWork: (work: WorkItem) => void; selectedAgent: string }) {
  const [agentId, setAgentId] = useState(agents.some(agent => agent.id === selectedAgent) ? selectedAgent : 'engineer');
  useEffect(() => setAgentId(agents.some(agent => agent.id === selectedAgent) ? selectedAgent : 'engineer'), [selectedAgent]);
  const agent = agents.find(item => item.id === agentId) || agents[0];
  const repositoryTask = agentId !== 'researcher';
  const [tools, setTools] = useState(agent.tools);
  const [error, setError] = useState('');
  useEffect(() => setTools(agent.tools), [agent]);
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const values = new FormData(event.currentTarget);
    const objective = String(values.get('objective') || '').trim();
    const contextValue = String(values.get(repositoryTask ? 'repository' : 'topic') || '').trim();
    const branch = String(values.get('branch') || '').trim();
    if (objective.length < 8) { setError('Enter a task objective of at least 8 characters.'); return; }
    if (repositoryTask && !/^[\w.-]+\/[\w.-]+$/.test(contextValue)) { setError('Use a repository in owner/name form, such as acme/payments.'); return; }
    if (!repositoryTask && contextValue.length < 3) { setError('Enter a topic of at least 3 characters.'); return; }
    if (repositoryTask && !branch) { setError('Enter a base branch.'); return; }
    const id = `mock-${Date.now().toString(36)}`;
    const time = new Date().toLocaleTimeString('en-AU', { hour: '2-digit', minute: '2-digit', hour12: false });
    const runtime = String(values.get('runtime') || agent.runtime);
    const timeout = Number(values.get('timeout'));
    const model = String(values.get('model') || '');
    const memory = String(values.get('memory') || '');
    const work: WorkItem = {
      id, title: objective, status: 'Pending', agent: agentId, team: 'Team A', principal: demoIdentity.subject,
      context: { label: repositoryTask ? 'Repository' : 'Topic', value: contextValue }, branch: repositoryTask ? branch : undefined, submitted: 'Just now', updated: time, requestRef: id,
      decision: 'Pending', decisionReason: 'Authorization has not been evaluated.', requestedRuntime: `${runtime} · ${timeout} min`,
      requested: { resourceScopes: repositoryTask ? [`repo:${contextValue}`] : [], tools, model, memory: memory ? [memory] : [] },
      events: [{ id: `${id}-request`, kind: 'Request', title: 'Work requested', time: `${time}:00`, result: 'Recorded', source: 'Demo UI', target: contextValue, description: 'An example request was added in this browser. No authorization or worker allocation occurred.' }],
    };
    addWork(work);
    window.location.hash = href(`work/${id}`);
  }
  return <><Crumbs items={[{ label: 'Work', link: 'work' }, { label: 'New work' }]}/><Head title="New work"/>
    <form onSubmit={submit} className="portal-form"><div><section><h2>Task</h2><label>Agent<select name="agent" value={agentId} onChange={event => setAgentId(event.target.value)}>{agents.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <label>What should it do?<textarea name="objective" placeholder="For example: Fix the payment timeout bug"/><small>This becomes the task objective, not an access grant.</small></label>
      {repositoryTask ? <div className="portal-field-row"><label>Repository<input name="repository" placeholder="acme/payments"/></label><label>Base branch<input name="branch" defaultValue="main"/></label></div>
        : <label>Topic<input name="topic" placeholder="Customer authentication"/></label>}</section>
      <section><h2>Requested access</h2><fieldset><legend>Tools</legend>{agent.tools.map(tool => <label className="portal-check" key={tool}><input type="checkbox" checked={tools.includes(tool)} onChange={event => setTools(current => event.target.checked ? [...current, tool] : current.filter(item => item !== tool))}/>{tool}</label>)}</fieldset>
      <div className="portal-field-row"><label>Model profile<input name="model" key={`${agent.id}-model`} defaultValue={agent.model}/></label><label>Memory scope<input name="memory" key={`${agent.id}-memory`} defaultValue={agent.memory}/></label></div></section>
      <section><h2>Runtime request</h2><div className="portal-field-row"><label>Runtime profile<select name="runtime" key={`${agent.id}-runtime`} defaultValue={agent.runtime}>{agent.runtimeProfiles.map(profile => <option key={profile}>{profile}</option>)}</select></label><label>Time limit (minutes)<input name="timeout" type="number" min="1" max="120" defaultValue="20"/></label></div></section>
      {error && <p role="alert" className="portal-form-error">{error}</p>}<div className="portal-form-actions"><button className="portal-button primary" type="submit">Create example request</button><a href={href('work')}>Cancel</a></div></div>
      <aside className="portal-request-route"><h2>Services behind this request</h2><Fact label="Agent template" value={agent.name}/><Fact label="Tool Gateway" value={`${tools.length} tools requested`}/><Fact label="Model Gateway" value={agent.model}/><Fact label="Memory Interface · future" value={agent.memory}/><Fact label="RuntimeBackend" value={agent.runtime}/><Fact label="Policy check" value={`${demoPolicy.id} / v${demoPolicy.version}`}/><p>A request does not grant access; policy and template limits determine what is issued.</p><a href={href('platform')}>View platform setup</a></aside></form></>;
}

export function Portal() {
  const [location, setLocation] = useState(() => window.location.hash);
  const [works, setWorks] = useState<WorkItem[]>(() => exampleWorks.map(work => structuredClone(work)));
  useEffect(() => { const changed = () => setLocation(window.location.hash); window.addEventListener('hashchange', changed); return () => window.removeEventListener('hashchange', changed); }, []);
  const { parts, query } = route();
  const section = parts[0] || 'work';
  const work = works.find(item => item.id === parts[1]);
  const agent = agents.find(item => item.id === parts[1]);
  let content: ReactNode;
  if (section === 'work' && parts[1] === 'new') content = <NewWork addWork={item => setWorks(current => [item, ...current])} selectedAgent={query.get('agent') || ''}/>;
  else if (section === 'work' && !parts[1]) content = <WorkList works={works}/>;
  else if (section === 'work' && work && parts[2] === 'access') content = <WorkAccess work={work}/>;
  else if (section === 'work' && work && parts[2] === 'activity' && parts[3]) {
    const event = work.events.find(item => item.id === parts[3]);
    content = event ? <EventDetail work={work} event={event}/> : <Head title="Record not found"/>;
  } else if (section === 'work' && work && parts[2] === 'activity') content = <WorkActivity work={work} initial={query.get('type') || 'All'}/>;
  else if (section === 'work' && work) content = <WorkDetail work={work}/>;
  else if (section === 'agents' && agent) content = <AgentPage agent={agent}/>;
  else if (section === 'agents') content = <AgentsPage/>;
  else if (section === 'policy') content = <PolicyPage/>;
  else if (section === 'platform' && parts[1] === 'activity') content = <PlatformActivity works={works} initial={query.get('type') || 'All'}/>;
  else if (section === 'platform') content = <PlatformPage/>;
  else if (section === 'identity') content = <IdentityPage/>;
  else content = <><Head title="Work not found"/><a href={href('work')}>Back to Work</a></>;
  void location; // hashchange drives the render; route() reads the current URL.
  return <div className="portal-shell"><a className="portal-skip" href="#portal-main">Skip to content</a><aside className="portal-sidebar"><a className="portal-brand" href={href('work')}><span>A</span>Agenova</a>
    <nav aria-label="Main navigation"><a className={section === 'work' ? 'active' : ''} href={href('work')}>Work</a><a className={section === 'agents' ? 'active' : ''} href={href('agents')}>Agents</a><a className={section === 'policy' ? 'active' : ''} href={href('policy')}>Policy</a><a className={section === 'platform' ? 'active' : ''} href={href('platform')}>Platform</a></nav>
    <div className="portal-sidebar-foot">Interactive demo<br/>Illustrative records</div></aside><div className="portal-main"><header className="portal-topbar"><span>{section === 'work' ? 'Work' : section === 'agents' ? 'Agents' : section === 'policy' ? 'Policy' : section === 'platform' ? 'Platform' : 'Demo identity'}</span><div><span className="portal-demo-indicator">Example data · no live connection</span><a href={href('identity')} className="portal-identity">TA <span>{demoIdentity.displayName}</span></a></div></header>
      <main id="portal-main" className="portal-page">{content}</main></div></div>;
}
