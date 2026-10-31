// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect} from '@playwright/test';
import type {View} from '../../../src/connected-source';
// Re-render existing synthetic-local acceptance records without new inference.
const refs=(process.env.AGENOVA_REVIEW_WORK_REFS||'').split(',').filter(Boolean);
if(refs.length)test('review local New Work form without another model call',async({page})=>{
 await page.goto('/login');
 await expect(page.getByRole('heading',{name:'New work',exact:true})).toBeVisible();
 await expect(page.getByRole('combobox',{name:'Task type',exact:true})).toHaveCount(0);
 await expect(page.getByLabel('What should it do?',{exact:true})).toHaveValue('');
 await expect(page.getByRole('combobox',{name:'Permission setup',exact:true})).toBeVisible();
 await page.screenshot({path:test.info().outputPath('new-work.png'),fullPage:true});
});
for(const [index,ref] of refs.entries())test(`review stored local Work ${index+1}`,async({page})=>{
 expect(ref).toMatch(/^work-[a-zA-Z0-9-]+$/);
 await page.goto('/login');
 const response=await page.request.get(`/api/requests/${ref}/evidence`);expect(response.ok()).toBe(true);
 const view=await response.json() as View;expect(view.outcome?.status).toBe('Succeeded');
 expect(view.facts.some(f=>(f.operation||'').startsWith('NativeHTTPS'))).toBe(false);
 await page.goto(`/?mode=connected&project=research&template=researcher-advisor-v2#/work/${ref}`);
 await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible({timeout:15000});
 await expect(page.locator('.portal-title-line')).toContainText(view.outcome?.completion?.status==='incomplete'?'Incomplete':'Succeeded');
 await expect(page.getByRole('table',{name:'Requested and granted task permissions'})).toBeVisible();
 await page.screenshot({path:test.info().outputPath(`review-${index+1}.png`),fullPage:true});
});
