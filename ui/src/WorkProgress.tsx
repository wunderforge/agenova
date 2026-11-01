// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {useEffect,useState} from 'react';
import type {View} from './connected-source';
import {flowStages} from './RunFlow';
import {workStatus} from './connected-source';

export function jumpToWorkFact(id:string){
 const target=document.getElementById(`work-fact-${id}`);
 if(!target)return;
 window.dispatchEvent(new CustomEvent('agenova:inspect-fact',{detail:id}));
 let parent=target.parentElement;
 while(parent){if(parent instanceof HTMLDetailsElement)parent.open=true;parent=parent.parentElement;}
 document.querySelectorAll('.portal-event-highlight').forEach(e=>e.classList.remove('portal-event-highlight'));
 target.classList.add('portal-event-highlight');target.scrollIntoView({behavior:'smooth',block:'center'});
}

export function WorkProgress({work}:{work:View}){
 const status=workStatus(work);const terminal=!!work.outcome||!!work.recovery||['Denied','Cancelled','Expired','Approval required','Failed','Interrupted'].includes(status);
 const [now,setNow]=useState(Date.now());
 useEffect(()=>{if(terminal)return;const id=setInterval(()=>setNow(Date.now()),1000);return()=>clearInterval(id);},[terminal,work.requestRef]);
 const facts=work.facts;const reversed=[...facts].reverse();
 const cleanup=reversed.find(f=>['CleanupSucceeded','CleanupFailed'].includes(f.operation||''));
 const lastModel=reversed.find(f=>f.kind==='ProviderOutcome'&&f.operation==='model.invoke');
 const started=facts.some(f=>f.operation==='Running');
 const stages=flowStages({status,received:facts.some(f=>f.kind==='RequestReceived'),authorized:work.state?.decision.result==='Allow'&&!!work.state.effectiveAuthority,started,result:!!work.outcome,cleanup:cleanup?.operation==='CleanupSucceeded',failureAt:cleanup?.operation==='CleanupFailed'?'Cleanup':status==='Failed'?lastModel?.providerStatus==='Failed'?'Model':started?'Worker':'Request':undefined});
 const latestAction=reversed.find(f=>f.operation==='ActionValidated');
 const pending=[...facts].reverse().find(f=>f.kind==='ProviderAttempt'&&!facts.some(o=>o.kind==='ProviderOutcome'&&o.invocationId===f.invocationId));
 const active=!terminal&&pending;
 const current=active?active.operation==='tool.invoke'?`${active.target||'Tool'} running`:'Waiting for model':terminal?status:status==='Running'?'Agent running':status;
 const first=Date.parse(facts[0]?.timestamp||'');const last=Date.parse(facts.at(-1)?.timestamp||'');
 const duration=Number.isFinite(first)?Math.max(0,((terminal?last:now)-first)/1000):undefined;
 const reservations=facts.filter(f=>f.kind==='ModelBudgetReservation');
 const used=reservations.length||facts.filter(f=>f.kind==='ProviderAttempt'&&f.operation==='model.invoke').length;
 const limit=work.state?.effectiveAuthority?.runtime.maxModelCalls||6;
 const tracked=reservations.length>0||work.request.spec.runtime?.maxModelCalls!=null||!!work.state?.effectiveAuthority?.runtime.maxModelCalls;
 const targets=[facts.find(f=>f.kind==='RequestReceived')?.id,reversed.find(f=>f.kind==='AuthorityResolved'||f.kind==='RequestResolution')?.id,latestAction?.id||facts.find(f=>f.operation==='Running')?.id,reversed.find(f=>f.kind==='RunOutcome')?.id,cleanup?.id];
 return <section className="portal-work-progress" aria-label="Work progress">
  <div className="portal-progress-head"><h2>Work status</h2><span role="status">{current}{duration!==undefined&&<small> {duration.toFixed(1)}s</small>}</span></div>
  <ol>{stages.map((stage,i)=><li key={stage.label} data-stage-state={stage.state}><button type="button" disabled={!targets[i]} onClick={()=>targets[i]&&jumpToWorkFact(targets[i]!)} aria-current={stage.state==='active'?'step':undefined}><span className="portal-progress-orb" aria-hidden="true">{stage.state==='done'?'✓':stage.state==='blocked'?'!':i+1}</span><span>{stage.label==='Worker'?'Agent':stage.label==='Request'?'Requested':stage.label==='Access'?'Authorized':stage.label}</span><small>{stage.label==='Result'&&work.outcome?'Recorded':stage.detail}</small></button></li>)}</ol>
  <div className="portal-progress-foot"><span>Model calls · {tracked?`${used} / ${limit}`:used?`${used} recorded`:'Not recorded'}</span><button className="portal-link-button" type="button" onClick={()=>{const target=document.getElementById('work-activity');target?.scrollIntoView({behavior:'smooth',block:'start'});}}>Activity</button></div>
 </section>;
}
