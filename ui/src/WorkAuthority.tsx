// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { View } from './connected-source';
import { workPermissions } from './work-permissions';

export function WorkPermissions({work}:{work:View}){
 const rows=workPermissions(work);
 return <section className="portal-panel portal-work-permissions" aria-label="Task permissions"><h2>Permissions</h2>
  <table aria-label="Requested and granted task permissions"><thead><tr><th>Capability</th><th>Requested</th><th>Granted</th></tr></thead>
   <tbody>{rows.map(row=><tr key={row.id} data-permission={row.id} data-permission-state={row.state}><th scope="row">{row.label}</th><td>{row.requested}</td><td>{row.granted}</td></tr>)}</tbody>
  </table>
 </section>;
}

export function WorkContext({work}:{work:View}){
 const network=work.facts.find(f=>f.operation==='NetworkActive');
 const cleanup=[...work.facts].reverse().find(f=>['CleanupSucceeded','CleanupFailed'].includes(f.operation||''));
 const interfaces=[work.facts.some(f=>f.kind==='ToolDecision')?'Tool Gateway':null,work.facts.some(f=>f.kind==='ModelDecision')?'Model Gateway':null].filter(Boolean).join(' · ');
 const policy=work.state?.policyRef;
 const backend=work.state?.claim?.backendIdentity?.backend;
 const fields=[['Agent',work.request.spec.templateRef],['Requested by',work.state?.principal.subject],['Team / Project',[work.state?.principal.team,work.request.spec.projectRef].filter(Boolean).join(' / ')],
  ['Policy',policy?<a href={`#/policy?project=${encodeURIComponent(work.request.spec.projectRef||'')}&id=${encodeURIComponent(policy.id)}&version=${encodeURIComponent(policy.version)}`}>{policy.id} / {policy.version}</a>:undefined],
  ['Governed interfaces',interfaces||'None recorded'],['Execution backend',backend==='local-synthetic-no-infra-proof'?'Local synthetic runtime':backend],
  ['Infrastructure evidence',network?.reason?.includes('Simulated')?'Simulated':network?'Not independently verified':'Not recorded'],
  ['Lifecycle',work.recovery?'Interrupted · cleanup '+work.recovery.cleanup.toLowerCase():cleanup?.operation==='CleanupSucceeded'?'Cleaned up':cleanup?.operation==='CleanupFailed'?'Cleanup failed':work.state?.claim?.phase],
  ...(work.outcome?.model?[['Model provider',work.outcome.model.model]]:[])];
 return <section className="portal-work-context" aria-label="Work details"><h2>Details</h2><div className="portal-work-context-grid">{fields.map(([label,value])=><div key={String(label)}><span>{label}</span><div>{value||'Not recorded'}</div></div>)}</div>
 </section>;
}

export function WorkRefusals({work,highlight=false}:{work:View;highlight?:boolean}){
 const denied=work.facts.filter(f=>f.result==='Deny'||f.decision?.result==='Deny');
 if(!denied.length)return null;
 return <section className={`portal-work-refusals${highlight?' portal-work-refusals-highlight':''}`} aria-label="Recorded refusals" role={highlight?'alert':undefined}>{denied.map(f=><div className="portal-blocked-event" key={f.id}><span>{highlight?`${f.target||f.operation||'Request'} blocked`:`Blocked · ${f.target||f.operation||'Request'}`}</span><a href={`#/work/${encodeURIComponent(work.requestRef)}/activity/${encodeURIComponent(f.id)}`}>{highlight?'Evidence':'View evidence'}</a></div>)}</section>;
}

export function WorkNotices({work}:{work:View}){
 const denied=work.facts.filter(f=>f.result==='Deny'||f.decision?.result==='Deny');
 const completion=work.outcome?.completion;
 const native=[...work.facts].reverse().find(f=>f.operation==='NativeHTTPSObservation');
 const target=completion?.target;
 const unavailable=completion?.reason==='missing-capability'&&target&&!work.state?.effectiveAuthority?.tools?.includes(target);
 const incomplete=completion&&['partial','incomplete'].includes(completion.status);
 const activity=`#/work/${encodeURIComponent(work.requestRef)}/activity`;
 const failure=work.outcome?.failure;
 const failureFact=[...work.facts].reverse().find(f=>f.kind==='RunOutcome');
 let failureEvent=failure?{id:'failure',text:work.outcome?.status==='Succeeded'?(incomplete?'Cleanup needs attention':'Task completed; cleanup needs attention'):failure,href:failureFact?`${activity}/${encodeURIComponent(failureFact.id)}`:activity,label:'Evidence'}:undefined;
 if(failureEvent&&denied.some(f=>{const target=f.target||f.operation||'Request';return [`${target} blocked`,`Blocked · ${target}`].some(text=>text.trim().toLowerCase()===failureEvent!.text.trim().toLowerCase());}))failureEvent=undefined;
 if(denied.length&&!native&&!unavailable&&!incomplete)return <><WorkRefusals work={work} highlight={work.outcome?.status!=='Succeeded'}/>{failureEvent&&<NoticeRows events={[failureEvent]}/>}</>;
 const events=denied.map(f=>({id:f.id,text:`${f.target||f.operation||'Request'} blocked`,href:`${activity}/${encodeURIComponent(f.id)}`,label:'Evidence'}));
 if(unavailable&&!denied.some(f=>f.target===target))events.push({id:'unavailable',text:`${target} unavailable`,href:`#/work/${encodeURIComponent(work.requestRef)}/completion`,label:'Details'});
 if(native)events.push({id:native.id,text:native.reasonCode?.endsWith('-timeout')?'Direct HTTPS timed out':'Direct HTTPS attempted',href:`${activity}/${encodeURIComponent(native.id)}`,label:'Evidence'});
 if(!events.length&&incomplete)events.push({id:'incomplete',text:completion.status==='partial'?'Task partially completed':'Task incomplete',href:`#/work/${encodeURIComponent(work.requestRef)}/completion`,label:'Details'});
 if(failureEvent&&!events.some(e=>e.text.trim().toLowerCase()===failureEvent.text.trim().toLowerCase()))events.push(failureEvent);
 if(!events.length)return null;
 return <NoticeRows events={events}/>;
}
function NoticeRows({events}:{events:{id:string;text:string;href:string;label:string}[]}){
 return <section className="portal-work-notices portal-work-notices-highlight" aria-label="Work notices" role="alert">{events.map(e=><div className="portal-blocked-event" key={e.id}><span>{e.text}</span><a href={e.href}>{e.label}</a></div>)}</section>;
}
