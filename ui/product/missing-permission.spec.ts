// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Actual local APIs/Gateway/MCP configuration; synthetic runtime/model only.
import {test,expect} from '@playwright/test';

for(const variant of ['missing-tool','wrong-scope','no-tools'] as const){
 test(`web research ${variant}: real refusal, no provider attempts, no false result`,async({page},info)=>{
  await page.goto('/login');await page.locator('select[name=identity]').selectOption('owner');await page.getByRole('button',{name:'Enter local workspace'}).click();
  await page.goto('/?mode=connected&project=research&template=researcher-v2#/work/new');
  await page.getByRole('combobox',{name:'Task preset',exact:true}).selectOption('web');
  await page.getByLabel('Work name (optional)').fill('Web access denied · '+variant);
  await page.getByLabel('What should it do?').fill('Read https://jitsi.org/jitsi-meet/ and summarize meeting features for a consulting team.');
  if(variant!=='wrong-scope')await page.getByRole('checkbox',{name:'web.read',exact:true}).uncheck();
  if(variant==='missing-tool')await page.getByRole('checkbox',{name:'crm.read',exact:true}).check();
  if(variant==='wrong-scope')await page.getByLabel('Resource scopes',{exact:true}).fill('crm:customer-retention');
  if(variant==='no-tools')await page.getByLabel('Resource scopes',{exact:true}).fill('');
  await page.getByRole('button',{name:'Start work',exact:true}).click();
  await expect(page.locator('.portal-head .portal-badge')).toHaveText('Failed',{timeout:30000});
  await expect(page.locator('.portal-work-failure')).toContainText('denied');
  await expect(page.locator('.portal-blocked-event')).toContainText('web.read');
  const banner=page.locator('.portal-work-refusals-highlight');await expect(banner).toHaveAttribute('role','alert');
  await expect(banner).toHaveText('web.read blockedEvidence');
  expect(await banner.evaluate(el=>!!(el.compareDocumentPosition(document.querySelector('.portal-work-permissions')!)&Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
  await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
  const ref=decodeURIComponent(page.url().split('#/work/')[1]);const evidence=await(await page.request.get(`/api/requests/${encodeURIComponent(ref)}/evidence`)).json();
  expect(evidence.outcome.status).toBe('Failed');expect(evidence.outcome.text||'').toBe('');expect(evidence.diagnosis.code).toBe('invocation-denied');
  const refusals=evidence.facts.filter((f:any)=>f.kind==='ToolDecision'&&f.result==='Deny');expect(refusals).toHaveLength(1);expect(refusals[0].target).toBe('web.read');
  expect(evidence.facts.filter((f:any)=>f.kind==='ProviderAttempt')).toHaveLength(0);
  const table=page.getByRole('table',{name:'Requested and granted task permissions'});await expect(table).toBeVisible();
  await expect(table).not.toContainText('Memory');await expect(table).not.toContainText('Network');
  const requestedTools=variant==='wrong-scope'?['web.read']:variant==='missing-tool'?['crm.read']:[];
  expect(evidence.request.spec.requestedAccess.tools||[]).toEqual(requestedTools);
  expect(evidence.state.effectiveAuthority.tools||[]).toEqual(requestedTools);
  if(variant==='no-tools')await expect(table.getByRole('row').filter({hasText:'Tools'})).toHaveCount(0);
  // Execution refusal must not rewrite the original issued authority as denied.
  expect(await table.locator('tr[data-permission-state=not-granted]').count()).toBe(0);
  await page.screenshot({path:info.outputPath('failed-web-task.png'),fullPage:true});
  await banner.getByRole('link',{name:'Evidence',exact:true}).click();await expect(page.getByRole('heading',{name:'What this record says',exact:true})).toBeVisible();
 });
}
