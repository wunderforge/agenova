// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect,type Page} from '@playwright/test';

async function openTask(page:Page,mode:string,type:string,objective:string){
 await page.goto('/login');await expect(page.getByRole('heading',{name:'New work',exact:true})).toBeVisible();
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption(mode);
 await page.getByRole('combobox',{name:'Task type',exact:true}).selectOption(type);
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
 await page.screenshot({path:test.info().outputPath('suggestion.png'),fullPage:true});
 const response=page.waitForResponse(r=>r.url().endsWith('/api/requests')&&r.request().method()==='POST');
 await page.getByRole('button',{name:'Start work',exact:true}).click();const view=await (await response).json();expect(view.accessAdvice.mode).toBe('suggest');expect(view.accessAdvice.engine).toBe('rules');
 await expect(page.getByText('Permission preparation',{exact:true})).toBeVisible();
});
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
 await page.getByRole('button',{name:'Plan and start work',exact:true}).click();await expect(page.getByRole('alert')).toContainText('Choose a supported task type');
 expect((await (await page.request.get('/api/requests')).json()).length).toBe(before.length);
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('suggest');await page.getByRole('combobox',{name:'Task type',exact:true}).selectOption('public-research');await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Read https://jitsi.org/security/');
 await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();await expect(page.getByRole('heading',{name:'Suggested permissions'})).toBeVisible();
 await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Different task');await expect(page.getByRole('heading',{name:'Suggested permissions'})).toHaveCount(0);await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();
});
