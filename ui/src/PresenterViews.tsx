// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';
import { connectedSource, validPolicyRule, workTitle, workStatus, type Setup, type View, type PolicyRule } from './connected-source';

export function ConfigurationContext({ setup }: { setup: Setup }) {
 const navigate = (project: string, template?: string) => { const url = new URL(window.location.href); url.searchParams.set('project',project); template ? url.searchParams.set('template',template) : url.searchParams.delete('template'); window.location.assign(url.href); };
 return <div className="portal-context"><span>{setup.organization?.displayName || setup.principal.team}</span><label>Configuration project<select value={setup.project || setup.projects?.[0] || ''} onChange={e => navigate(e.target.value)}>{(setup.projects || [...new Set(setup.policy.Rules.map(rule => rule.project))]).map(project => <option key={project}>{project}</option>)}</select></label>
  {setup.templates && setup.templates.length > 0 && <label>Registered agent<select value={setup.template?.metadata.name || ''} onChange={e => navigate(setup.project || '',e.target.value)}>{setup.templates.map(name => <option key={name}>{name}</option>)}</select></label>}<small>Configuration visibility is not permission to run.</small></div>;
}

export function AgentCatalog({ setup }: { setup: Setup }) {
 const t=setup.template;
 return <><div className="portal-head"><div><h1>Agents</h1><p>Bring your application. Keep governance outside its code.</p></div></div>
  {!t ? <p>No approved template is available in this project.</p> : <section className="portal-panel"><div className="portal-head"><div><small>REGISTERED TEMPLATE</small><h2>{t.metadata.name}</h2></div><a className="portal-button primary" href="#/work/new">Start work</a></div>
  <div className="portal-template-diagram"><div><strong>Your agent image</strong><p className="portal-break">{t.spec.artifact?.image || 'Not recorded'}</p><small>{t.spec.artifact?.image?.includes('@sha256:') ? 'Digest-pinned reference' : 'Tag or image reference · digest not recorded'}</small></div><span aria-hidden="true">→</span><div><strong>One governed Work</strong><p>Temporary authority, isolated execution and recorded decisions.</p><small>Image registration does not attest compatibility.</small></div></div>
  <dl className="portal-metadata"><dt>Entrypoint</dt><dd><code>{t.spec.entrypoint?.command?.join(' ') || 'Not recorded'}</code></dd><dt>Execution contract</dt><dd>Registered entrypoint; verify compatibility with the installed runtime.</dd></dl>
  <h3>Capability ceiling, not standing permission</h3>{Object.entries(t.spec.capabilityCeiling || {}).map(([key,value]) => <div className="portal-fact" key={key}><span>{key}</span><strong>{Array.isArray(value) ? value.join(', ') || 'None' : value || 'Not specified'}</strong></div>)}
  <details className="portal-details"><summary>Show template</summary><pre>{JSON.stringify(t,null,2)}</pre></details></section>}</>;
}

function comparable(work: View): Record<string,string> {
 const a=work.state?.effectiveAuthority;
 const list=(values?: string[] | null)=>values?.join(', ') || 'None';
 return { Caller:work.state?.principal.subject || 'Not recorded', Agent:work.request.spec.templateRef,
  Task:String(work.request.spec.task?.input?.objective || ''), 'Task type':work.request.spec.task?.type || 'Not recorded',
  Project:work.request.spec.projectRef || 'Not recorded', Policy:work.state ? `${work.state.policyRef.id} / ${work.state.policyRef.version}` : 'Not recorded',
  'Requested resources':list(work.request.spec.requestedAccess?.resourceScopes), 'Granted resources':a ? list(a.resourceScopes) : 'No authority issued',
  'Requested tools':list(work.request.spec.requestedAccess?.tools), 'Granted tools':a ? list(a.tools) : 'No authority issued',
  'Requested network':list(work.request.spec.requestedAccess?.networkProfiles), 'Granted network':a ? list(a.networkProfiles) : 'No authority issued',
  Outcome:workStatus(work), 'Image at execution':'Not recorded in this evidence contract',
 };
}
export function WorkComparison({ works }: { works: View[] }) {
 const query=new URLSearchParams(window.location.hash.split('?')[1] || '');
 const [left,setLeft]=useState(query.get('left') || works[0]?.requestRef || '');
 const [right,setRight]=useState(query.get('right') || works[1]?.requestRef || '');
 const [loaded,setLoaded]=useState<View[]>([]); const [error,setError]=useState('');
 useEffect(()=>{const controller=new AbortController();setLoaded([]);setError('');if(!left || !right)return;
  Promise.all([connectedSource.request(left,controller.signal),connectedSource.request(right,controller.signal)]).then(value=>{if(!controller.signal.aborted)setLoaded(value);}).catch(()=>{if(!controller.signal.aborted)setError('Comparison unavailable. Both records require current access.');});return()=>controller.abort();},[left,right]);
 const values=loaded.map(comparable);
 return <><nav className="portal-crumbs"><a href="#/work">Work</a> / Compare</nav><div className="portal-head"><div><h1>Compare two Work requests</h1><p>Same agent does not mean the same authority. Historical snapshots, not current configuration.</p></div></div>
  <div className="portal-field-row">{[left,right].map((value,index)=><label key={index}>Work {index===0?'A':'B'}<select value={value} onChange={e=>(index===0?setLeft:setRight)(e.target.value)}><option value="">Choose a Work</option>{[...new Set([...works.map(w=>w.requestRef),left,right].filter(Boolean))].map(ref=><option key={ref} value={ref}>{works.find(w=>w.requestRef===ref) ? workTitle(works.find(w=>w.requestRef===ref)!) : ref}</option>)}</select></label>)}</div>
  {error && <p role="alert">{error}</p>}{left===right && left && <p>Choose two different Work requests.</p>}
  {values.length===2 && <div className="portal-table-wrap"><table className="portal-table portal-compare-table"><thead><tr><th>Recorded dimension</th>{loaded.map((w,i)=><th key={i}><a href={`#/work/${encodeURIComponent(w.requestRef)}`}>{workTitle(w)}</a></th>)}</tr></thead><tbody>{Object.keys(values[0]).map(key=><tr key={key} data-different={values[0][key]!==values[1][key]}><th>{key}</th><td>{values[0][key]}</td><td>{values[1][key]}</td></tr>)}</tbody></table></div>}
 </>;
}

export function SafetySummary({ work }: { work: View }) {
 const denies=work.facts.filter(f=>f.result==='Deny' || f.decision?.result==='Deny');
 const admission=work.state?.decision.result==='Deny';
 const activated=work.facts.find(f=>f.operation==='NetworkActive');
 const recordedCalls=work.facts.filter(f=>f.kind==='ProviderAttempt');
 return <section className={`portal-panel portal-boundary ${denies.length || admission ? 'has-blocks':''}`} aria-label="Execution boundary">
  <div className="portal-head"><div><small>PER-WORK GOVERNANCE</small><h2>{admission ? 'Stopped before execution' : denies.length ? 'Unauthorized access blocked' : 'Authority scoped to this Work'}</h2></div><span className="portal-badge">{denies.length || (admission?1:0)} blocked checks</span></div>
  <p className="portal-scope-summary"><strong>Requested:</strong> {work.request.spec.requestedAccess?.resourceScopes?.join(', ') || 'No resource scope requested'} <span aria-hidden="true">→</span> <strong>Granted:</strong> {work.state?.effectiveAuthority?.resourceScopes?.join(', ') || 'None'}</p>
  <div className="portal-boundary-grid"><div><strong>Policy</strong><p>{work.state?.policyRef.id || 'Not recorded'} / {work.state?.policyRef.version || '—'}</p><small>{work.state?.decision.result || 'Decision pending'}</small></div><div><strong>Governed interfaces</strong><p>{recordedCalls.length} recorded call attempts</p><small>Access decisions and calls are separate evidence.</small></div><div><strong>Infrastructure</strong><p>{activated ? activated.reason?.includes('Simulated') ? 'Simulated network lifecycle' : 'Network activation recorded' : 'Enforcement proof not recorded'}</p><small>Not an independently verified packet-blocking test.</small></div></div>
  {admission && <p>{work.state?.decision.reason} {work.state?.claim ? '' : 'No claim was issued.'} Absence of call records alone is not independent zero-call proof.</p>}
  {denies.slice(0,3).map(f=><div className="portal-blocked-event" key={f.id}><strong>{f.operation==='tool.invoke' || f.kind==='ToolDecision' ? 'Tool Gateway' : f.kind==='ModelDecision' ? 'Model Gateway' : 'Admission'}</strong><span>{f.target || 'Access check'}<small style={{display:'block',marginTop:6}}>{f.reason || f.decision?.reason || 'Access denied'}</small></span><a href={`#/work/${encodeURIComponent(work.requestRef)}/activity/${encodeURIComponent(f.id)}`}>Open evidence →</a></div>)}
  <a href={`#/work/${encodeURIComponent(work.requestRef)}/access`}>Compare requested and granted →</a>
 </section>;
}

export function ObservabilityCard({ setup }: { setup: Setup }) {
 const o=setup.observability;
 let destination:string|undefined;
 try {const url=new URL(o?.diagnosticURL || '',window.location.origin);if(o?.diagnosticURL && !url.username&&!url.password&&!url.search&&!url.hash&&(url.protocol==='https:'||url.protocol==='http:'&&['127.0.0.1','localhost','[::1]'].includes(url.hostname)))destination=url.href;}catch{/* Invalid configuration is not a navigable destination. */}
 return <section className="portal-panel"><div className="portal-head"><div><small>PLUGGABLE OBSERVABILITY</small><h2>Observability</h2></div><span className="portal-badge">{o?.enabled ? 'Configured' : 'Not configured'}</span></div>
  <div className="portal-boundary-grid"><div><strong>Application → OTLP</strong><p>{o?.endpoint || 'No receiver configured'}</p><small>Platform configuration; no provider credentials displayed.</small></div><div><strong>Export health</strong><p>{o?.enabled ? `${o.exportFailures} export failures recorded` : 'Not active'}</p><small>Export success is not proof of downstream ingestion.</small></div><div><strong>Destination verification</strong><p>{o?.ingestion==='verified' ? 'Verified by an external acceptance record' : 'Not verified'}</p><small>Logs, metrics and traces need separate downstream checks.</small></div></div>
  <p>Signals: {o?.enabled ? 'OTLP logs / metrics / traces configured; downstream routing and receipt not verified.' : 'Not connected.'}</p>{destination && <a className="portal-button" href={destination} target="_blank" rel="noreferrer">Open configured diagnostics ↗</a>}
 </section>;
}

export function PolicyRules({ setup }: { setup: Setup }) {
 return <>{setup.policy.Rules.map((rule:PolicyRule,index)=><article className="portal-panel portal-rule" key={index}><div className="portal-head"><h3>{rule.templateRef}</h3><span className="portal-badge">{rule.action}</span></div>
  <p>{rule.subject ? `Caller ${rule.subject}` : 'Matching callers'} · {rule.team} · {rule.project}</p>
  {rule.constraints ? Object.entries(rule.constraints).map(([key,value])=><div className="portal-fact" key={key}><span>{key}</span><strong>{typeof value==='object' && value && 'values' in value ? (Array.isArray(value.values) ? value.values.join(', ') : '') || 'Explicit empty set · none allowed' : key==='maxTimeout' && typeof value==='number' ? `${value/1e9} seconds` : String(value)}</strong></div>) : <p className="portal-muted">No additional rule caps. Template and requested-access limits still apply.</p>}
 </article>)}</>;
}

interface History { active: { id:string; version:string } | null; versions: { policy: Setup['policy']; active:boolean }[]; events:{actor:string;target:string;at:string}[] }
function validHistory(value:unknown):value is History{
 if(!value || typeof value!=='object')return false;
 const h=value as History;
 return (h.active===null||!!h.active&&typeof h.active.id==='string'&&typeof h.active.version==='string') && Array.isArray(h.versions)&&h.versions.length<=100&&h.versions.every(item=>item&&typeof item.active==='boolean'&&item.policy&&typeof item.policy.ID==='string'&&typeof item.policy.Version==='string'&&(item.policy.Rules===null||Array.isArray(item.policy.Rules)&&item.policy.Rules.every(validPolicyRule))) && Array.isArray(h.events)&&h.events.length<=50&&h.events.every(event=>event&&[event.actor,event.target,event.at].every(item=>typeof item==='string'));
}
export function PolicyHistory({ setup }: { setup: Setup }) {
 const [history,setHistory]=useState<History>();const [error,setError]=useState('');
 const q=new URLSearchParams(window.location.hash.split('?')[1]||'');const project=q.get('project') || setup.project || setup.policy.Rules[0]?.project || '';
 const id=q.get('id'),version=q.get('version');
 useEffect(()=>{const controller=new AbortController();setHistory(undefined);setError('');fetch(`/api/governance?project=${encodeURIComponent(project)}`,{signal:controller.signal,cache:'no-store',redirect:'error'}).then(async r=>{if(!r.ok)throw new Error('Policy history is unavailable for this project.');const value:unknown=await r.json();if(!validHistory(value))throw new Error('Policy history is incomplete. No current document is substituted for history.');return {...value,versions:value.versions.map(entry=>({...entry,policy:{...entry.policy,Rules:entry.policy.Rules||[]}}))};}).then(value=>{if(!controller.signal.aborted)setHistory(value);}).catch(cause=>{if(!controller.signal.aborted)setError(cause instanceof Error?cause.message:'Policy history unavailable.');});return()=>controller.abort();},[project,setup.policy.ID,setup.policy.Version]);
 const selected=history?.versions.find(item=>item.policy.ID===id && item.policy.Version===version);
 return <section className="portal-panel"><h2>Immutable versions & activation history</h2>{error && <p role="status">{error}</p>}
  {id && version && <p>Requested historical policy: {id} / {version}. {selected ? 'Stored immutable document found.' : 'Historical document unavailable; current Policy is not substituted.'}</p>}
  {selected && <PolicyRules setup={{...setup,policy:selected.policy}}/>}
  {history?.versions.map(item=><div className="portal-fact" key={item.policy.ID+'@'+item.policy.Version}><a href={`#/policy?project=${encodeURIComponent(project)}&id=${encodeURIComponent(item.policy.ID)}&version=${encodeURIComponent(item.policy.Version)}`}>{item.policy.ID} / {item.policy.Version}</a><strong>{item.active?'Active for new Work':'Historical version'}</strong></div>)}
  {history && !history.versions.length && <p>No project-owned versions yet. The installation Policy may be the active fallback.</p>}
  <details className="portal-details"><summary>Activation audit</summary>{history?.events.map((event,i)=><p key={i}>{event.at} · {event.actor} · {event.target}</p>)}</details><p>Publish or reactivate an immutable version in <a href="/admin#governance">Administration</a>. Existing Work retains its original snapshot.</p>
 </section>;
}
