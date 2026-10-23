// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect,test,type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import type { ClaimRequest,IssuedState,AgentTemplate,Fact } from '../src/contracts.generated';
import type { Setup,View } from '../src/connected-source';
const request=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json',import.meta.url),'utf8')) as ClaimRequest;
const state=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json',import.meta.url),'utf8')) as IssuedState;
const template:AgentTemplate={apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{artifact:{image:'example-worker'},entrypoint:{command:['worker']},defaults:{modelProfile:'approved-coding-model',memoryScopes:['team-docs']},capabilityCeiling:{tools:['git.read','git.write'],resourceScopes:['repo:acme/payments'],modelProfiles:['approved-coding-model'],memoryScopes:['team-docs'],runtimeProfiles:['standard-isolated'],maxTimeout:'30m'}}};
const setup:Setup={installation:{kind:'installed',platform:'reference-kind',revision:'sha256:'+'a'.repeat(64)},principal:state.principal,template,policy:{ID:state.policyRef.id,Version:state.policyRef.version,Rules:[{team:'team-a',action:'claim.create',project:'payments',templateRef:'engineer'}]},capabilities:{taskSubmission:'ready',runtime:'configured',model:'configured',tool:'notConnected',memory:'notConnected'}};
function work(phase:'Running'|'Succeeded'='Running'):View{
 const copy=structuredClone(state);
copy.claim!.phase=phase;
copy.effectiveAuthority!.tools=['git.read'];
copy.effectiveAuthority!.runtime.timeout='15m';
 const facts:Fact[]=[{id:'f1',sequence:1,timestamp:'2026-09-15T02:00:00Z',kind:'RequestResolution',requestRef:copy.requestRef,decision:copy.decision,result:'Allow',reasonCode:'assignment-allow'},{id:'f2',sequence:2,timestamp:'2026-09-15T02:00:01Z',kind:'AuthorityResolved',requestRef:copy.requestRef,claimId:copy.claim!.id,effectiveAuthority:copy.effectiveAuthority,reasonCode:'template-policy-intersection'},{id:'f3',sequence:3,timestamp:'2026-09-15T02:00:02Z',kind:'Runtime',requestRef:copy.requestRef,claimId:copy.claim!.id,operation:phase}];
 return{version:'agenova.evidence/v0',requestRef:copy.requestRef,request:structuredClone(request),state:copy,facts,...(phase==='Succeeded'?{outcome:{status:'Succeeded',text:'Use a bounded retry with exponential backoff.',model:{invocationId:'model-1',model:'local-test-model',inputTokens:12,outputTokens:20}}}:{})};
}
async function inspectActivity(page:Page){const details=page.locator('.portal-work-evidence');if(await details.getAttribute('open')===null)await details.locator(':scope > summary').click();}
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

for (const online of [false, true]) {
 test(`network access is explicit intent, never inherited from template: online=${online}`, async ({page}, info) => {
  const configured=structuredClone(setup);
  configured.template!.spec.capabilityCeiling!.networkProfiles=['public-web'];
  let submitted:ClaimRequest|undefined;
  let current=work('Succeeded');
  await api(page,()=>[current],request=>{
   submitted=request;
   current={...current,request,requestRef:request.metadata.name};
   current.state!.effectiveAuthority!.networkProfiles=request.spec.requestedAccess?.networkProfiles;
  },configured);
  await page.goto('/?mode=connected#/work/new');
  await expect(page.getByLabel('Public network access')).toHaveValue('');
  if(online) await page.getByLabel('Public network access').selectOption('public-web');
  await page.getByLabel('What should it do?').fill('Investigate using the explicitly requested access only.');
  await page.getByRole('button',{name:'Start work',exact:true}).click();
  await expect.poll(()=>submitted?.spec.task?.input?.objective).toBe('Investigate using the explicitly requested access only.');
  expect(submitted?.spec.requestedAccess?.networkProfiles).toEqual(online?['public-web']:undefined);
  await inspectActivity(page);await page.getByText('Access & limits',{exact:true}).click();
  await expect(page.getByText('Network profiles',{exact:true}).first()).toBeVisible();
  if(online) await expect(page.locator('code').filter({hasText:'public-web'}).first()).toBeVisible();
  await page.screenshot({path:info.outputPath(`network-authority-${online?'online':'offline'}.png`),fullPage:true});
 });
}
test('Work details shows requested permissions first; only ungranted requests are gray',async({page},info)=>{
 const current=work('Succeeded');current.request.spec.requestedAccess={tools:['git.read','git.write'],resourceScopes:['repo:acme/payments'],modelProfile:'approved-coding-model'};
 current.state!.effectiveAuthority!.memoryScopes=['unrequested-memory'];current.state!.effectiveAuthority!.networkProfiles=['unrequested-network'];
 await api(page,()=>[current]);await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 const table=page.getByRole('table',{name:'Requested and granted task permissions'});await expect(table).toBeVisible();
 await expect(table.locator('tbody tr')).toHaveCount(6);await expect(table).not.toContainText('unrequested');await expect(table).not.toContainText('Memory');await expect(table).not.toContainText('Network');
 const denied=table.locator('tr[data-permission="tools:git.write"]');await expect(denied.locator('td').last()).toHaveText('Not granted');
 await expect(denied.locator('td').first()).toHaveCSS('color','rgb(36, 35, 53)');await expect(denied.locator('td').last()).toHaveCSS('color','rgb(116, 109, 133)');
 const allowed=table.locator('tr[data-permission-state=granted] td');for(const value of await allowed.all()){await expect(value).toHaveCSS('color','rgb(36, 35, 53)');await expect(value).toHaveCSS('font-weight','400');}
 await expect(page.locator('.portal-work-evidence')).not.toHaveAttribute('open');await expect(page.getByText('Public web access goes through the Tool Gateway.',{exact:true})).toHaveCount(0);
 await expect(page.getByRole('region',{name:'Work details'})).toContainText(state.policyRef.id);await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible();
 await page.screenshot({path:info.outputPath('work-permissions-focused.png'),fullPage:true});
 await inspectActivity(page);await expect(page.getByRole('heading',{name:'Work status',exact:true})).toBeVisible();
});
for(const failed of [true,false]){
 test(`recorded refusal highlights above permissions only for failed Work: failed=${failed}`,async({page},info)=>{
  const current=work('Succeeded');
  if(failed){current.state!.claim!.phase='Failed';current.outcome={status:'Failed',failure:'Tool invocation denied.'};}
  current.facts.push({id:'refused-tool',sequence:4,timestamp:'2026-10-23T00:00:00Z',kind:'ToolDecision',requestRef:current.requestRef,target:'web.read',result:'Deny',reason:'Tool not granted to this Work.'});
  await api(page,()=>[current]);await page.goto(`/?mode=connected#/work/${current.requestRef}`);
  const refusal=page.locator('.portal-work-refusals');await expect(refusal).toHaveCount(1);
  const before=await refusal.evaluate(el=>!!(el.compareDocumentPosition(document.querySelector('.portal-work-permissions')!)&Node.DOCUMENT_POSITION_FOLLOWING));
  expect(before).toBe(failed);await expect(refusal.locator('.portal-blocked-event')).toHaveCount(1);
  if(failed){
   await expect(refusal).toHaveAttribute('role','alert');await expect(refusal).toHaveText('web.read blockedEvidence');
   await expect(refusal.locator('.portal-blocked-event')).toHaveCSS('background-color','rgb(255, 247, 230)');
   await expect(refusal.locator('span')).toHaveCSS('font-weight','500');
   await expect(refusal.locator('svg')).toHaveCount(0);
   await expect(refusal.getByRole('link',{name:'Evidence',exact:true})).toHaveAttribute('href',`#/work/${encodeURIComponent(current.requestRef)}/activity/refused-tool`);
   for(const width of [1100,390,320]){await page.setViewportSize({width,height:1000});expect(await refusal.evaluate(el=>el.scrollWidth<=el.clientWidth)).toBe(true);await expect(refusal.getByRole('link',{name:'Evidence',exact:true})).toBeVisible();}
   await page.setViewportSize({width:1100,height:1000});await page.screenshot({path:info.outputPath('failed-refusal-banner.png'),fullPage:true});
   await refusal.getByRole('link',{name:'Evidence',exact:true}).click();await expect(page.getByRole('heading',{name:'What this record says',exact:true})).toBeVisible();
  }else{
   await expect(page.locator('.portal-work-refusals-highlight')).toHaveCount(0);await expect(refusal).toHaveText('Blocked · web.readView evidence');
   await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible();
  }
 });
}
test('ordinary execution failure without a refusal never fabricates a blocked banner',async({page})=>{
 const current=work();current.state!.claim!.phase='Failed';current.outcome={status:'Failed',failure:'Provider unavailable.'};
 await api(page,()=>[current]);await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-work-failure')).toContainText('Provider unavailable.');
 await expect(page.locator('.portal-work-refusals')).toHaveCount(0);
});
test('connected chart and profile use only current API records, and clear on connection failure', async ({ page }, info) => {
 const current=work('Succeeded');
 // A later provider call must not become another request or move its start hour.
 current.facts.push({id:'late-model',sequence:4,timestamp:'2026-10-20T23:00:00Z',kind:'ProviderOutcome',requestRef:current.requestRef,providerStatus:'Succeeded'});
 await api(page,()=>[current]);
 await page.goto('/?mode=connected#/work');
 await expect(page.getByRole('img')).toHaveAttribute('aria-label',/1 loaded requests on 2026-09-15/);
 await page.getByLabel('Open profile menu',{exact:true}).click();
 const account=page.locator('.portal-topbar .portal-profile-popover');
 await expect(account).toContainText(setup.principal.subject);
 await expect(account).toContainText(setup.principal.team);
 await expect(account).not.toContainText('Team A engineer');
 await page.keyboard.press('Escape');
 await page.screenshot({path:info.outputPath('june-complete-connected.png'),fullPage:true});
 await page.route('**/api/**',route=>route.fulfill({status:503,json:{code:'unavailable'}}));
 await expect(page.getByRole('alert')).toContainText('unavailable');
 await page.getByLabel('Open profile menu',{exact:true}).click();
 await expect(account).toContainText('Not connected');
 await expect(account).not.toContainText(setup.principal.subject);
 await expect(page.getByRole('img')).toHaveCount(0);
});

test('historical missing network evidence stays unknown rather than retroactively offline', async ({page}) => {
 const historical=work('Succeeded');
 await api(page,()=>[historical]);
 await page.goto(`/?mode=connected#/work/${encodeURIComponent(historical.requestRef)}`);
 await inspectActivity(page);await page.getByText('Access & limits',{exact:true}).click();
 await expect(page.getByText('Not recorded; runtime boundary unknown',{exact:true})).toBeVisible();
 await expect(page.getByText('No public network granted',{exact:true})).toHaveCount(0);
});
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
 await inspectActivity(page);await page.getByText('Execution details',{exact:true}).click();
 await page.getByRole('button',{name:'Copy request ID',exact:true}).click();
 await expect(page.locator('.portal-copy-reference')).toContainText('Copied');
 expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe(current.requestRef);
 await page.goto('/?mode=connected#/work');
 await expect(page.locator('.portal-work-title')).toHaveText('Payment retry investigation');
 await page.getByLabel('Search work').fill('recommend a specific fix');
 await expect(page.locator('.portal-work-title')).toHaveCount(1);
 await page.screenshot({path:info.outputPath('named-work-list.png'),fullPage:true});
});

test('connected Portal rejects an unrelated local demo API', async ({page}) => {
 await api(page,()=>[]);
 await page.route('**/api/setup', route => route.fulfill({json:{...setup,installation:{kind:'local-demo'}}}));
 await page.goto('/?mode=connected#/work');
 await expect(page.getByRole('alert')).toContainText('not an installed Agenova Platform');
 await expect(page.getByRole('link',{name:'New work'})).toHaveCount(0);
});
test('connected Work polling does not repeatedly query Platform setup', async ({page}) => {
 let setupCalls=0;
 let listCalls=0;
 await api(page,()=>[work()]);
 await page.route('**/api/setup', route=>{setupCalls++;return route.fulfill({json:setup});});
 await page.route('**/api/requests', route=>{listCalls++;return route.fulfill({json:[work()]});});
 await page.goto('/?mode=connected#/work');
 await expect.poll(()=>listCalls,{timeout:6000}).toBeGreaterThanOrEqual(2);
 // Development StrictMode may mount the effect more than once. Its initial
 // setup reads may repeat, but subsequent Work polls must not add more.
 const initialSetupCalls=setupCalls;
 await expect.poll(()=>listCalls,{timeout:6000}).toBeGreaterThanOrEqual(5);
 expect(setupCalls).toBe(initialSetupCalls);
});
test('idle connected setup is revalidated without one-second Kubernetes polling', async ({page}) => {
 test.setTimeout(45_000);
 let setupCalls=0;
 let available=true;
 await api(page,()=>[work('Succeeded')]);
 await page.route('**/api/setup', route=>{
   setupCalls++;
   return available ? route.fulfill({json:setup}) : route.fulfill({status:503,json:{code:'unavailable'}});
 });
 await page.goto('/?mode=connected#/work');
 await expect.poll(()=>setupCalls).toBeGreaterThan(0);
 await expect(page.locator('.portal-work-title')).toHaveCount(1);
 const initial=setupCalls;
 available=false;
 await expect.poll(()=>setupCalls,{timeout:35_000,intervals:[1000]}).toBeGreaterThan(initial);
 await expect(page.getByRole('alert')).toContainText('connection is unavailable');
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
 await inspectActivity(page);await expect(page.locator('.portal-agent-failure')).toContainText(reason);
 await expect(page.locator('.portal-agent-failure strong')).toHaveCSS('color','rgb(177, 65, 83)');
 await expect(page.locator('.portal-worker-state.positive')).toHaveText('Succeeded');
 await expect(page.locator('.portal-worker-state.negative')).toHaveCount(0);
 await page.screenshot({path:info.outputPath('work-outcome-failed.png'),fullPage:true});
 await page.getByRole('link',{name:'View failure record'}).click();
 await expect(page.getByRole('heading',{name:'Work failed',level:1,exact:true})).toBeVisible();
 await expect(page.locator('.portal-head .portal-badge.failed')).toHaveText('Failed');
 // Wait for record navigation; the previous Work page also has a Failed badge.
 await expect(page.locator('.portal-state.failed[role="alert"]')).toContainText(reason);
 await page.screenshot({path:info.outputPath('failed-record-detail.png'),fullPage:true});
 await page.getByRole('link',{name:'Activity',exact:true}).click();
 await expect(page.locator('.portal-record-row').filter({hasText:'Work failed'}).locator('.portal-badge.failed')).toHaveCount(2);
});
test('connected June explorer and inspector use only loaded API evidence', async ({page},info) => {
 const current=work();
 current.facts.push(
  {id:'attempt',sequence:4,timestamp:'2026-09-15T02:00:03Z',kind:'ProviderAttempt',requestRef:current.requestRef,invocationId:'m',operation:'model.invoke',target:'approved-coding-model'},
 );
 await api(page,()=>[current]);
 await page.setViewportSize({width:1440,height:1000});
 await page.goto('/?mode=connected#/work');
 await expect(page.getByLabel('Loaded work summary').locator('strong')).toHaveText(['1','0','0','0']);
 const row=page.locator('.portal-table tbody tr');
 await expect(row).toHaveCount(1);
 await expect(row).toContainText(state.principal.subject);
 await expect(row).toContainText('Allow');
 await page.screenshot({path:info.outputPath('june-connected-list.png'),fullPage:true});
 await page.locator('.portal-work-title').click();
 await inspectActivity(page);const inspect=page.getByRole('button',{name:'Inspect Model request: approved-coding-model'});
 await inspect.click();
 const inspector=page.getByRole('complementary',{name:'Selected activity'});
 await expect(inspector).toContainText('Waiting for response');
 await expect(inspector).not.toContainText('Succeeded');
 await expect(inspector.getByRole('link')).toHaveAttribute('href',new RegExp(`/activity/attempt$`));
 await page.screenshot({path:info.outputPath('june-connected-detail.png'),fullPage:true});
 current.facts.push({id:'completion',sequence:5,timestamp:'2026-09-15T02:00:04Z',kind:'ProviderOutcome',requestRef:current.requestRef,invocationId:'m',operation:'model.invoke',providerStatus:'Succeeded'});
 await expect(inspector).toContainText('Succeeded');
 await expect(inspector.getByRole('link')).toHaveAttribute('href',new RegExp(`/activity/completion$`));
 await expect(inspect).toHaveAttribute('aria-pressed','true');
 await inspector.getByRole('link').click();
 await expect(page.getByRole('heading',{name:'Model request finished',exact:true})).toBeVisible();
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
 await inspectActivity(page);
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
 await inspectActivity(page);await expect(page.locator('.portal-flow-node.active')).toContainText('Agent');
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

test('June feedback locates real pending calls and separates success from failure',async({page},info)=>{
 const current=work();
 const add=(kind:string,extra:Partial<Fact>={})=>current.facts.push({id:`signal-${current.facts.length}`,sequence:current.facts.length+1,timestamp:'2026-09-15T02:00:03Z',kind,requestRef:current.requestRef,...extra});
 add('WorkerActivity',{operation:'TurnStarted',target:'Turn 1'});
 add('ProviderAttempt',{invocationId:'signal-model',operation:'model.invoke'});
 await api(page,()=>[current]);
 const cdp=await page.context().newCDPSession(page);
 await cdp.send('Emulation.setCPUThrottlingRate',{rate:4});
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await inspectActivity(page);
 await expect(page.locator('.portal-worker')).toContainText('Waiting for a model response');
 expect(await page.locator('.portal-shell').evaluate(el=>getComputedStyle(el).getPropertyValue('--portal-paper').trim())).toBe('#faf9fc');
 await expect(page.locator('.portal-page > .portal-head .portal-badge')).toHaveCSS('color','rgb(99, 66, 199)');
 const row=page.locator('.portal-worker-actions li[data-active=true]');
 await expect(row).toHaveAttribute('data-kind','model');
 await expect(row.locator('.portal-worker-lamp')).toHaveCount(1);
 await expect(page.locator('.portal-worker-current .portal-worker-lamp')).toHaveCount(0);
 await expect(row.locator('a')).toHaveCSS('text-decoration-line','none');
 await expect(row.locator('.portal-worker-lamp')).toHaveCSS('animation-name','portal-worker-wait');
 await page.locator('.portal-turn > summary').hover();
 await expect(page.locator('.portal-turn > summary')).toHaveCSS('color','rgb(99, 66, 199)');
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
 await info.attach('june-wait-performance',{body:JSON.stringify(frameReport,null,2),contentType:'application/json'});
 await page.screenshot({path:info.outputPath('june-model-wait.png'),fullPage:true});
 add('ProviderOutcome',{invocationId:'signal-model',operation:'model.invoke',providerStatus:'Succeeded'});
 add('ProviderAttempt',{invocationId:'signal-tool',operation:'tool.invoke',reasonCode:'mock-tool',target:'Mock git.read'});
 await expect(row).toHaveAttribute('data-kind','tool');
 expect(await row.evaluate(el=>getComputedStyle(el).getPropertyValue('--call-light').trim())).toBe('#36827e');
 await expect(page.locator('.portal-worker-state.positive')).toHaveCSS('color','rgb(33, 124, 96)');
 await page.screenshot({path:info.outputPath('june-tool-wait.png'),fullPage:true});
 add('ProviderOutcome',{invocationId:'signal-tool',operation:'tool.invoke',reasonCode:'mock-artifact-not-found',providerStatus:'Failed'});
 current.state!.claim!.phase='Failed';current.outcome={status:'Failed',failure:'The recorded mock tool call failed.'};
 await expect(page.locator('.portal-worker-state.negative')).toHaveCSS('color','rgb(177, 65, 83)');
 await expect(page.locator('.portal-turn[data-negative=true] > summary')).toContainText('1 failed');
 await expect(page.locator('.portal-worker')).not.toHaveClass(/has-active-call/);
 expect(await page.evaluate(()=>document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity).length)).toBe(0);
 await page.screenshot({path:info.outputPath('june-failure.png'),fullPage:true});
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
 await inspectActivity(page);
 await expect(page.locator('.portal-worker-current')).toContainText('Turn 1 · Waiting for a model response');
 add('ProviderOutcome',{invocationId:'m1',operation:'model.invoke',providerStatus:'Succeeded'});
 add('ToolDecision',{invocationId:'t1',operation:'tool.invoke',result:'Allow'});
 add('ProviderAttempt',{invocationId:'t1',operation:'tool.invoke',reasonCode:'mock-tool',target:'Mock git.read · logs/timeout.log'});
 await expect(page.locator('.portal-worker-current')).toContainText('Waiting for tool observation');
 await expect(page.locator('.portal-worker-actions li[data-active=true]')).toContainText('Tool call (mock)');
 add('ProviderOutcome',{invocationId:'t1',operation:'tool.invoke',reasonCode:'mock-tool',providerStatus:'Succeeded'});
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
await expect(page.getByText('No work recorded yet.')).toBeVisible();
 await page.getByRole('link',{name:'New work',exact:true}).click();
await expect(page.getByLabel('Model profile')).toHaveValue('approved-coding-model');
await page.getByRole('checkbox',{name:'git.read',exact:true}).check();
await page.getByRole('checkbox',{name:'git.write',exact:true}).check();
await page.getByLabel('Resource scopes',{exact:true}).fill('repo:acme/payments');
await page.getByLabel('What should it do?').fill('Explain a safe retry strategy');
await page.getByRole('button',{name:'Start work',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Explain a safe retry strategy',exact:true})).toBeVisible();
expect(observed?.spec.projectRef).toBe('payments');
expect(observed).not.toHaveProperty('principal');
expect(observed?.spec.requestedAccess?.tools).toEqual(['git.read','git.write']);
expect(observed?.spec.requestedAccess?.resourceScopes).toEqual(['repo:acme/payments']);
 await inspectActivity(page);await page.getByText('Access & limits',{exact:true}).click();
 await page.getByRole('link',{name:'Compare requested and granted',exact:true}).click();
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
await inspectActivity(page);await page.getByText('Access & limits',{exact:true}).click();
await expect(page.getByText('No authority was issued.')).toBeVisible();
await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
await page.getByText('Execution details').click();
await expect(page.locator('.portal-details .portal-fact').filter({hasText:'Claim ID'})).toContainText('Not recorded');
});
test('pending live request never invents grant or result and malformed response stays unavailable',async({page})=>{
 const pending:View={version:'agenova.evidence/v0',requestRef:request.metadata.name,request,facts:[]};
await api(page,()=>[pending]);
await page.goto(`/?mode=connected#/work/${pending.requestRef}`);
await inspectActivity(page);await expect(page.getByText('Request received; waiting for authorization.')).toBeVisible();
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
await expect(page.locator('.product-config-card').filter({hasText:'Tool Gateway'})).toContainText('Not configured');
await expect(page.getByText('Configuration ≠ verified availability',{exact:true}).first()).toBeVisible();
await page.screenshot({path:info.outputPath('connected-platform.png'),fullPage:true});
});

test('terminal authority keeps refreshing until final result and cleanup report arrive', async ({page}) => {
  const finishing = work('Succeeded');
  delete finishing.outcome;
  await api(page, () => [finishing]);
  await page.goto(`/?mode=connected#/work/${finishing.requestRef}`);
  await inspectActivity(page);
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

test('recovered interrupted work shows unknown model outcome without fabricated success',async({page},info)=>{
 const current=work();current.state!.claim!.phase='Failed';
 const reason='The service was interrupted. Runtime release was verified; uncertain external operations were not replayed.';
 current.outcome={status:'Failed',failure:reason};
 const runtimeFact=(operation:string):Fact=>({id:`runtime-${operation}`,sequence:0,timestamp:'2026-10-12T00:00:00Z',kind:'Runtime',requestRef:current.requestRef,claimId:current.state!.claim!.id,backendIdentity:current.state!.claim!.backendIdentity,operation});
 current.facts=[
  {id:'received',sequence:0,timestamp:'2026-10-12T00:00:00Z',kind:'RequestReceived',requestRef:current.requestRef},
  ...current.facts.filter(f=>f.kind!=='Runtime'),
  ...['Bound','BackendReady','Running'].map(runtimeFact),
  {id:'model-allow',sequence:0,timestamp:'2026-10-12T00:00:00Z',kind:'ModelDecision',requestRef:current.requestRef,claimId:current.state!.claim!.id,invocationId:'model-unknown',result:'Allow',operation:'model.invoke',target:'approved-coding-model'},
 ];
 current.facts.push(
  {id:'attempt-unknown',sequence:4,timestamp:'2026-10-12T00:00:00Z',kind:'ProviderAttempt',requestRef:current.requestRef,operation:'model.invoke',invocationId:'model-unknown',target:'approved-coding-model',providerStatus:'Attempted'},
  ...['Failed','TerminateSucceeded','CleanupSucceeded'].map(runtimeFact),
  {id:'interrupted',sequence:5,timestamp:'2026-10-12T00:00:01Z',kind:'RunOutcome',requestRef:current.requestRef,operation:'Failed',reason,reasonCode:'run-recovered-interruption'},
 );
 current.facts=current.facts.map((f,i)=>({...f,sequence:i+1}));
 current.state!.evidence.runtimeEvents=['Bound','BackendReady','Running','Failed','TerminateSucceeded','CleanupSucceeded'].map(kind=>({kind}));
 await api(page,()=>[current]);
 await page.goto(`/?mode=connected#/work/${current.requestRef}`);
 await expect(page.locator('.portal-worker-state')).toHaveText('Outcome unknown');
 await expect(page.locator('.portal-agent-failure')).toContainText(reason);
 await expect(page.locator('.portal-worker-state.positive')).toHaveCount(0);
 await expect(page.getByText('Waiting for response',{exact:true})).toHaveCount(0);
 await page.screenshot({path:info.outputPath('recovered-work.png'),fullPage:true});
});

test('scoped configuration uses server-owned project and template selectors',async({page},info)=>{
 await api(page,()=>[]);
 await page.route('**/api/setup*',route=>{
  const query=new URL(route.request().url()).searchParams;
  const project=query.get('project') || 'payments';
  const name=query.get('template') || 'engineer';
  return route.fulfill({json:{...setup,project,projects:['payments','analytics'],templates:['engineer','reviewer'],template:{...template,metadata:{name}},policy:{ID:project+'-policy',Version:'2',Rules:[{team:'team-a',action:'claim.create',project,templateRef:name}]}}});
 });
 await page.goto('/?mode=connected#/policy');
 await expect(page.getByText('payments-policy',{exact:true})).toBeVisible();
 await expect(page.getByLabel('Configuration project')).toHaveValue('payments');
 await expect(page.getByLabel('Configuration template')).toHaveCount(0);
 await page.goto('/?mode=connected&project=analytics&template=reviewer#/policy');
 await expect(page.getByText('analytics-policy',{exact:true})).toBeVisible();
 await expect(page.locator('.portal-rule')).toContainText('reviewer');
 await page.screenshot({path:info.outputPath('scoped-policy.png'),fullPage:true});
});

test('member without a template can still read permitted Work evidence',async({page})=>{
 await api(page,()=>[work('Succeeded')],undefined,{...setup,template:null});
 await page.goto('/?mode=connected#/work');
 await expect(page.locator('.portal-work-title')).toHaveCount(1);
 await page.goto('/?mode=connected#/work/new');
 await expect(page.getByText('No approved template is available in this project.',{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();
});

test('durable history paginates server summaries and keeps the Work navigation active', async ({page}) => {
 const current=work('Succeeded'); await api(page,()=>[current]);
 await page.route('**/api/history?**',async route=>{
  const next=new URL(route.request().url()).searchParams.get('cursor');
  await route.fulfill({json:{items:[{requestRef:next?'historical-work':current.requestRef,taskType:'investigation',status:next?'Failed':'Succeeded',created:'2026-10-20T00:00:00Z',updated:'2026-10-20T00:01:00Z'}],...(next?{}:{nextCursor:'opaque-next-page'})}});
 });
 await page.goto('/?mode=connected#/history');
 await expect(page.getByRole('heading',{name:'Work history',exact:true})).toBeVisible();
 await expect(page.locator('.portal-sidebar').getByRole('link',{name:'Work',exact:true})).toHaveClass('active');
 await expect(page.getByRole('link',{name:current.requestRef,exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Next page',exact:true}).click();
 await expect(page.getByRole('link',{name:'historical-work',exact:true})).toBeVisible();
 await expect(page.getByRole('link',{name:current.requestRef,exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'Newest work',exact:true}).click();
 await expect(page.getByRole('link',{name:current.requestRef,exact:true})).toBeVisible();
});

test('restart-interrupted evidence never renders historical Running authority as active', async ({page}) => {
 const interrupted:View={...work(),recovery:{code:'service-restarted',cleanup:'Unknown'},diagnosis:{stage:'recovery',code:'provider-outcome-unknown'}};
 await api(page,()=>[interrupted]);
 await page.goto(`/?mode=connected#/work/${interrupted.requestRef}`);
 await inspectActivity(page);await expect(page.getByText('Execution interrupted by service restart',{exact:true})).toBeVisible();
 await expect(page.locator('.portal-head').getByText('Interrupted',{exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'Active access',exact:true})).toHaveCount(0);
 await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
 await inspectActivity(page);await page.getByText('Access & limits',{exact:true}).click();
 await expect(page.getByRole('heading',{name:'Issued access',exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'Active access',exact:true})).toHaveCount(0);
 await expect(page.getByRole('link',{name:'Open configured trace',exact:true})).toHaveCount(0);
});
