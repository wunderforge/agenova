// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { connectedSource, suggestedProjects, SourceError, type Setup, type View } from './connected-source';
import type { ClaimRequest } from './contracts.generated';
import { maxWorkName } from './work-name';
import { requestedMinutes } from './duration';
import { taskPresets } from './task-presets';

const pendingKey = 'agenova.pending-request';
function pendingReference(){try{return sessionStorage.getItem(pendingKey)||'';}catch{return '';}}
export function WorkSubmissionCopy({ setup, created, reference }: {setup:Setup;created:(work:View)=>void;reference?:string|null}){
 const [copy,setCopy]=useState<View>();const [error,setError]=useState('');
 useEffect(()=>{const controller=new AbortController();setCopy(undefined);setError('');if(reference)connectedSource.request(reference,controller.signal).then(value=>{if(!controller.signal.aborted)setCopy(value);}).catch(()=>{if(!controller.signal.aborted)setError('The source Work is unavailable. No task or authority was copied.');});return()=>controller.abort();},[reference]);
 if(reference&&!copy)return <p role={error?'alert':'status'}>{error||'Loading the original task request…'}</p>;
 return <WorkSubmission key={reference||'new'} setup={setup} created={created} copy={copy}/>;
}
export function WorkSubmission({ setup: initial, created, copy }: { setup: Setup; created: (work: View) => void; copy?: View }) {
 const [setup, setSetup] = useState(initial);
 const [loading, setLoading] = useState(false);
 const [error, setError] = useState('');
 const [busy, setBusy] = useState(false);
 const [draft, setDraft] = useState<ClaimRequest>();
 const [pending, setPending] = useState(pendingReference);
 const [tools, setTools] = useState<string[]>((copy?.request.spec.requestedAccess?.tools || []).filter(tool=>initial.template?.spec.capabilityCeiling?.tools?.includes(tool)));
 const [taskType, setTaskType] = useState(copy?.request.spec.task?.type || 'investigation');
 const [formKey, setFormKey] = useState(0);
 const [preset,setPreset]=useState('');
 const [objective,setObjective]=useState(String(copy?.request.spec.task?.input?.objective||''));
 const [scopes,setScopes]=useState(copy?.request.spec.requestedAccess?.resourceScopes?.filter(value=>initial.template?.spec.capabilityCeiling?.resourceScopes?.includes(value)).join(', ')||'');
 const [review,setReview]=useState({scopes:copy?.request.spec.requestedAccess?.resourceScopes?.join(', ')||'',network:'',timeout:''});
 const contextRequest = useRef<AbortController | null>(null);
 const locked = busy || loading || !!pending;
 const template = setup.template;
 const ceiling = template?.spec.capabilityCeiling;
 const original = copy?.request;
 const input = original?.spec.task?.input;
 const presets=taskPresets(ceiling?.tools||[],ceiling?.resourceScopes||[]);
 const selectedPreset=presets.find(p=>p.id===preset);
 useEffect(() => () => contextRequest.current?.abort(), []);
 useEffect(()=>{if(copy && (copy.request.spec.projectRef!==initial.project || copy.request.spec.templateRef!==initial.template?.metadata.name))void select(copy.request.spec.projectRef || '',copy.request.spec.templateRef,true);},[copy?.requestRef]);
 async function select(project: string, name?: string,copying=false) {
  contextRequest.current?.abort(); const controller = new AbortController(); contextRequest.current = controller;
  setLoading(true); setError(''); setTools([]); setDraft(undefined);setPreset('');setObjective(copying?String(input?.objective||''):'');setScopes('');setTaskType(copying?original?.spec.task?.type||'investigation':'investigation');
  try {
   const next = await connectedSource.setup(controller.signal, { project, template: name });
   if (controller.signal.aborted) return;
   const copiedScopes=copying?original?.spec.requestedAccess?.resourceScopes?.filter(value=>next.template?.spec.capabilityCeiling?.resourceScopes?.includes(value)).join(', ')||'':'';
   setSetup(next);setScopes(copiedScopes);if(copying)setTools((original?.spec.requestedAccess?.tools||[]).filter(tool=>next.template?.spec.capabilityCeiling?.tools?.includes(tool)));setFormKey(key => key + 1);setReview({scopes:copiedScopes,network:'',timeout:''});
  } catch (cause) { if (!controller.signal.aborted) { setSetup({...setup, template: null}); setError(cause instanceof Error ? cause.message : 'Configuration unavailable.'); } }
  finally { if (!controller.signal.aborted) setLoading(false); }
 }
 function remember(ref: string) { setPending(ref); try{sessionStorage.setItem(pendingKey, ref);}catch{/* Memory-only recovery if browser storage is unavailable. */} }
 function clearPending() { setPending(''); try{sessionStorage.removeItem(pendingKey);}catch{/* No persisted task body or token is ever required. */} }
 async function reconcile() {
  if (!pending || busy) return; setBusy(true); setError('');
  try { const work = await connectedSource.submission(pending); clearPending(); created(work); }
  catch (cause) { setError(cause instanceof SourceError && cause.status === 404
   ? 'No visible record yet. Processing may still be in progress. Retry the identical request or check again; do not create another task.'
   : cause instanceof Error ? cause.message : 'Result remains unknown.'); }
  finally { setBusy(false); }
 }
 async function send(request: ClaimRequest) {
  setBusy(true); setError(''); remember(request.metadata.name);
  try { const result = await connectedSource.submit(request); clearPending(); setDraft(undefined); created(result); }
  catch (cause) {
   const uncertain = !(cause instanceof SourceError) || cause.uncertain;
   if (!uncertain) { clearPending(); setDraft(undefined); }
   setError(cause instanceof Error ? cause.message : 'Submission result is not confirmed.');
  } finally { setBusy(false); }
 }
 async function submit(event: FormEvent<HTMLFormElement>) {
  event.preventDefault(); if (locked || !template) return;
  const values = new FormData(event.currentTarget);
  const list = (name: string) => String(values.get(name) || '').split(',').map(value => value.trim()).filter(Boolean);
  const text = (name: string) => String(values.get(name) || '').trim();
  const request: ClaimRequest = { apiVersion: 'agenova.io/v1alpha1', kind: 'ClaimRequest', metadata: { name: `work-${crypto.randomUUID()}` }, spec: {
   templateRef: template.metadata.name, projectRef: text('project'),
   task: {type: taskType, input: {objective: text('objective'), ...(text('workName') ? {workName: text('workName')} : {}),
    ...(taskType === 'repository-change' && text('repository') ? {repository: text('repository'), baseBranch: text('branch') || 'main'} : {})}},
   requestedAccess: {tools, resourceScopes: list('scopes'), modelProfile: text('model'), memoryScopes: list('memory'), ...(text('network') ? {networkProfiles: [text('network')]} : {})},
   runtime: {profileRef: text('runtime'), timeout: `${text('timeout')}m`},
  }};
  if (!request.spec.task?.input?.objective || !request.spec.projectRef) { setError('Choose a project and describe the task.'); return; }
  setDraft(request); await send(request);
 }
 const allowedProjects = setup.projects || suggestedProjects(setup);
 const project = allowedProjects.includes(setup.project || '') ? setup.project! : allowedProjects[0] || '';
 const minutes = requestedMinutes(ceiling?.maxTimeout);
 return <><nav className="portal-crumbs"><a href="#/work">Work</a> / New work</nav>
  <div className="portal-head"><div><h1>{copy ? 'Create another Work' : 'New work'}</h1><p>Reuse an agent. Request only what this assignment needs.</p></div></div>
  {pending && <section className="portal-panel portal-uncertain" role="status"><h2>Submission result not confirmed</h2><p>Request: <code>{pending}</code>. Check its evidence before creating another task. Only the reference is retained in this browser session.</p>
   <button className="portal-button" disabled={busy} onClick={() => void reconcile()}>Check existing request</button>{draft && <button className="portal-button" disabled={busy} onClick={() => void send(draft)}>Retry identical request</button>}
   <button className="portal-button" disabled={busy} onClick={()=>{if(window.confirm('This does not stop or cancel the earlier Work. It may still execute. Abandon this reference and intentionally prepare a different task?')){clearPending();setDraft(undefined);setError('Earlier request was not cancelled. This will be a new logical task.');}}}>Prepare a different task…</button>
   </section>}
  <form className="portal-form" onSubmit={event => void submit(event)} onChange={event=>{const values=new FormData(event.currentTarget);setReview({scopes:String(values.get('scopes')||''),network:String(values.get('network')||''),timeout:String(values.get('timeout')||'')});}}><div>
   <fieldset disabled={locked} className="portal-form-context"><legend>Assignment context</legend><div className="portal-field-row">
    <label>Project<select name="project" value={project} onChange={event => void select(event.target.value)} required><option value="" disabled>Choose a project</option>{allowedProjects.map(value => <option key={value}>{value}</option>)}</select></label>
    <label>Agent<select value={template?.metadata.name || ''} onChange={event => void select(project, event.target.value)}><option value="" disabled>Choose an agent</option>{(setup.templates || (template ? [template.metadata.name] : [])).map(name => <option key={name}>{name}</option>)}</select></label>
   </div></fieldset>
   {loading && <p role="status">Loading the selected configuration…</p>}
   {!loading && !template && <p role="alert">No approved template is available in this project.</p>}
   <fieldset key={formKey} disabled={locked || !template} className="portal-task-fields"><legend>Task and requested access</legend>
    <section><label>Task preset<select value={preset} onChange={e=>{const value=e.target.value;const chosen=presets.find(p=>p.id===value);setPreset(value);setTools(chosen?[chosen.tool]:[]);setScopes(chosen?.scope||'');setTaskType(chosen?.type||'investigation');setReview(current=>({...current,scopes:chosen?.scope||''}));}}><option value="">Custom task</option>{presets.map(p=><option key={p.id} value={p.id}>{p.label}</option>)}</select></label><small>Presets suggest task type and access. Describe your own task below; existing text is kept.</small><label>Task type<select value={taskType} onChange={event => setTaskType(event.target.value)}><option value="investigation">Investigation</option><option value="document-summary">Document summary</option><option value="repository-change">Repository change</option>{!['investigation','document-summary','repository-change'].includes(taskType) && <option>{taskType}</option>}</select></label>
     <label>Work name (optional)<input name="workName" maxLength={maxWorkName} defaultValue={String(input?.workName || '')} placeholder="Give this task a short name"/></label>
     <label>What should it do?<textarea name="objective" required value={objective} onChange={e=>setObjective(e.target.value)} placeholder={selectedPreset?.example||'Describe the outcome you need from this task.'}/></label>
     {selectedPreset?.tool==='web.fetch'&&<small>Uses the public product sources configured for this project.</small>}
     {tools.includes('web.read')&&setup.installation.platform.startsWith('LOCAL ACCEPTANCE')&&<p className="product-notice">Local preview reads the first public HTTPS URL in your task text. No search service or API key is used.</p>}
     {taskType === 'repository-change' && <div className="portal-field-row"><label>Repository<input name="repository" defaultValue={String(input?.repository || '')}/></label><label>Base branch<input name="branch" defaultValue={String(input?.baseBranch || 'main')}/></label></div>}
    </section><section><h2>Requested access</h2><p>Template limits are a ceiling, not a grant. Policy resolves this request on the server.</p>
     <fieldset><legend>Tools</legend>{(ceiling?.tools || []).map(tool => <label className="portal-check" key={tool}><input type="checkbox" checked={tools.includes(tool)} onChange={event => setTools(current => event.target.checked ? [...current, tool] : current.filter(value => value !== tool))}/>{tool}</label>)}</fieldset>
     <label>Resource scopes<input name="scopes" value={scopes} onChange={e=>setScopes(e.target.value)} placeholder="Choose only the resources this task needs"/></label>
     <div className="portal-chips">{ceiling?.resourceScopes?.map(scope => <code key={scope}>{scope}</code>)}</div><small>Available template scopes · comma-separated request above</small>
     {!ceiling?.networkProfiles?.length&&ceiling?.tools?.some(tool=>tool==='web.fetch'||tool==='web.read')?<><input type="hidden" name="network" value=""/><p className="portal-muted">External data uses the Tool Gateway. Direct Pod public egress is not requested.</p></>:<><label>Public network access<select name="network" defaultValue=""><option value="">No public network requested</option>{(ceiling?.networkProfiles || []).map(profile => <option key={profile}>{profile}</option>)}</select></label><p className="portal-muted">A network request needs Policy approval. Public-web permits public TCP 443 and DNS, not domain-specific or read-only access.</p></>}
     <div className="portal-field-row"><label>Model profile<select name="model" defaultValue={template?.spec.defaults?.modelProfile || ceiling?.modelProfiles?.[0] || ''}>{ceiling?.modelProfiles?.map(value => <option key={value}>{value}</option>)}</select></label><label>Memory scope<input name="memory" defaultValue=""/></label></div>
    </section><section><h2>Runtime request</h2><div className="portal-field-row"><label>Runtime profile<select name="runtime">{ceiling?.runtimeProfiles?.map(value => <option key={value}>{value}</option>)}</select></label><label>Time limit (minutes)<input name="timeout" type="number" min="0.000001" max="120" step="any" defaultValue={minutes} required/></label></div></section>
   </fieldset>
   {error && <p role="alert" className="portal-form-error">{error}{error.includes('Sign in') && <a href="/login"> Sign in</a>}</p>}
   <div className="portal-form-actions"><button className="portal-button primary" disabled={locked || !template || !project} type="submit">{busy ? 'Starting…' : 'Start work'}</button><a href="#/work">Cancel</a></div>
  </div><aside className="portal-panel portal-request-route"><h2>Before execution</h2><p>{setup.principal.subject}</p><dl><dt>Organization</dt><dd>{setup.organization?.displayName || setup.principal.team}</dd><dt>Project</dt><dd>{project || 'None available'}</dd><dt>Agent image</dt><dd className="portal-break">{template?.spec.artifact?.image || 'Not recorded'}</dd><dt>Policy</dt><dd>{setup.policy.ID} / {setup.policy.Version}</dd><dt>Requested tools</dt><dd>{tools.join(', ')||'None'}</dd><dt>Requested resources</dt><dd>{review.scopes||'None'}</dd><dt>Requested network</dt><dd>{review.network||'No public network'}</dd><dt>Time limit</dt><dd>{review.timeout||minutes} minutes</dd></dl><p>Task intent → Policy resolution → temporary Work authority → governed execution.</p><small>This is a request, not a grant. No provider credentials are included.</small></aside></form>
 </>;
}
