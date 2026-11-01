// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect,type Page} from '@playwright/test';
import type {View} from '../../../src/connected-source';
test.skip(process.env.AGENOVA_TEST_LIVE_RESEARCHER!=='1','Explicit local inference opt-in required; no cloud provider or model download.');

async function form(page:Page,objective:string,mode:string){
 await page.goto('/login');await expect(page.getByRole('heading',{name:'New work',exact:true})).toBeVisible();
 await expect(page.getByRole('combobox',{name:'Agent',exact:true})).toHaveValue('researcher-advisor-v3');
 await expect(page.getByRole('combobox',{name:'Task type',exact:true})).toHaveCount(0);
 await page.getByLabel('What should it do?',{exact:true}).fill(objective);
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption(mode);
 await expect(page.getByRole('combobox',{name:'Task preset',exact:true})).toHaveCount(0);
 await expect(page.getByLabel('Max model calls',{exact:true})).toHaveValue('6');
 await page.getByLabel('Time limit (minutes)',{exact:true}).fill('5');
}
async function start(page:Page,automatic=false){
 const posted=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith(automatic?'/api/access-submit':'/api/requests'));
 await page.getByRole('button',{name:automatic?'Plan and start work':'Start work',exact:true}).click();
 const response=await posted;expect(response.status()).toBe(202);const accepted=await response.json() as View;
 expect(accepted.request?.spec.task?.type).toBeUndefined();
 let current:View=accepted;
 await expect.poll(async()=>{current=await (await page.request.get(`/api/requests/${accepted.requestRef}/evidence`)).json();return !!current.outcome},{timeout:320000,intervals:[1500,2500]}).toBe(true);
 await page.goto(`/?mode=connected&project=research&template=researcher-advisor-v3#/work/${accepted.requestRef}`);
 await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
 await test.info().attach('work-evidence',{body:JSON.stringify(current,null,2),contentType:'application/json'});
 expect(current.outcome?.status).toBe('Succeeded');expect(current.outcome?.model?.inputTokens).toBeGreaterThan(0);
 expect(current.facts.some(f=>f.kind==='ProviderOutcome'&&f.operation==='model.invoke'&&f.providerStatus==='Succeeded')).toBe(true);
 expect(current.facts.some(f=>(f.operation||'').startsWith('NativeHTTPS'))).toBe(false);
 await expect(page.getByRole('heading',{name:current.outcome?.completion?.reportedBy==='platform'?'Execution summary':'Result',exact:true})).toBeVisible({timeout:15000});
 await expect(page.locator('.portal-title-line')).not.toContainText('Running');
 return current;
}
test('Manual public goal chooses its own official source and returns observed facts',async({page})=>{
 await form(page,'Explain what Python is used for, based on its official website.','manual');
 await page.getByLabel('web.read',{exact:true}).check();await page.getByLabel('Resource scopes',{exact:true}).fill('web:public-pages');
 const v=await start(page);expect(v.accessAdvice).toBeUndefined();expect(v.outcome?.completion?.status).toBe('completed');
 expect(v.outcome?.text).toMatch(/python\.org/i);
 expect((v.outcome?.text?.match(/\S+/g)||[]).length).toBeLessThanOrEqual(110);
 expect(v.facts.some(f=>f.kind==='ProviderOutcome'&&f.operation==='tool.invoke'&&f.providerStatus==='Succeeded')).toBe(true);
 await page.screenshot({path:test.info().outputPath('public-goal.png'),fullPage:true});
});
test('Automatic internal goal uses the same Agent with synthetic CRM evidence',async({page})=>{
 await form(page,'Identify three customers with the largest usage declines and draft follow-up recommendations.','automatic');
 const v=await start(page,true);expect(v.accessAdvice?.profileID).toBe('customer-retention');
 expect(v.request?.spec.requestedAccess?.tools).toEqual(['crm.read']);expect(v.outcome?.completion?.status).toBe('completed');
 expect(v.outcome?.text).toMatch(/synthetic/i);for(const id of ['C001','C002','C003'])expect(v.outcome?.text).toContain(id);
 expect(v.outcome?.text).toMatch(/follow|recommend|reach|contact|schedule|offer|engage|check.in/i);
 expect(v.facts.some(f=>f.kind==='ProviderOutcome'&&f.operation==='tool.invoke'&&f.providerStatus==='Succeeded')).toBe(true);
 await page.screenshot({path:test.info().outputPath('internal-goal.png'),fullPage:true});
});
test('Public goal with only CRM access stays incomplete without an unrelated customer read',async({page})=>{
 await form(page,'Explain what Python is used for, based on its official website.','manual');
 await page.getByLabel('crm.read',{exact:true}).check();await page.getByLabel('Resource scopes',{exact:true}).fill('crm:customer-retention');
 const v=await start(page);expect(v.outcome?.completion?.status).toBe('incomplete');
 expect(v.facts.some(f=>f.kind==='ToolDecision'&&f.result==='Deny'&&f.target==='web.read'&&f.actionRef&&f.turn)).toBe(true);
 await expect(page.getByText('web.read blocked',{exact:true})).toBeVisible();
 expect(v.facts.filter(f=>f.kind==='ProviderAttempt'&&f.operation==='tool.invoke')).toHaveLength(0);
 expect(v.outcome?.text).not.toMatch(/C00[123]|Northstar|Fern Studio/);
 await page.screenshot({path:test.info().outputPath('missing-web-access.png'),fullPage:true});
});

test('Suggest uses the separately configured planner, allocates nothing and clears stale runtime advice',async({page})=>{
 await form(page,'Identify three customers with the largest usage declines and draft follow-up recommendations.','suggest');
 const before=await(await page.request.get('/api/requests')).json();
 const proposed=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/api/access-advice'));
 await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();const response=await proposed;expect(response.ok()).toBe(true);const plan=await response.json();
 expect(plan.status).toBe('ready');expect(plan.profileID).toBe('customer-retention');expect(plan.model.name).toBe('qwen2.5-coder:7b');
 expect((await(await page.request.get('/api/requests')).json()).length).toBe(before.length);
 await expect(page.getByLabel('crm.read',{exact:true})).toBeChecked();await expect(page.getByLabel('web.read',{exact:true})).not.toBeChecked();
 await page.screenshot({path:test.info().outputPath('suggestion.png'),fullPage:true});
 await page.getByLabel('Max model calls',{exact:true}).fill('8');await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();
 await page.goto('/?mode=connected&project=research&template=researcher-advisor-v3#/platform');await expect(page.getByText('qwen2.5-coder:7b',{exact:true}).first()).toBeVisible();
 await page.screenshot({path:test.info().outputPath('platform-models.png'),fullPage:true});
});
