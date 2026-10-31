// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { connectedSource, suggestedProjects, SourceError, type Setup, type View, type AccessMode, type AccessPlan } from './connected-source';
import {AccessAdviceReview} from './AccessAdviceReview';
import './access-advice.css';
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
 const [accessMode,setAccessMode]=useState<AccessMode>(copy?'manual':initial.template?.spec.accessGuide?initial.accessAdvisor?.defaultMode||'manual':'manual');
 const [advice,setAdvice]=useState<AccessPlan>();
 const [draftAutomatic,setDraftAutomatic]=useState(false);
 const formRef=useRef<HTMLFormElement>(null);
 const requestName=useRef(`work-${crypto.randomUUID()}`);
 function invalidateAdvice(){setAdvice(undefined);requestName.current=`work-${crypto.randomUUID()}`;}
 const [setup, setSetup] = useState(initial);
 const [loading, setLoading] = useState(false);
 const [error, setError] = useState('');
 const [busy, setBusy] = useState(false);
 const [planning, setPlanning] = useState(false);
 const [draft, setDraft] = useState<ClaimRequest>();
 const [pending, setPending] = useState(pendingReference);
 const [tools, setTools] = useState<string[]>((copy?.request.spec.requestedAccess?.tools || []).filter(tool=>initial.template?.spec.capabilityCeiling?.tools?.includes(tool)));
 const [taskType, setTaskType] = useState(copy?.request.spec.task?.type || '');
 const [formKey, setFormKey] = useState(0);
 const [preset,setPreset]=useState('');
 const [objective,setObjective]=useState(String(copy?.request.spec.task?.input?.objective||''));
 const [scopes,setScopes]=useState(copy?.request.spec.requestedAccess?.resourceScopes?.filter(value=>initial.template?.spec.capabilityCeiling?.resourceScopes?.includes(value)).join(', ')||'');
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
  invalidateAdvice();setAccessMode('manual');
  contextRequest.current?.abort(); const controller = new AbortController(); contextRequest.current = controller;
  setLoading(true); setError(''); setTools([]); setDraft(undefined);setPreset('');setObjective(copying?String(input?.objective||''):'');setScopes('');setTaskType(copying?original?.spec.task?.type||'':'');
  try {
   const next = await connectedSource.setup(controller.signal, { project, template: name });
   if (controller.signal.aborted) return;
   const copiedScopes=copying?original?.spec.requestedAccess?.resourceScopes?.filter(value=>next.template?.spec.capabilityCeiling?.resourceScopes?.includes(value)).join(', ')||'':'';
   setSetup(next);setScopes(copiedScopes);if(copying)setTools((original?.spec.requestedAccess?.tools||[]).filter(tool=>next.template?.spec.capabilityCeiling?.tools?.includes(tool)));setFormKey(key => key + 1);
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
 async function send(request: ClaimRequest,automatic=false) {
  setBusy(true); setError(''); remember(request.metadata.name);
  try { const result = automatic?await connectedSource.autoSubmit(request):await connectedSource.submit(request,undefined,advice?.status==='ready'?advice.id:undefined); clearPending(); setDraft(undefined); created(result); }
  catch (cause) {
   const uncertain = !(cause instanceof SourceError) || cause.uncertain;
   if (!uncertain) { clearPending(); setDraft(undefined); }
   setError(cause instanceof Error ? cause.message : 'Submission result is not confirmed.');
  } finally { setBusy(false); }
 }
 function buildRequest(form:HTMLFormElement,planning=false):ClaimRequest {
  const values = new FormData(form);
  const list = (name: string) => String(values.get(name) || '').split(',').map(value => value.trim()).filter(Boolean);
  const text = (name: string) => String(values.get(name) || '').trim();
  const request: ClaimRequest = { apiVersion: 'agenova.io/v1alpha1', kind: 'ClaimRequest', metadata: { name: requestName.current }, spec: {
   templateRef: template!.metadata.name, projectRef: text('project'),
   task: {...(taskType?{type:taskType}:{}), input: {objective: text('objective'), ...(text('workName') ? {workName: text('workName')} : {}),
    ...(text('repository') ? {repository: text('repository'), baseBranch: text('branch') || 'main'} : {})}},
   requestedAccess: {tools, resourceScopes: list('scopes'), modelProfile: text('model'), memoryScopes: list('memory'), ...(text('network') ? {networkProfiles: [text('network')]} : {})},
   runtime: {profileRef: text('runtime'), timeout: `${text('timeout')}m`},
  }};
  if(planning)request.spec.requestedAccess={};
  return request;
 }
 function choosePreset(value:string){
  const chosen=presets.find(p=>p.id===value);
  setPreset(value);setTools(chosen?[chosen.tool]:[]);setScopes(chosen?.scope||'');
  const rules=chosen&&setup.accessAdvisor?.engine==='rules'
   ?template?.spec.accessGuide?.profiles?.filter(p=>p.requestedAccess.tools?.includes(chosen.tool)&&p.requestedAccess.resourceScopes?.includes(chosen.scope))||[]:[];
  // An ambiguous rules configuration must clarify, never choose the first rule.
  setTaskType(rules.length===1?rules[0].taskType||'':'');
 }
 async function suggest(){
  if(locked||!template||!formRef.current)return;const form=formRef.current;
  const request=buildRequest(form,true);if(!request.spec.task?.input?.objective){setError('Describe the task first.');return;}
  setBusy(true);setPlanning(true);setError('');
  try{const plan=await connectedSource.advise(request);setAdvice(plan);if(plan.status==='ready'&&plan.request){
   const a=plan.request.spec.requestedAccess;setTools(a?.tools||[]);setScopes(a?.resourceScopes?.join(', ')||'');
   for(const [name,value]of Object.entries({model:a?.modelProfile||'',memory:a?.memoryScopes?.join(', ')||'',network:a?.networkProfiles?.[0]||''})){const item=form.elements.namedItem(name);if(item instanceof HTMLInputElement||item instanceof HTMLSelectElement)item.value=value;}
  }}catch(cause){setError(cause instanceof Error?cause.message:'Access advice is unavailable.');}finally{setBusy(false);setPlanning(false);}
 }
 async function submit(event: FormEvent<HTMLFormElement>) {
  event.preventDefault(); if (locked || !template) return;
  if(accessMode==='suggest'&&advice?.status!=='ready'){setError('Get an access suggestion before starting, or choose Manual.');return;}
  const automatic=accessMode==='automatic';const request=buildRequest(event.currentTarget,automatic);
  if (!request.spec.task?.input?.objective || !request.spec.projectRef) { setError('Choose a project and describe the task.'); return; }
  setDraft(request);setDraftAutomatic(automatic);await send(request,automatic);
 }
 const allowedProjects = setup.projects || suggestedProjects(setup);
 const project = allowedProjects.includes(setup.project || '') ? setup.project! : allowedProjects[0] || '';
 const minutes = requestedMinutes(ceiling?.maxTimeout);
 return <><nav className="portal-crumbs"><a href="#/work">Work</a> / New work</nav>
  <div className="portal-head"><h1>{copy ? 'Create another Work' : 'New work'}</h1></div>
  {pending && <section className="portal-panel portal-uncertain" role="status"><h2>Submission result not confirmed</h2><p>Request: <code>{pending}</code>. Check its evidence before creating another task.</p>
   <button className="portal-button" disabled={busy} onClick={() => void reconcile()}>Check existing request</button>{draft && <button className="portal-button" disabled={busy} onClick={() => void send(draft,draftAutomatic)}>Retry identical request</button>}
   <button className="portal-button" disabled={busy} onClick={()=>{if(window.confirm('This does not stop or cancel the earlier Work. It may still execute. Abandon this reference and intentionally prepare a different task?')){clearPending();setDraft(undefined);setError('Earlier request was not cancelled. This will be a new logical task.');}}}>Prepare a different task…</button>
   </section>}
  <form ref={formRef} className="portal-form portal-new-work" onSubmit={event => void submit(event)} onChange={event=>{const name=(event.target as unknown as HTMLInputElement).name;if(['objective','workName','taskType','runtime','timeout','repository','branch','preset'].includes(name))invalidateAdvice();}}>
   <fieldset disabled={locked} className="portal-form-context"><legend className="portal-new-work-sr-only">Assignment context</legend><div className="portal-field-row">
    <label>Project<select name="project" value={project} onChange={event => void select(event.target.value)} required><option value="" disabled>Choose a project</option>{allowedProjects.map(value => <option key={value}>{value}</option>)}</select></label>
    <label>Agent<select value={template?.metadata.name || ''} onChange={event => void select(project, event.target.value)}><option value="" disabled>Choose an agent</option>{(setup.templates || (template ? [template.metadata.name] : [])).map(name => <option key={name}>{name}</option>)}</select></label>
   </div></fieldset>
   {loading && <p role="status">Loading configuration…</p>}
   {!loading && !template && <p role="alert">No approved template is available in this project.</p>}
   <fieldset key={formKey} disabled={locked || !template} className="portal-task-fields"><legend className="portal-new-work-sr-only">Work configuration</legend>
    <section className="portal-new-work-card" aria-labelledby="new-work-task-heading"><h2 id="new-work-task-heading">Task</h2>
     <label>Work name (optional)<input name="workName" maxLength={maxWorkName} defaultValue={String(input?.workName || '')} placeholder="A short name for this task"/></label>
     <label>What should it do?<textarea name="objective" required value={objective} onChange={e=>setObjective(e.target.value)} placeholder={selectedPreset?.example||'Describe the outcome you need.'}/></label>
     <label className="portal-new-work-preset">Task preset<select name="preset" value={preset} onChange={e=>choosePreset(e.target.value)}><option value="">Custom task</option>{presets.map(p=><option key={p.id} value={p.id}>{p.label}</option>)}</select></label>
     {(ceiling?.resourceScopes?.some(scope=>scope.startsWith('repo:'))||original?.spec.task?.type==='repository-change') && <div className="portal-field-row"><label>Repository<input name="repository" defaultValue={String(input?.repository || '')}/></label><label>Base branch<input name="branch" defaultValue={String(input?.baseBranch || 'main')}/></label></div>}
    </section>
    <section className="portal-new-work-card" aria-labelledby="new-work-permissions-heading"><h2 id="new-work-permissions-heading">Permissions</h2>
     {setup.accessAdvisor?.enabled&&<div className="portal-advice-mode"><label>Permission setup<select value={accessMode} onChange={e=>{setAccessMode(e.target.value as AccessMode);setAdvice(undefined);setError('');}}>
      {(['manual',...setup.accessAdvisor.allowedModes.filter(mode=>mode!=='manual')] as AccessMode[]).map(mode=><option key={mode} value={mode} disabled={mode!=='manual'&&!template?.spec.accessGuide}>{({manual:'Manual',suggest:'Suggest permissions',automatic:'Automatic'})[mode]}</option>)}
     </select></label></div>}
     {accessMode==='automatic'&&<p className="portal-new-work-mode-hint">Permissions are planned when you start.</p>}
     {accessMode==='suggest'&&<div className="portal-new-work-suggest"><button type="button" className="portal-button" disabled={locked} onClick={()=>void suggest()}>{planning?'Planning…':advice?.status==='ready'?'Update suggestion':'Suggest permissions'}</button></div>}
     {advice&&<AccessAdviceReview plan={advice}/>}
     <div className="portal-new-work-permission-fields" hidden={accessMode==='automatic'||accessMode==='suggest'&&advice?.status!=='ready'}>
      <fieldset className="portal-new-work-tools"><legend>Tools</legend><div>{(ceiling?.tools || []).map(tool => <label className="portal-check" key={tool}><input type="checkbox" checked={tools.includes(tool)} onChange={event => setTools(current => event.target.checked ? [...current, tool] : current.filter(value => value !== tool))}/>{tool}</label>)}{!ceiling?.tools?.length&&<span className="portal-muted">None available</span>}</div></fieldset>
      <label>Resource scopes<input name="scopes" value={scopes} onChange={e=>setScopes(e.target.value)} placeholder="Resources this task needs"/></label>
      {!!ceiling?.resourceScopes?.length&&<div className="portal-chips">{ceiling.resourceScopes.map(scope => <code key={scope}>{scope}</code>)}</div>}
      <div className="portal-field-row"><label>Model profile<select name="model" defaultValue={template?.spec.defaults?.modelProfile || ceiling?.modelProfiles?.[0] || ''}>{ceiling?.modelProfiles?.map(value => <option key={value}>{value}</option>)}</select></label>
       {!!ceiling?.memoryScopes?.length?<label>Memory scope<input name="memory" defaultValue=""/></label>:<input type="hidden" name="memory" value=""/>}
       {!!ceiling?.networkProfiles?.length?<label>Public network access<select name="network" defaultValue=""><option value="">No public network requested</option>{ceiling.networkProfiles.map(profile => <option key={profile}>{profile}</option>)}</select></label>:<input type="hidden" name="network" value=""/>}
      </div>
     </div>
    </section>
    <section className="portal-new-work-card" aria-labelledby="new-work-runtime-heading"><h2 id="new-work-runtime-heading">Run settings</h2><div className="portal-field-row"><label>Runtime profile<select name="runtime">{ceiling?.runtimeProfiles?.map(value => <option key={value}>{value}</option>)}</select></label><label>Time limit (minutes)<input name="timeout" type="number" min="0.000001" max="120" step="any" defaultValue={minutes} required/></label></div></section>
   </fieldset>
   <details className="portal-new-work-config"><summary>Configuration details</summary><dl><dt>Policy</dt><dd>{setup.policy.ID} / {setup.policy.Version}</dd><dt>Agent image</dt><dd>{template?.spec.artifact?.image || 'Not recorded'}</dd><dt>Organization</dt><dd>{setup.organization?.displayName || setup.principal.team}</dd><dt>Requested by</dt><dd>{setup.principal.subject}</dd></dl></details>
   {error && <p role="alert" className="portal-form-error">{error}{error.includes('Sign in') && <a href="/login"> Sign in</a>}</p>}
   <div className="portal-form-actions portal-new-work-actions"><button className="portal-button primary" disabled={locked || !template || !project || accessMode==='suggest'&&advice?.status!=='ready'} type="submit">{busy&&!planning ? accessMode==='automatic'?'Planning and starting…':'Starting…' : accessMode==='automatic'?'Plan and start work':'Start work'}</button><a href="#/work">Cancel</a></div>
  </form>
 </>;
}
