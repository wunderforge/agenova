// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect,type Page} from '@playwright/test';
// The model-planner composition has its own suite; these are explicit rules contracts.
test.skip(process.env.AGENOVA_TEST_LOCAL_ADVISOR==='1','Rules fixture suite; real-model.spec.ts covers the model-planner composition.');

async function openTask(page:Page,mode:string,type:string,objective:string){
 await page.goto('/login');await expect(page.getByRole('heading',{name:'New work',exact:true})).toBeVisible();
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption(mode);
 await expect(page.getByRole('combobox',{name:'Task type',exact:true})).toHaveCount(0);
 await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill(objective);
}
test('manual submits explicit permissions without planning',async({page})=>{
 await openTask(page,'manual','public-research','Read https://jitsi.org/security/ and summarize.');
 const before=await (await page.request.get('/api/setup?project=research&template=researcher-advisor-v1')).json();
 await page.getByLabel('web.read',{exact:true}).check();await page.getByLabel('Resource scopes',{exact:true}).fill('web:public-pages');
 const response=page.waitForResponse(r=>r.url().endsWith('/api/requests')&&r.request().method()==='POST');
 await page.getByRole('button',{name:'Start work',exact:true}).click();const view=await (await response).json();
 expect(view.request.spec.requestedAccess.tools).toEqual(['web.read']);expect(view.accessAdvice).toBeUndefined();
 const after=await (await page.request.get('/api/setup?project=research&template=researcher-advisor-v1')).json();expect(after.accessAdvisor.callsUsed).toBe(before.accessAdvisor.callsUsed);
 await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
});
// Rules remain an explicit upstream/API workflow. No hidden UI category is reintroduced.
async function rulesIntent(page:Page,operation:string,objective:string){
 const setup=await(await page.request.get('/api/setup?project=research')).json();
 const profile=setup.template.spec.accessGuide.profiles.find((p:any)=>p.requestedAccess.tools?.includes(operation));expect(profile).toBeTruthy();
 return {apiVersion:'agenova.io/v1alpha1',kind:'ClaimRequest',metadata:{name:`rules-api-${crypto.randomUUID()}`},spec:{projectRef:'research',templateRef:setup.template.metadata.name,task:{type:profile.taskType,input:{objective}},requestedAccess:{},runtime:{profileRef:setup.template.spec.capabilityCeiling.runtimeProfiles[0],timeout:'5m'}}};
}

test('explicit typed rules API suggests without allocation and confirms through ordinary admission',async({page})=>{
 await page.goto('/login');const request=await rulesIntent(page,'web.read','Read https://jitsi.org/security/ and summarize.');
 const before=await(await page.request.get('/api/requests')).json();
 const proposed=await page.request.post('/api/access-advice',{data:{mode:'suggest',request}});expect(proposed.ok()).toBe(true);const plan=await proposed.json();expect(plan.status).toBe('ready');expect(plan.engine).toBe('rules');expect(plan.request.spec.requestedAccess.tools).toEqual(['web.read']);
 expect((await(await page.request.get('/api/requests')).json()).length).toBe(before.length);
 const accepted=await page.request.post('/api/requests',{data:plan.request,headers:{'X-Agenova-Advice-ID':plan.id}});expect(accepted.status()).toBe(202);const v=await accepted.json();expect(v.accessAdvice.mode).toBe('suggest');expect(v.accessAdvice.engine).toBe('rules');
 await page.goto(`/?mode=connected#/work/${v.requestRef}`);await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
});

for(const width of [1440,390]){
 test(`task precedes adjacent permission choices and stays uncluttered at ${width}px`,async({page})=>{
  await page.setViewportSize({width,height:1050});
  await openTask(page,'manual','public-research','Read the supplied public page.');
  const objective=page.getByRole('textbox',{name:'What should it do?',exact:true});
  const permissions=page.getByRole('region',{name:'Permissions',exact:true});
  const method=permissions.getByRole('combobox',{name:'Permission setup',exact:true});
  const tool=permissions.getByLabel('web.read',{exact:true});
  const taskBounds=(await objective.boundingBox())!,methodBounds=(await method.boundingBox())!,toolBounds=(await tool.boundingBox())!;
  expect(taskBounds.y+taskBounds.height).toBeLessThan(methodBounds.y);
  expect(toolBounds.y-methodBounds.y-methodBounds.height).toBeLessThan(100);
  await expect(page.locator('.portal-request-route')).toHaveCount(0);
  await method.selectOption('automatic');await expect(tool).toBeHidden();await expect(objective).toHaveValue('Read the supplied public page.');
  await expect(permissions).toContainText('Permissions are planned when you start.');
  await method.selectOption('manual');await expect(tool).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.getByRole('heading',{name:'Permissions',exact:true}).click();
  await page.screenshot({path:test.info().outputPath(`new-work-manual-${width}.png`),fullPage:true});
 });
}
test('explicit typed rules API supports automatic submission without confirmation',async({page})=>{
 await page.goto('/login');const request=await rulesIntent(page,'crm.read','Find customers with the largest usage declines in the customer-retention dataset.');
 const response=await page.request.post('/api/access-submit',{data:{mode:'automatic',request}});expect(response.status()).toBe(202);const view=await response.json();
 expect(view.accessAdvice.mode).toBe('automatic');expect(view.request.spec.requestedAccess.tools).toEqual(['crm.read']);expect(view.request.spec.requestedAccess.resourceScopes).toEqual(['crm:customer-retention']);
 await page.goto(`/?mode=connected#/work/${view.requestRef}`);await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
 await page.getByText('Permission preparation',{exact:true}).click();await expect(page.getByText('Automatic',{exact:true})).toBeVisible();
});

test('untyped rules task remains unstarted without guessing or requiring a removed UI preset',async({page})=>{
 await openTask(page,'automatic','investigation','Do something useful.');const before=await(await page.request.get('/api/requests')).json();
 await page.getByRole('button',{name:'Plan and start work',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();expect((await(await page.request.get('/api/requests')).json()).length).toBe(before.length);
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('suggest');await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Read https://jitsi.org/security/');
 await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();await expect(page.getByRole('heading',{name:'Suggested permissions'})).toHaveCount(0);
 expect((await(await page.request.get('/api/requests')).json()).length).toBe(before.length);
});
