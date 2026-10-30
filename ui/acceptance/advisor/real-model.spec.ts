// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect} from '@playwright/test';

test.skip(process.env.AGENOVA_TEST_LOCAL_ADVISOR!=='1','Opt-in existing loopback Ollama model; no downloads or paid inference.');
for(const task of [
 {name:'public research',objective:'Read https://jitsi.org/security/ and summarize the security features from that public page.',tool:'web.read',scope:'web:public-pages'},
 {name:'customer analysis',objective:'Use the internal customer-retention dataset to identify three customers with the largest usage declines. Draft follow-up recommendations without sending anything.',tool:'crm.read',scope:'crm:customer-retention'},
]){
 test(`real local model selects ${task.name} from free task context`,async({page})=>{
  await page.goto('/login');
  await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('suggest');
  // Same generic task type: this must be semantic model selection, not task-type routing.
  await page.getByRole('combobox',{name:'Task type',exact:true}).selectOption('investigation');
  await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill(task.objective);
  const reply=page.waitForResponse(r=>r.url().endsWith('/api/access-advice')&&r.request().method()==='POST');
  await page.getByRole('button',{name:'Suggest permissions',exact:true}).click();
  const result=await (await reply).json();
  expect(result.status).toBe('ready');expect(result.engine).toBe('model');
  expect(result.model.name).toBe('qwen2.5-coder:7b');expect(result.model.inputTokens).toBeGreaterThan(0);expect(result.model.outputTokens).toBeGreaterThan(0);
  expect(result.request.spec.requestedAccess.tools).toEqual([task.tool]);expect(result.request.spec.requestedAccess.resourceScopes).toEqual([task.scope]);
  expect(result.request.spec.requestedAccess.networkProfiles||[]).toEqual([]);
  await expect(page.getByRole('heading',{name:'Suggested permissions'})).toBeVisible();
  await expect(page.getByLabel(task.tool,{exact:true})).toBeChecked();
  expect((await (await page.request.get('/api/requests')).json()).length).toBe(0);
  await page.screenshot({path:test.info().outputPath(`${task.tool}.png`),fullPage:true});
  await test.info().attach('sanitized-planner-result',{body:JSON.stringify({status:result.status,engine:result.engine,profile:result.profileID,model:result.model,access:result.request.spec.requestedAccess},null,2),contentType:'application/json'});
 });
}
test('real local model automatic mode plans and starts with one click',async({page})=>{
 await page.goto('/login');
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('automatic');
 await page.getByRole('combobox',{name:'Task type',exact:true}).selectOption('investigation');
 await page.getByRole('textbox',{name:'What should it do?',exact:true}).fill('Analyze the internal customer-retention dataset for declining usage and recommend follow-ups. Do not send messages or browse public sources.');
 const reply=page.waitForResponse(r=>r.url().endsWith('/api/access-submit')&&r.request().method()==='POST');
 await page.getByRole('button',{name:'Plan and start work',exact:true}).click();
 const work=await (await reply).json();
 expect(work.accessAdvice.mode).toBe('automatic');expect(work.accessAdvice.plannerModel).toBe('qwen2.5-coder:7b');
 expect(work.accessAdvice.plannerInputTokens).toBeGreaterThan(0);expect(work.request.spec.requestedAccess.tools).toEqual(['crm.read']);
 await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
 await page.getByText('Permission preparation',{exact:true}).click();
 await expect(page.getByText('Automatic',{exact:true})).toBeVisible();
 await page.screenshot({path:test.info().outputPath('model-automatic.png'),fullPage:true});
});
