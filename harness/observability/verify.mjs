// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Opt-in isolated kind verification. No secret value is written to artifacts.
import { execFileSync, spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createRequire } from 'node:module';
const args=process.argv.slice(2);
function option(name,fallback){const i=args.indexOf(name);return i<0?fallback:args[i+1];}
const context=option('--context','');
if(!context)throw new Error('Select an existing kind context explicitly with --context.');
const namespace='agenova-observability-180';
const directory=resolve('.tmp/observability180');mkdirSync(directory,{recursive:true});
const cli=resolve('.tmp/agenova-180');const state=resolve(directory,'state');
const kubectl=(...args)=>execFileSync('kubectl',['--context',context,'--namespace',namespace,...args],{encoding:'utf8',timeout:240000,maxBuffer:8*1024*1024});
const run=(...args)=>execFileSync(cli,[...args,'--state-dir',state],{encoding:'utf8',timeout:600000,maxBuffer:8*1024*1024});
const save=(name,data)=>writeFileSync(resolve(directory,name),typeof data==='string'?data:JSON.stringify(data,null,2)+'\n');
const namespaceObject={apiVersion:'v1',kind:'Namespace',metadata:{name:namespace,labels:{'app.kubernetes.io/managed-by':'agenova'}}};
execFileSync('kubectl',['--context',context,'apply','-f','-'],{input:JSON.stringify(namespaceObject),stdio:['pipe','ignore','inherit']});
const password=randomBytes(24).toString('base64url');
// This Secret belongs only to this explicit verification namespace.
execFileSync('kubectl',['--context',context,'--namespace',namespace,'apply','-f','-'],{input:JSON.stringify({apiVersion:'v1',kind:'Secret',metadata:{name:'grafana-admin',namespace,labels:{'app.kubernetes.io/managed-by':'agenova'}},type:'Opaque',stringData:{username:'admin',password}}),stdio:['pipe','ignore','inherit']});
let platform=readFileSync('deploy/reference/platform.observability.kind.yaml','utf8').replace('context: kind-agenova-k8s-lab',`context: ${context}`).replace('model: llama3.1:latest',`model: ${option('--model','qwen3:8b')}`);
const platformPath=resolve(directory,'platform.yaml');writeFileSync(platformPath,platform);
save('validate.json',run('platform','validate','-f',platformPath,'--json'));
save('first-plan.json',run('platform','plan','-f',platformPath,'--json'));
console.log('Applying explicit Platform and independent telemetry components…');
save('apply.json',run('platform','apply','-f',platformPath,'--yes','--json'));
const repeat=JSON.parse(run('platform','plan','-f',platformPath,'--json'));save('repeat-plan.json',repeat);
if(repeat.changes?.length)throw new Error('Repeat plan is not empty.');
save('reapply.json',run('platform','apply','-f',platformPath,'--yes','--json'));
run('policy','apply','-f','deploy/reference/demo/policy.yaml');run('agent-template','apply','-f','deploy/reference/demo/engineer.yaml');
const children=[];
function forward(resource,mapping){const child=spawn('kubectl',['--context',context,'--namespace',namespace,'port-forward',resource,mapping,'--address','127.0.0.1'],{stdio:'ignore'});children.push(child);return child;}
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function json(url,options){const response=await fetch(url,{...options,signal:AbortSignal.timeout(10000)});if(!response.ok)throw new Error(`HTTP ${response.status}: ${new URL(url).pathname}`);return response.json();}
async function until(check,label,ms=120000){const end=Date.now()+ms;while(Date.now()<end){try{const value=await check();if(value)return value;}catch{}await delay(1000);}throw new Error(`Timed out: ${label}`);}
async function submit(name,project='payments'){
 const request={apiVersion:'agenova.io/v1alpha1',kind:'ClaimRequest',metadata:{name},spec:{templateRef:'engineer',projectRef:project,task:{type:'investigation',input:{objective:'Return a brief final answer: explain why retry timeouts need a total deadline. Do not use tools.'}},requestedAccess:{tools:[],resourceScopes:[],modelProfile:'coding-standard'},runtime:{profileRef:'standard-isolated',timeout:'5m'}}};
 await json('http://127.0.0.1:8098/api/requests',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(request)});
 return until(async()=>{const v=await json(`http://127.0.0.1:8098/api/requests/${name}/evidence`);return v.outcome?v:undefined;},name,330000);
}
try{
 forward('deployment/agenova-control-plane','8098:8081');forward('service/agenova-grafana','3000:3000');
 forward('service/agenova-tempo','13200:3200');forward('service/agenova-loki','13100:3100');forward('service/agenova-prometheus','19090:9090');
 await until(()=>json('http://127.0.0.1:8098/api/setup'),'API');
 const suffix=Date.now().toString(36);
 console.log('Running real Ollama success and pre-claim denial…');
 const success=await submit(`obs-success-${suffix}`);save('success.json',success);
 if(success.outcome.status!=='Succeeded')throw new Error('Real model Work did not succeed.');
 const denied=await submit(`obs-denied-${suffix}`,'billing');save('denied.json',denied);
 if(denied.state.claim||denied.facts.some(f=>f.kind==='ProviderAttempt'))throw new Error('Deny dispatched or invented claim.');
 const cliView=JSON.parse(run('work','show',success.requestRef,'--json'));if(JSON.stringify(cliView)!==JSON.stringify(success))throw new Error('CLI/API mismatch');
 save('history.json',run('work','history','--json'));
 console.log('Replacing the control plane after completed Work and checking history…');
 kubectl('rollout','restart','deployment/agenova-control-plane');kubectl('rollout','status','deployment/agenova-control-plane','--timeout=90s');
 children[0].kill();forward('deployment/agenova-control-plane','8098:8081');
 const recovered=await until(()=>json(`http://127.0.0.1:8098/api/requests/${success.requestRef}/evidence`),'restarted evidence');save('after-restart.json',recovered);
 if(JSON.stringify(recovered)!==JSON.stringify(success))throw new Error('Restart changed terminal evidence.');
 // Agent execution is independent of optional Collector availability.
 console.log('Stopping Collector, executing another real Work, then restoring…');
 kubectl('scale','deployment/agenova-otel','--replicas=0');
 const outage=await submit(`obs-outage-${suffix}`);save('collector-outage-work.json',outage);
 if(outage.outcome.status!=='Succeeded')throw new Error('Collector outage changed Work outcome');
 kubectl('scale','deployment/agenova-otel','--replicas=1');kubectl('rollout','status','deployment/agenova-otel','--timeout=90s');
 const resumedDeny=await submit(`obs-restored-deny-${suffix}`,'billing');save('post-restore-denied.json',resumedDeny);await delay(7000);
 const trace=await until(()=>json(`http://127.0.0.1:13200/api/traces/${success.traceId}`),'Tempo trace');save('tempo-trace.json',trace);
 const metrics=await until(async()=>{const result=await json('http://127.0.0.1:19090/api/v1/query?query=agenova_operations_total');return result.data?.result?.length?result:undefined;},'Prometheus metrics');save('prometheus.json',metrics);
 const decisions=await until(async()=>{const result=await json('http://127.0.0.1:19090/api/v1/query?query='+encodeURIComponent('agenova_governance_decisions_total{component="admission",decision="Deny"}'));return result.data?.result?.length?result:undefined;},'Governance decision metric');save('decisions.json',decisions);
 const logs=await until(async()=>{const result=await json('http://127.0.0.1:13100/loki/api/v1/query_range?query='+encodeURIComponent('{service_name="agenova-control-plane"}')+'&limit=100');return result.data?.result?.length?result:undefined;},'Loki logs');save('loki.json',logs);
 const require=createRequire(resolve('ui/package.json'));const{chromium}=require('@playwright/test');const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1440,height:1000}});// Reset only this harness-owned Grafana admin using stdin; never expose it in argv/artifacts.
  execFileSync('kubectl',['--context',context,'--namespace',namespace,'exec','-i','deployment/agenova-grafana','--','grafana','cli','--homepath','/usr/share/grafana','admin','reset-admin-password','--password-from-stdin'],{input:password+'\n',stdio:['pipe','ignore','inherit']});
  await page.goto('http://127.0.0.1:3000/login');await page.getByRole('textbox',{name:/email or username/i}).fill('admin');await page.getByLabel('Password',{exact:true}).fill(password);await page.getByRole('button',{name:/log in/i}).click();await page.waitForURL(url=>!url.pathname.includes('login'));await page.goto('http://127.0.0.1:3000/d/agenova/agenova-work-diagnostics');await page.getByText('Work results',{exact:true}).waitFor();await delay(6000);await page.screenshot({path:resolve(directory,'grafana.png'),animations:'disabled'});
 }finally{await browser.close()}
 save('result.json',{context,namespace,success:success.requestRef,denied:denied.requestRef,collectorOutage:outage.requestRef,signals:['traces','metrics','logs'],restartParity:true,idempotent:true,e14:'blocked: upstream real identity not delivered',providerRuntimeFaultInjection:'separate verifier: see fault-result.json'});
 console.log(`PASS: real signals, restart parity, Collector outage and Grafana rendering. Artifacts: ${directory}`);
}finally{for(const child of children)child.kill()}
