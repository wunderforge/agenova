// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {expect,test,type Page} from '@playwright/test';
import {readFileSync} from 'node:fs';
import type {Setup,View} from '../src/connected-source';

const vectors = JSON.parse(readFileSync(new URL('../../work/0179-scoped-memory/memory-reader-vectors.json',import.meta.url),'utf8')) as {view:View};
const setup:Setup = {installation:{kind:'installed',platform:'reference-reader-fixture',revision:'sha256:'+'a'.repeat(64)},
  principal:vectors.view.state!.principal,
  template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{artifact:{image:'reader-fixture'},entrypoint:{command:['worker']},capabilityCeiling:{memoryScopes:['team-docs'],memoryOperations:['read','write']}}},
  policy:{ID:'policy:demo',Version:'1',Rules:[{team:'team-a',action:'claim.create',project:'payments',templateRef:'engineer'}]},
  capabilities:{taskSubmission:'ready',runtime:'configured',model:'notConnected',memory:'notConnected'}};
async function api(page:Page, current:()=>View) {
  await page.route('**/api/**',route=>{
    const path = new URL(route.request().url()).pathname;
    return route.fulfill({json:path === '/api/setup' ? setup : path === '/api/requests' ? [current()] : current()});
  });
}
for (const width of [1100,390]) {
  test(`Memory reader shows private-safe records at ${width}px`,async({page},info)=>{
    await page.setViewportSize({width,height:1000});
    const current = structuredClone(vectors.view);
    await api(page,()=>current);
    await page.goto('/?mode=connected#/work/memory-reader');
    await expect(page.getByRole('heading',{name:'memory-reader',exact:true})).toBeVisible();
    const actions = page.locator('.portal-worker-actions');
    await expect(actions).toContainText('Memory write'); await expect(actions).toContainText('Written');
    await expect(actions).toContainText('Memory search'); await expect(actions).toContainText('Found');
    await expect(actions).not.toContainText('Model request');
    await expect(page.locator('.portal-activity-help')).not.toContainText('no execution calls recorded');
    await expect(page.getByText('Task instructions: Content withheld',{exact:true})).toBeVisible();
    await page.screenshot({path:info.outputPath(`memory-work-${width}.png`),fullPage:true});
    await actions.getByRole('link',{name:'Memory search',exact:true}).click();
    await expect(page.getByRole('heading',{name:'Memory search finished',exact:true})).toBeVisible();
    await expect(page.locator('.portal-fact').filter({hasText:'Returned count'})).toContainText('1');
    await expect(page.locator('.portal-fact').filter({hasText:'Permission'})).toContainText('Allow');
    await expect(page.getByText('Memory content: Content withheld',{exact:true})).toBeVisible();
    await expect(page.locator('body')).not.toContainText('private-');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
    await page.screenshot({path:info.outputPath(`memory-record-${width}.png`),fullPage:true});
    await page.goto('/?mode=connected#/platform');
    const memory = page.locator('tr').filter({has:page.getByText('Memory Interface',{exact:true})});
    await expect(memory).toContainText('Not connected'); await expect(memory).toContainText('Activity recorded');
    current.facts[11].reason = 'private-database-error-sentinel';
    await page.goto('/?mode=connected#/work/memory-reader');
    await expect(page.getByRole('alert')).toContainText('incomplete work record');
    await expect(page.locator('body')).not.toContainText('private-database-error-sentinel');
  });
}
test('Memory reader keeps uncertain writes and denied checks distinct',async({page})=>{
  const current = structuredClone(vectors.view);
  current.facts[8].memory = {status:'WriteUncertain',count:0,durationMilliseconds:2,truncated:false};
  current.facts[8].providerStatus = 'Failed'; current.facts[8].reasonCode = 'memory-write-uncertain';
  await api(page,()=>current);
  await page.goto('/?mode=connected#/work/memory-reader/activity');
  await page.getByRole('button',{name:'Memory Interface',exact:true}).click();
  await expect(page.locator('.portal-record-row')).toHaveCount(7);
  await expect(page.locator('.portal-record-row')).toContainText(['Allow','Attempted','WriteUncertain','Allow','Attempted','Found','Deny']);
  await page.goto('/?mode=connected#/work/memory-reader/activity/fact%3A9');
  await expect(page.locator('.portal-head .portal-badge')).toHaveText('WriteUncertain');
  await expect(page.locator('.portal-fact').filter({hasText:'Returned count'})).toContainText('0');
});
