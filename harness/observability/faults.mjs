// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Run only after verify.mjs has finished. Mutates solely its dedicated namespace.
import {execFileSync,spawn} from 'node:child_process';
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
const index=process.argv.indexOf('--context');const context=process.argv[index+1];if(index<0||!context)throw new Error('Explicit --context required');
const namespace='agenova-observability-180',directory=resolve('.tmp/observability180'),state=resolve(directory,'state');
const cli=resolve('.tmp/agenova-180');const kubectl=(...args)=>execFileSync('kubectl',['--context',context,'--namespace',namespace,...args],{encoding:'utf8',timeout:240000});
const run=(...args)=>execFileSync(cli,[...args,'--state-dir',state],{encoding:'utf8',timeout:600000,maxBuffer:8*1024*1024});
const save=(name,data)=>writeFileSync(resolve(directory,name),JSON.stringify(data,null,2)+'\n');
let tunnel;
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function json(path,options){const response=await fetch('http://127.0.0.1:8098'+path,{...options,signal:AbortSignal.timeout(10000)});if(!response.ok)throw new Error(`HTTP ${response.status}`);return response.json()}
async function connect(){tunnel?.kill();await delay(500);tunnel=spawn('kubectl',['--context',context,'--namespace',namespace,'port-forward','deployment/agenova-control-plane','8098:8081','--address','127.0.0.1'],{stdio:'ignore'});for(let i=0;i<60;i++){try{await json('/api/setup');return}catch{}await delay(1000)}throw new Error('API unavailable')}
async function submit(name){const body={apiVersion:'agenova.io/v1alpha1',kind:'ClaimRequest',metadata:{name},spec:{templateRef:'engineer',projectRef:'payments',task:{type:'investigation',input:{objective:'Return a brief final answer explaining total retry deadlines.'}},requestedAccess:{tools:[],resourceScopes:[],modelProfile:'coding-standard'},runtime:{profileRef:'standard-isolated',timeout:'3m'}}};await json('/api/requests',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});for(let i=0;i<200;i++){const view=await json(`/api/requests/${name}/evidence`);if(view.outcome)return view;await delay(1000)}throw new Error('Work did not finish')}
const platformPath=resolve(directory,'platform.yaml');const original=readFileSync(platformPath,'utf8');
const role=JSON.parse(kubectl('get','role','agenova-control-plane-runtime','-o','json'));
const patch=rules=>kubectl('patch','role','agenova-control-plane-runtime','--type=json','-p',JSON.stringify([{op:'replace',path:'/rules',value:rules}]));
const suffix=Date.now().toString(36);
try{
 await connect();
 const active=await json('/api/history?limit=100');if(active.items.some(x=>['Pending','Bound','Running'].includes(x.status)))throw new Error('Active Work exists; do not inject faults');
 console.log('Injecting namespace-scoped runtime allocation RBAC fault…');
 const restricted=structuredClone(role.rules).map(rule=>({...rule,resources:rule.resources.filter(resource=>resource!=='sandboxclaims')})).filter(rule=>rule.resources.length);
 let runtime;
 try{patch(restricted);runtime=await submit(`obs-runtime-failed-${suffix}`);save('runtime-failed.json',runtime)}finally{patch(role.rules)}
 if(runtime.outcome.status!=='Failed'||runtime.facts.some(f=>f.kind==='ProviderAttempt')||!runtime.facts.some(f=>f.kind==='Runtime'&&f.operation==='AllocationFailed'))throw new Error('Runtime fault evidence invalid');
 console.log('Applying an explicit unavailable provider model, then restoring Platform…');
 const broken=original.replace(/model: [^\n]+/,'model: agenova-observability-missing-model');const brokenPath=resolve(directory,'provider-fault-platform.yaml');writeFileSync(brokenPath,broken);
 let provider;
 try{save('provider-fault-apply.json',JSON.parse(run('platform','apply','-f',brokenPath,'--yes','--json')));await connect();provider=await submit(`obs-provider-failed-${suffix}`);save('provider-failed.json',provider)}
 finally{save('restored-apply.json',JSON.parse(run('platform','apply','-f',platformPath,'--yes','--json')));await connect()}
 if(provider.outcome.status!=='Failed'||!provider.facts.some(f=>f.kind==='ProviderOutcome'&&f.providerStatus==='Failed'))throw new Error('Provider fault evidence invalid');
 for(const view of [runtime,provider]){const queried=JSON.parse(run('work','show',view.requestRef,'--json'));if(JSON.stringify(queried)!==JSON.stringify(view))throw new Error('Fault CLI/API mismatch after restart')}
 save('fault-result.json',{context,namespace,runtime:runtime.requestRef,provider:provider.requestRef,distinctFailures:true,restored:true});
 console.log('PASS: runtime allocation and provider failures are distinct, durable and CLI/API-consistent; Platform restored.');
}finally{tunnel?.kill()}
