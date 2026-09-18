// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect,test,type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import type { ClaimRequest,IssuedState,AgentTemplate,Fact } from '../src/contracts.generated';
import type { Setup,View } from '../src/connected-source';
const request=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json',import.meta.url),'utf8')) as ClaimRequest;
const state=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json',import.meta.url),'utf8')) as IssuedState;
const template:AgentTemplate={apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{artifact:{image:'example-worker'},entrypoint:{command:['worker']},defaults:{modelProfile:'approved-coding-model',memoryScopes:['team-docs']},capabilityCeiling:{tools:['git.read','git.write'],resourceScopes:['repo:acme/payments'],modelProfiles:['approved-coding-model'],memoryScopes:['team-docs'],runtimeProfiles:['standard-isolated'],maxTimeout:'30m'}}};
const setup:Setup={principal:state.principal,template,policy:{ID:state.policyRef.id,Version:state.policyRef.version,Rules:[{team:'team-a',action:'claim.create',project:'payments',templateRef:'engineer'}]},capabilities:{taskSubmission:'ready',runtime:'configured',model:'configured',tool:'notConnected',memory:'notConnected'}};
function work(phase:'Running'|'Succeeded'='Running'):View{
 const copy=structuredClone(state);
copy.claim!.phase=phase;
copy.effectiveAuthority!.tools=['git.read'];
copy.effectiveAuthority!.runtime.timeout='15m';
 const facts:Fact[]=[{id:'f1',sequence:1,timestamp:'2026-09-15T02:00:00Z',kind:'RequestResolution',requestRef:copy.requestRef,decision:copy.decision,result:'Allow',reasonCode:'assignment-allow'},{id:'f2',sequence:2,timestamp:'2026-09-15T02:00:01Z',kind:'AuthorityResolved',requestRef:copy.requestRef,claimId:copy.claim!.id,effectiveAuthority:copy.effectiveAuthority,reasonCode:'template-policy-intersection'},{id:'f3',sequence:3,timestamp:'2026-09-15T02:00:02Z',kind:'Runtime',requestRef:copy.requestRef,claimId:copy.claim!.id,operation:phase}];
 return{version:'agenova.evidence/v0',requestRef:copy.requestRef,request:structuredClone(request),state:copy,facts,...(phase==='Succeeded'?{outcome:{status:'Succeeded',text:'Use a bounded retry with exponential backoff.',model:{invocationId:'model-1',model:'local-test-model',inputTokens:12,outputTokens:20}}}:{})};
}
async function api(page:Page,current:()=>View[],submitted?:(request:ClaimRequest)=>void,configured:Setup=setup){
 await page.route('**/api/**',async route=>{const path=new URL(route.request().url()).pathname;
  if(path==='/api/setup')return route.fulfill({json:configured});
  if(path==='/api/requests'&&route.request().method()==='POST'){submitted?.(route.request().postDataJSON() as ClaimRequest);
return route.fulfill({status:202,json:current()[0]});
}
  if(path==='/api/requests')return route.fulfill({json:current()});
  const ref=decodeURIComponent(path.split('/')[3]||'');
const found=current().find(w=>w.requestRef===ref);
return found?route.fulfill({json:found}):route.fulfill({status:404,json:{code:'not_found',message:'Not found'}});
 });
}
test('work name is separate from full instructions and identity', async({page},info)=>{
 let current=work();
 const objective='Investigate payment retries. Read the artifacts, identify the cause, and recommend a specific fix.';
 let submitted:ClaimRequest|undefined;
 await api(page,()=>[current],request=>{
   submitted=request;
   current={...current,request,requestRef:request.metadata.name};
 });
 await page.goto('/?mode=connected#/work/new');
 await page.getByLabel('Work name (optional)').fill('Payment retry investigation');
 await page.getByLabel('What should it do?').fill(objective);
 await page.getByRole('button',{name:'Start work',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Payment retry investigation',exact:true})).toBeVisible();
 expect(submitted?.spec.task?.input?.objective).toBe(objective);
 expect(submitted?.spec.task?.input?.workName).toBe('Payment retry investigation');
 expect(submitted?.metadata.name).toMatch(/^work-[a-f0-9-]+$/);
 const instructions=page.locator('.portal-task-instructions');
 await expect(instructions).not.toHaveAttribute('open');
 await instructions.getByText('Task instructions',{exact:true}).click();
 await expect(instructions.locator('pre')).toHaveText(objective);
 await page.context().grantPermissions(['clipboard-read','clipboard-write']);
 await page.getByText('Execution details',{exact:true}).click();
 await page.getByRole('button',{name:'Copy request ID',exact:true}).click();
 await expect(page.locator('.portal-copy-reference')).toContainText('Copied');
 expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe(current.requestRef);
 await page.goto('/?mode=connected#/work');
 await expect(page.locator('.portal-work-title')).toHaveText('Payment retry investigation');
 await page.getByLabel('Search work').fill('recommend a specific fix');
 await expect(page.locator('.portal-work-title')).toHaveCount(1);
 await page.screenshot({path:info.outputPath('named-work-list.png'),fullPage:true});
});
test('legacy task title uses its first sentence but retains complete instructions',async({page},info)=>{
 const current=work();
 const first='Investigate why synthetic payment retries exceed the deadline.';
 const objective=first+' Read the available artifacts, identify the cause, and recommend a specific fix.';
 current.request.spec.task!.input!.objective=objective;
 await api(page,()=>[current]);
 await page.goto('/?mode=connected#/work');
 await expect(page.locator('.portal-work-title')).toHaveText(first);
 await expect(page.locator('.portal-work-title')).toHaveCSS('-webkit-line-clamp','2');
 await page.screenshot({path:info.outputPath('legacy-short-work-list.png'),fullPage:true});
 await page.locator('.portal-work-title').click();
 await expect(page.locator('.portal-head h1')).toHaveText(first);
 await page.getByText('Task instructions',{exact:true}).click();
 await expect(page.locator('.portal-task-instructions pre')).toHaveText(objective);
});
test('failed work shows a separate red outcome, reason and failed detail record without recoloring successful calls',async({page},info)=>{
 const current=work();
 current.state!.claim!.phase='Failed';
 const reason='The agent exited without returning a final answer.';
 current.outcome={status:'Failed',failure:reason};
 current.facts.push(
  {id:'turn',sequence:4,timestamp:'2026-09-15T02:00:03Z',kind:'WorkerActivity',requestRef:current.requestRef,operation:'TurnStarted',target:'Turn 6'},
  {id:'model-start',sequence:5,timestamp:'2026-09-15T02:00:04Z',kind:'ProviderAttempt',requestRef:current.requestRef,operation:'model.invoke',invocationId:'model-ok'},
  {id:'model-ok',sequence:6,timestamp:'2026-09-15T02:00:05Z',kind:'ProviderOutcome',requestRef:current.requestRef,operation:'model.invoke',invocationId:'model-ok',providerStatus:'Succeeded'},
  {id:'failed-runtime',sequence:7,timestamp:'2026-09-15T02:00:06Z',kind:'Runtime',requestRef:current.requestRef,operation:'Failed'},
  {id:'cleanup-ok',sequence:8,timestamp:'2026-09-15T02:00:07Z',kind:'Runtime',requestRef:current.requestRef,operation:'CleanupSucceeded'},
  {id:'work-failed',sequence:9,timestamp:'2026-09-15T02:00:08Z',kind:'RunOutcome',requestRef:current.requestRef,operation:'Failed',reason,reasonCode:'agent-no-final-result'},
 );
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-agent-failure')).toContainText(reason);
 await expect(page.locator('.portal-agent-failure strong')).toHaveCSS('color','rgb(255, 140, 164)');
 await expect(page.locator('.portal-worker-state.positive')).toHaveText('Succeeded');
 await expect(page.locator('.portal-worker-state.negative')).toHaveCount(0);
 await page.screenshot({path:info.outputPath('work-outcome-failed.png'),fullPage:true});
 await page.getByRole('link',{name:'View failure record'}).click();
 await expect(page.locator('.portal-head .portal-badge.failed')).toHaveText('Failed');
 await expect(page.getByRole('alert')).toContainText(reason);
 await page.screenshot({path:info.outputPath('failed-record-detail.png'),fullPage:true});
 await page.getByRole('link',{name:'Activity',exact:true}).click();
 await expect(page.locator('.portal-record-row').filter({hasText:'Work failed'}).locator('.portal-badge.failed')).toHaveCount(2);
});
test('format retry explains the failing turn without counting checks as calls',async({page},info)=>{
 const current=work();current.state!.claim!.phase='Failed';
 const reason='The agent exhausted its model-turn limit while retrying an invalid tool/finish response format.';
 const issue='The final-answer action contained tool or input fields; both must be empty. The agent must retry.';
 current.outcome={status:'Failed',failure:reason};
 current.facts.push(
  {id:'turn',sequence:4,timestamp:'2026-09-16T02:00:00Z',kind:'WorkerActivity',requestRef:current.requestRef,operation:'TurnStarted',target:'Turn 6'},
  {id:'attempt',sequence:5,timestamp:'2026-09-16T02:00:01Z',kind:'ProviderAttempt',requestRef:current.requestRef,invocationId:'m',operation:'model.invoke'},
  {id:'succeeded',sequence:6,timestamp:'2026-09-16T02:00:02Z',kind:'ProviderOutcome',requestRef:current.requestRef,invocationId:'m',operation:'model.invoke',providerStatus:'Succeeded'},
  {id:'retry',sequence:7,timestamp:'2026-09-16T02:00:03Z',kind:'WorkerActivity',requestRef:current.requestRef,invocationId:'m',operation:'ActionValidated',target:'Turn 6',reasonCode:'agent-action-invalid',reason:issue},
  {id:'failed',sequence:8,timestamp:'2026-09-16T02:00:04Z',kind:'RunOutcome',requestRef:current.requestRef,operation:'Failed',reason,reasonCode:'agent-invalid-action-limit'},
 );
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.getByRole('alert')).toHaveCount(1);
 await expect(page.locator('.portal-turn summary')).toContainText('1 call');
 await expect(page.locator('.portal-turn summary')).toContainText('Format retry');
 await expect(page.locator('li[data-retry=true]')).toContainText(issue);
 await expect(page.locator('li[data-retry=true] .portal-worker-lamp')).toHaveCSS('animation-name','none');
 await expect(page.locator('.portal-worker-state.positive')).toHaveText('Succeeded');
 await page.screenshot({path:info.outputPath('action-format-retry.png'),fullPage:true});
 await page.getByRole('link',{name:'Action check',exact:true}).click();
 await expect(page.locator('.portal-head')).toContainText('Action format checked');
 await expect(page.locator('.portal-head .portal-badge')).toHaveText('Retry required');
 await expect(page.locator('.portal-panel > p')).toContainText(issue);
});

test('historical missing failure reason is explicit, not inferred from successful model calls',async({page})=>{
 const current=work();current.state!.claim!.phase='Failed';
 current.outcome={status:'Failed',failure:'Execution or cleanup failed; inspect the recorded activity.'};
 current.facts.push({id:'failed',sequence:4,timestamp:'2026-09-15T02:00:08Z',kind:'RunOutcome',requestRef:current.requestRef,operation:'Failed'});
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}/activity/failed`);
 await expect(page.locator('.portal-head .portal-badge.failed')).toHaveText('Failed');
 await expect(page.getByRole('alert')).toContainText('No specific failure reason was recorded for this work.');
});
test('connected motion follows provider and cleanup evidence without resetting DOM or scroll',async({page},info)=>{
 const current=work();
 current.facts.push({id:'provider-start',sequence:4,timestamp:'2026-09-15T02:00:03Z',kind:'ProviderAttempt',requestRef:current.requestRef,invocationId:'call-1',providerStatus:'Attempted'});
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-flow-node.active')).toContainText('Agent');
 await expect(page.locator('.portal-worker')).toContainText('Waiting for a model response');
 expect(await page.evaluate(()=>document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity).length)).toBe(1);
 await page.screenshot({path:info.outputPath('connected-worker-waiting.png'),fullPage:true});
 await page.emulateMedia({reducedMotion:'reduce'});
 expect(await page.locator('.portal-worker-actions li[data-active=true] .portal-worker-lamp').evaluate(el=>getComputedStyle(el).animationName)).toBe('none');
 await page.emulateMedia({reducedMotion:'no-preference'});
 await page.evaluate(()=>{
   (window as unknown as {savedFlow:Element|null}).savedFlow=document.querySelector('.portal-flow');
   window.scrollTo({top:150,behavior:'instant'});
 });
 const position=await page.evaluate(()=>scrollY);
 current.facts.push({id:'provider-finish',sequence:5,timestamp:'2026-09-15T02:00:04Z',kind:'ProviderOutcome',requestRef:current.requestRef,invocationId:'call-1',providerStatus:'Succeeded'});
 await expect(page.locator('.portal-worker-actions')).toContainText('Succeeded');
 await expect(page.locator('.portal-worker')).not.toHaveClass(/has-active-call/);
 expect(await page.evaluate(()=>document.querySelector('.portal-flow')===(window as unknown as {savedFlow:Element|null}).savedFlow)).toBe(true);
 expect(await page.evaluate(()=>scrollY)).toBe(position);
 current.state!.claim!.phase='Succeeded';
 await expect(page.locator('.portal-flow-node.active')).toContainText('Cleanup');
 current.outcome={status:'Succeeded',text:'A real response belongs here only after the provider returns.'};
 await expect(page.locator('.portal-result')).toContainText('A real response belongs here only after the provider returns.');
 await expect(page.locator('.portal-flow-node').last()).toContainText('No record');
 await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
 await page.waitForTimeout(400);
 await page.screenshot({path:info.outputPath('connected-motion-result.png'),fullPage:true});
});

test('cobalt light locates real pending calls and separates success from failure',async({page},info)=>{
 const current=work();
 const add=(kind:string,extra:Partial<Fact>={})=>current.facts.push({id:`signal-${current.facts.length}`,sequence:current.facts.length+1,timestamp:'2026-09-15T02:00:03Z',kind,requestRef:current.requestRef,...extra});
 add('WorkerActivity',{operation:'TurnStarted',target:'Turn 1'});
 add('ProviderAttempt',{invocationId:'signal-model',operation:'model.invoke'});
 await api(page,()=>[current]);
 const cdp=await page.context().newCDPSession(page);
 await cdp.send('Emulation.setCPUThrottlingRate',{rate:4});
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-worker')).toContainText('Waiting for a model response');
 expect(await page.locator('.portal-shell').evaluate(el=>getComputedStyle(el).getPropertyValue('--portal-paper').trim())).toBe('#050914');
 await expect(page.locator('.portal-head .portal-badge')).toHaveCSS('color','rgb(168, 206, 255)');
 const row=page.locator('.portal-worker-actions li[data-active=true]');
 await expect(row).toHaveAttribute('data-kind','model');
 await expect(row.locator('.portal-worker-lamp')).toHaveCount(1);
 await expect(page.locator('.portal-worker-current .portal-worker-lamp')).toHaveCount(0);
 await expect(row.locator('a')).toHaveCSS('text-decoration-line','none');
 expect(await row.evaluate(el=>getComputedStyle(el,'::before').boxShadow)).not.toBe('none');
 await page.locator('.portal-turn > summary').hover();
 await expect.poll(()=>page.locator('.portal-turn > summary').evaluate(el=>getComputedStyle(el,'::after').opacity)).toBe('1');
 const frameReport=await page.evaluate(async()=>{
   const deltas:number[]=[];let last=0;
   await new Promise<void>(resolve=>{let start=0;const frame=(now:number)=>{if(!start)start=now;if(last)deltas.push(now-last);last=now;if(now-start<1000)requestAnimationFrame(frame);else resolve();};requestAnimationFrame(frame);});
   deltas.sort((a,b)=>a-b);
   const ongoing=document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity);
   return {cpuThrottle:4,frames:deltas.length,p95FrameMs:deltas[Math.floor(deltas.length*.95)],framesOver25ms:deltas.filter(d=>d>25).length,
     ongoing:ongoing.length,properties:ongoing.flatMap(a=>Object.keys((a.effect as KeyframeEffect).getKeyframes()[0]).filter(k=>!['offset','computedOffset','easing','composite'].includes(k)))};
 });
 expect(frameReport.ongoing).toBe(1);
 expect(frameReport.properties).toEqual(['opacity']);
 await info.attach('cobalt-wait-performance',{body:JSON.stringify(frameReport,null,2),contentType:'application/json'});
 await page.screenshot({path:info.outputPath('cobalt-model-wait.png'),fullPage:true});
 add('ProviderOutcome',{invocationId:'signal-model',operation:'model.invoke',providerStatus:'Succeeded'});
 add('ProviderAttempt',{invocationId:'signal-tool',operation:'tool.invoke',target:'Mock git.read'});
 await expect(row).toHaveAttribute('data-kind','tool');
 expect(await row.evaluate(el=>getComputedStyle(el).getPropertyValue('--call-light').trim())).toBe('#76d9f4');
 await expect(page.locator('.portal-worker-state.positive')).toHaveCSS('color','rgb(140, 213, 178)');
 await page.screenshot({path:info.outputPath('cobalt-tool-wait.png'),fullPage:true});
 add('ProviderOutcome',{invocationId:'signal-tool',operation:'tool.invoke',providerStatus:'Failed'});
 current.state!.claim!.phase='Failed';current.outcome={status:'Failed',failure:'The recorded mock tool call failed.'};
 await expect(page.locator('.portal-worker-state.negative')).toHaveCSS('color','rgb(255, 140, 164)');
 await expect(page.locator('.portal-turn[data-negative=true] > summary')).toContainText('1 failed');
 await expect(page.locator('.portal-worker')).not.toHaveClass(/has-active-call/);
 expect(await page.evaluate(()=>document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity).length)).toBe(0);
 await page.screenshot({path:info.outputPath('cobalt-failure.png'),fullPage:true});
 await cdp.send('Emulation.setCPUThrottlingRate',{rate:1});
});

test('malformed connected percent escapes show an error instead of a blank page',async({page})=>{
 const errors:string[]=[];
 page.on('pageerror',error=>errors.push(error.message));
 let calls=0;
 await page.route('**/api/**',async route=>{calls++;await route.fulfill({status:500,json:{code:'unexpected',message:'Should not be called'}});});
 await page.goto('/?mode=connected#/work/%');
 await expect(page.getByRole('alert')).toContainText('Invalid work reference.');
 expect(errors).toEqual([]);
 expect(calls).toBe(0);
 await page.goto('/?mode=connected#/work/valid/activity/%');
 await expect(page.getByRole('alert')).toContainText('Invalid work reference.');
 expect(errors).toEqual([]);
 expect(calls).toBe(0);
});

test('worker follows recorded ReAct turns, tool observation and final model response',async({page},info)=>{
 const current=work();
 const add=(kind:string,extra:Partial<Fact>={})=>current.facts.push({id:`loop-${current.facts.length}`,sequence:current.facts.length+1,timestamp:'2026-09-15T02:00:03Z',kind,requestRef:current.requestRef,...extra});
 add('WorkerActivity',{operation:'TurnStarted',target:'Turn 1'});
 add('ProviderAttempt',{invocationId:'m1',operation:'model.invoke'});
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-worker-current')).toContainText('Turn 1 · Waiting for a model response');
 add('ProviderOutcome',{invocationId:'m1',operation:'model.invoke',providerStatus:'Succeeded'});
 add('ToolDecision',{invocationId:'t1',operation:'tool.invoke',result:'Allow'});
 add('ProviderAttempt',{invocationId:'t1',operation:'tool.invoke',target:'Mock git.read · logs/timeout.log'});
 await expect(page.locator('.portal-worker-current')).toContainText('Waiting for tool observation');
 await expect(page.locator('.portal-worker-actions li[data-active=true]')).toContainText('Tool call (mock)');
 add('ProviderOutcome',{invocationId:'t1',operation:'tool.invoke',providerStatus:'Succeeded'});
 add('WorkerActivity',{operation:'ObservationReceived',target:'Turn 1'});
 add('WorkerActivity',{operation:'TurnStarted',target:'Turn 2'});
 add('ProviderAttempt',{invocationId:'m2',operation:'model.invoke'});
 await expect(page.locator('.portal-worker-current')).toContainText('Turn 2 · Waiting for a model response');
 await expect(page.locator('.portal-turn[open]')).toHaveCount(1);
 await expect(page.locator('.portal-turn[open] > summary')).toContainText('Turn 2');
 await page.screenshot({path:info.outputPath('react-loop-turn-2.png'),fullPage:true});
 await page.locator('.portal-turn > summary').filter({hasText:'Turn 1'}).focus();
 await page.keyboard.press('Enter');
 await expect(page.locator('.portal-turn[open] > summary')).toContainText('Turn 1');
 // A provider response polls into the page without closing the user's history.
 add('ProviderOutcome',{invocationId:'m2',operation:'model.invoke',providerStatus:'Succeeded'});
 await expect(page.locator('.portal-worker')).not.toHaveClass(/has-active-call/);
 await expect(page.locator('.portal-turn[open] > summary')).toContainText('Turn 1');
 await page.locator('.portal-turn > summary').filter({hasText:'Turn 2'}).click();
 add('WorkerActivity',{operation:'FinalAnswer',target:'Turn 2'});
 current.state!.claim!.phase='Succeeded';current.outcome={status:'Succeeded',text:'Use one shared deadline across all retry attempts.'};
 await expect(page.locator('.portal-worker-current')).toContainText('Turn 2 · No active calls');
 await expect(page.locator('.portal-worker')).not.toHaveClass(/has-active-call/);
 await expect(page.locator('.portal-result')).toContainText('Use one shared deadline');
 expect(await page.locator('.portal-result').evaluate(el=>!!(el.compareDocumentPosition(document.querySelector('.portal-worker')!) & Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
 await expect(page.getByRole('heading',{name:'Progress',exact:true})).toHaveCount(0);
 await expect(page.getByRole('heading',{name:'Recent activity',exact:true})).toHaveCount(0);
 await expect(page.locator('.portal-work-access[open], .portal-work-execution[open]')).toHaveCount(0);
 await page.getByRole('link',{name:'View records',exact:true}).click();
 await page.getByRole('button',{name:'Tool Gateway',exact:true}).click();
 await expect(page.getByRole('link',{name:'Mock tool call finished',exact:true})).toBeVisible();
 await expect(page.getByRole('link',{name:'Model request finished',exact:true})).toHaveCount(0);
});
test('live source submits canonical intent then polls actual result and narrowed authority',async({page},info)=>{
 let observed:ClaimRequest|undefined;
let done=false;
let first:View|undefined;
 await api(page,()=>first?[first]:[],body=>{observed=body;
first=work();
first.request=body;
first.requestRef=body.metadata.name;
first.state!.requestRef=first.requestRef;
first.state!.claim!.requestRef=first.requestRef;
first.facts.forEach(f=>f.requestRef=body.metadata.name);
});
 await page.goto('/?mode=connected#/work');
await expect(page.getByText('No work started in this session.')).toBeVisible();
 await page.getByRole('link',{name:'New work',exact:true}).click();
await expect(page.getByLabel('Model profile')).toHaveValue('approved-coding-model');
await page.getByLabel('What should it do?').fill('Explain a safe retry strategy');
await page.getByRole('button',{name:'Start work',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Explain a safe retry strategy',exact:true})).toBeVisible();
expect(observed?.spec.projectRef).toBe('payments');
expect(observed).not.toHaveProperty('principal');
expect(observed?.spec.requestedAccess?.tools).toEqual(['git.read','git.write']);
expect(observed?.spec.requestedAccess?.resourceScopes).toEqual(['repo:acme/payments']);
 await page.getByText('Access & limits',{exact:true}).click();
 await page.getByRole('link',{name:'Compare requested and granted'}).click();
await expect(page.locator('.portal-compare-side').last().getByText('git.write',{exact:true})).toHaveCount(0);
await expect(page.getByText('standard-isolated · 15m')).toBeVisible();
 expect(first).toBeDefined();
first!.state!.claim!.phase='Succeeded';
first!.outcome={status:'Succeeded',text:'Use a bounded retry with exponential backoff.',model:{invocationId:'model-1',model:'local-test-model',inputTokens:12,outputTokens:20}};
done=true;
 await page.getByRole('link',{name:'Explain a safe retry strategy',exact:true}).click();
await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible();
await expect(page.getByText('Use a bounded retry with exponential backoff.')).toBeVisible();
expect(done).toBe(true);
await page.screenshot({path:info.outputPath('connected-result.png'),fullPage:true});
});
test('new work suggests the active policy project instead of a demo constant',async({page})=>{
 let submitted:ClaimRequest|undefined;
 const billingSetup:Setup={...setup,policy:{...setup.policy,Rules:[{team:'team-a',action:'claim.create',project:'billing',templateRef:'engineer'}]}};
 await api(page,()=>[work()],body=>{submitted=body},billingSetup);
 await page.goto('/?mode=connected#/work/new');
 await expect(page.getByLabel('Project')).toHaveValue('billing');
 await page.getByLabel('What should it do?').fill('Explain retry backoff');
 await page.getByRole('button',{name:'Start work',exact:true}).click();
 await expect.poll(()=>submitted?.spec.projectRef).toBe('billing');
});
test('denial shows request evidence without invented claim worker or model result',async({page})=>{
 const denied=work();
denied.state!.principal={...state.principal,team:'team-b',subject:'user:team-b-engineer'};
denied.state!.decision={...state.decision,principalRef:'user:team-b-engineer',result:'Deny',reason:'Team B cannot start this work.'};
delete denied.state!.claim;
delete denied.state!.effectiveAuthority;
denied.facts=[{id:'denied-1',sequence:1,timestamp:'2026-09-15T02:00:00Z',kind:'RequestResolution',requestRef:denied.requestRef,decision:denied.state!.decision,result:'Deny',reasonCode:'assignment-deny'}];
 await api(page,()=>[denied]);
await page.goto(`/?mode=connected#/work/${denied.requestRef}`);
await page.getByText('Access & limits',{exact:true}).click();
await expect(page.getByText('No authority was issued.')).toBeVisible();
await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
await page.getByText('Execution details').click();
await expect(page.locator('.portal-details .portal-fact').filter({hasText:'Claim ID'})).toContainText('Not recorded');
});
test('pending live request never invents grant or result and malformed response stays unavailable',async({page})=>{
 const pending:View={version:'agenova.evidence/v0',requestRef:request.metadata.name,request,facts:[]};
await api(page,()=>[pending]);
await page.goto(`/?mode=connected#/work/${pending.requestRef}`);
await expect(page.getByText('Request received; waiting for authorization.')).toBeVisible();
await expect(page.getByRole('heading',{name:'Result'})).toHaveCount(0);
await expect(page.getByText('Update invoice retry tests')).toHaveCount(0);
 await page.unroute('**/api/**');
await page.route('**/api/**',route=>route.fulfill({json:{future:'invalid'}}));
await page.reload();
await expect(page.getByRole('alert')).toBeVisible();
await expect(page.getByText('Fix the payment timeout bug')).toHaveCount(0);
});
test('successful result remains visible when cleanup fails and configuration is not health',async({page},info)=>{
 const completed=work('Succeeded');
completed.outcome!.failure='Worker cleanup was not confirmed.';
await api(page,()=>[completed]);
await page.goto(`/?mode=connected#/work/${completed.requestRef}`);
await expect(page.getByText('Task completed; cleanup needs attention')).toBeVisible();
await expect(page.getByText(completed.outcome!.text!)).toBeVisible();
await page.goto('/?mode=connected#/platform');
await expect(page.getByRole('row',{name:/Tool Gateway/})).toContainText('Not connected');
await expect(page.getByText('Configuration is not a health check.',{exact:false})).toBeVisible();
await page.screenshot({path:info.outputPath('connected-platform.png'),fullPage:true});
});

test('terminal authority keeps refreshing until final result and cleanup report arrive', async ({page}) => {
  const finishing = work('Succeeded');
  delete finishing.outcome;
  await api(page, () => [finishing]);
  await page.goto(`/?mode=connected#/work/${finishing.requestRef}`);
  await expect(page.getByText('Waiting for the final result and cleanup evidence.')).toBeVisible();
  await expect(page.getByRole('heading', {name:'Result',exact:true})).toHaveCount(0);
  finishing.outcome = {status:'Succeeded',text:'Final observed task result.'};
  await expect(page.getByText('Final observed task result.')).toBeVisible();
});

test('queued cancellation overrides pending claim and stops automatic refresh', async ({page}) => {
  await page.clock.install();
  const cancelled = work();
  cancelled.state!.claim!.phase = 'Pending';
  delete cancelled.state!.claim!.backendIdentity;
  cancelled.state!.evidence.runtimeEvents = [];
  cancelled.facts = cancelled.facts.filter(fact => fact.kind !== 'Runtime');
  cancelled.outcome = {status:'Cancelled',failure:'Queued work expired before allocation.'};
  let evidenceReads = 0;
  page.on('request', request => {
    if (new URL(request.url()).pathname.endsWith('/evidence')) evidenceReads++;
  });
  await api(page, () => [cancelled]);
  await page.goto(`/?mode=connected#/work/${cancelled.requestRef}`);
  await expect(page.locator('.portal-head').getByText('Cancelled',{exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
  await expect(page.getByText('Queued work expired before allocation.').first()).toBeVisible();
  expect(evidenceReads).toBe(1);
  await page.clock.fastForward(5000);
  expect(evidenceReads).toBe(1);
});
