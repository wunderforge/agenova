// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect,type Page} from '@playwright/test';

async function openTask(page:Page,mode:string,type:string,objective:string){
 await page.goto('/login');await expect(page.getByRole('heading',{name:'New work',exact:true})).toBeVisible();
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption(mode);
 if(mode!=='manual'&&type==='public-research')await page.getByRole('combobox',{name:'Task preset',exact:true}).selectOption('web');
 if(mode!=='manual'&&type==='customer-analysis')await page.getByRole('combobox',{name:'Task preset',exact:true}).selectOption('crm');
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
test('suggestion is reviewable and does not allocate before confirmation',async({page})=>{
 await openTask(page,'suggest','public-research','Read https://jitsi.org/security/ and summarize.');
 const before=await (await page.request.get('/api/requests')).json();
 await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();await expect(page.getByRole('heading',{name:'Suggested permissions'})).toBeVisible();
 expect((await (await page.request.get('/api/requests')).json()).length).toBe(before.length);
 await expect(page.getByLabel('web.read',{exact:true})).toBeChecked();
 await expect(page.getByText('web.read',{exact:true})).toHaveCount(1);
 const reason=page.locator('.portal-advice-reason');await expect(reason).not.toHaveAttribute('open','');
 await reason.getByText('Why these permissions?',{exact:true}).click();await expect(reason.locator('p')).toBeVisible();
 await page.screenshot({path:test.info().outputPath('suggestion.png'),fullPage:true});
 const response=page.waitForResponse(r=>r.url().endsWith('/api/requests')&&r.request().method()==='POST');
 await page.getByRole('button',{name:'Start work',exact:true}).click();const view=await (await response).json();expect(view.accessAdvice.mode).toBe('suggest');expect(view.accessAdvice.engine).toBe('rules');
 await expect(page.getByText('Permission preparation',{exact:true})).toBeVisible();
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
test('automatic plans and starts CRM in one submission without confirmation',async({page})=>{
 await openTask(page,'automatic','customer-analysis','Find customers with the largest usage declines in the customer-retention dataset.');
 await expect(page.getByLabel('crm.read',{exact:true})).toBeHidden();
 const response=page.waitForResponse(r=>r.url().endsWith('/api/access-submit')&&r.request().method()==='POST');
 await page.getByRole('button',{name:'Plan and start work',exact:true}).click();const view=await (await response).json();
 expect(view.accessAdvice.mode).toBe('automatic');expect(view.request.spec.requestedAccess.tools).toEqual(['crm.read']);expect(view.request.spec.requestedAccess.resourceScopes).toEqual(['crm:customer-retention']);
 await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
 await page.getByText('Permission preparation',{exact:true}).click();await expect(page.getByText('Automatic',{exact:true})).toBeVisible();
 await page.screenshot({path:test.info().outputPath('automatic.png'),fullPage:true});
});
test('ambiguous automatic task stays unstarted and stale suggestion is cleared',async({page})=>{
 await openTask(page,'automatic','investigation','Do something useful.');const before=await (await page.request.get('/api/requests')).json();
 await page.getByRole('button',{name:'Plan and start work',exact:true}).click();await expect(page.getByRole('alert')).toContainText('Choose a registered task preset');
 expect((await (await page.request.get('/api/requests')).json()).length).toBe(before.length);
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('suggest');await page.getByRole('combobox',{name:'Task preset',exact:true}).selectOption('web');await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Read https://jitsi.org/security/');
 await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();await expect(page.getByRole('heading',{name:'Suggested permissions'})).toBeVisible();
 await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Different task');await expect(page.getByRole('heading',{name:'Suggested permissions'})).toHaveCount(0);await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();
});
