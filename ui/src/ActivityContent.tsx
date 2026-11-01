// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {useEffect, useState} from 'react';
import type {ActivityContent as Content} from './contracts.generated';
import {shapeDiagnostics} from './shape-check';

function readable(value:string){try{return JSON.stringify(JSON.parse(value),null,2);}catch{return value;}}
export function ActivityContent({work,reference}:{work?:string;reference?:string}) {
 const [state,setState]=useState<{key:string;content?:Content;error?:string}>();
 const key=`${work}/${reference}`;
 useEffect(()=>{
  if(!work||!reference)return;
  const abort=new AbortController();
  fetch(`/api/requests/${encodeURIComponent(work)}/content/${encodeURIComponent(reference)}`,{cache:'no-store',signal:AbortSignal.any([abort.signal,AbortSignal.timeout(8000)])})
  .then(async response=>{if(!response.ok)throw Error();const content:unknown=await response.json();if(shapeDiagnostics('ActivityContent',content).length)throw Error();if(!abort.signal.aborted)setState({key,content:content as Content});})
  .catch(()=>{if(!abort.signal.aborted)setState({key,error:'Content unavailable or expired.'});});
  return()=>abort.abort();
 },[work,reference,key]);
 if(!reference||!work)return <p className="portal-content-note">Content was not captured for this record.</p>;
 if(state?.key!==key)return <p className="portal-content-note" role="status">Loading content…</p>;
 if(!state.content)return <p className="portal-content-note">{state.error}</p>;
 const c=state.content;
 return <div className="portal-activity-content">
  <div className="portal-content-metrics">{c.model&&<span>{c.model}</span>}{c.elapsedMs!==undefined&&<span>{(c.elapsedMs/1000).toFixed(1)} s</span>}{c.outputTokens!==undefined&&<span>{c.inputTokens!==undefined?`${c.inputTokens} in · `:''}{c.outputTokens} out</span>}{c.finishReason&&<span>{c.finishReason}</span>}</div>
  {(c.promptMs||c.generateMs||c.loadMs)?<details className="portal-content-block"><summary>Timing</summary><dl className="portal-content-timing">{c.loadMs!==undefined&&<><dt>Loading</dt><dd>{(c.loadMs/1000).toFixed(2)} s</dd></>}{c.promptMs!==undefined&&<><dt>Input processing</dt><dd>{(c.promptMs/1000).toFixed(2)} s</dd></>}{c.generateMs!==undefined&&<><dt>Generation</dt><dd>{(c.generateMs/1000).toFixed(2)} s</dd></>}</dl></details>:null}
  {c.decisionSummary&&<section><h4>Decision summary <small>Model reported</small></h4><p>{c.decisionSummary}</p></section>}
  {c.thinking&&<details className="portal-content-block"><summary>Thinking · Provider returned</summary><pre>{c.thinking}</pre></details>}
  {c.input&&<section><h4>Request</h4><pre>{readable(c.input)}</pre></section>}
  {c.output&&<section><h4>{c.kind==='model-response'?'Model response':c.kind==='agent-feedback'?'Agent feedback':c.kind==='tool-request'?'Gateway decision':'Tool result'}</h4><pre>{readable(c.output)}</pre></section>}
  {c.truncated&&<p className="portal-content-note">Content truncated.</p>}
 </div>;
}
