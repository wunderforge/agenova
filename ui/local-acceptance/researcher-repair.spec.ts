// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {test,expect} from '@playwright/test';
import {fileURLToPath} from 'node:url';
import type {View} from '../src/connected-source';
const artifact=(name:string)=>fileURLToPath(new URL('../../.tmp/researcher-ui/'+name,import.meta.url));

test.beforeEach(async({page})=>{await page.goto('/login');});
test('new Work remains simple, with manual permissions adjacent to the selected mode',async({page})=>{
 await expect(page.getByRole('heading',{name:/New work/i,exact:true})).toBeVisible();
 await expect(page.getByLabel('Task type',{exact:true})).toHaveCount(0);
 await expect(page.getByLabel('Task preset',{exact:true})).toHaveCount(0);
 await expect(page.getByLabel('What should it do?',{exact:true})).toBeVisible();
 await page.getByRole('combobox',{name:'Permission setup',exact:true}).selectOption('manual');
 await page.getByRole('heading',{name:/New work/i,exact:true}).click();
 await expect(page.getByLabel('web.search',{exact:true})).toBeVisible();
 await expect(page.getByLabel('web.read',{exact:true})).toBeVisible();
 await page.screenshot({path:artifact('new-work-desktop.png'),fullPage:true});
});
test('real model response, rationale and tool observation are inspectable without altering authority',async({page})=>{
 const records:View[]=await (await page.request.get('/api/requests')).json();
 const view=[...records].reverse().find(v=>v.outcome?.completion?.status==='completed'&&v.facts.some(f=>f.contentRef));
 expect(view,'Run repair_acceptance.py first.').toBeTruthy();
 await page.goto(`/?mode=connected&project=research&template=researcher-advisor-v4#/work/${view!.requestRef}`);
 const activity=page.getByRole('region',{name:'Worker activity',exact:true});
 await expect(activity).toBeVisible();
 const turn=activity.locator('.portal-turn').last();await expect(turn).toHaveAttribute('open','');
 await turn.getByRole('button',{name:/Inspect Model request:/}).click();
 const inspector=page.getByRole('complementary',{name:'Selected activity'});
 await expect(inspector.getByRole('heading',{name:'Model response',exact:true})).toBeVisible();
 await expect(inspector.getByRole('heading',{name:/Decision summary/})).toBeVisible();
 await expect(inspector.locator('pre')).toContainText('finish');
 await expect(inspector).not.toContainText('Content unavailable');
 await activity.scrollIntoViewIfNeeded();
 await page.screenshot({path:artifact('model-response-desktop.png'),fullPage:false});
 await activity.screenshot({path:artifact('model-response-panel.png')});
 const first=activity.locator('.portal-turn').first();
 if(!(await first.getAttribute('open'))){await first.locator('summary').click();}
 await first.getByRole('button',{name:/Inspect Tool call:/}).first().click();
 await expect(inspector.getByRole('heading',{name:'Tool result',exact:true})).toBeVisible();
 await expect(inspector.getByRole('heading',{name:'Request',exact:true})).toBeVisible();
 await expect(inspector.locator('pre').first()).toContainText('parameters');
 await page.screenshot({path:artifact('tool-observation-desktop.png'),fullPage:false});
 await page.setViewportSize({width:900,height:1000});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:artifact('tool-observation-narrow.png'),fullPage:false});
});
test('actual Gateway denial is visible, with no backend attempt attributed to that Work',async({page})=>{
 const records:View[]=await (await page.request.get('/api/requests')).json();
 const view=[...records].reverse().find(v=>v.facts.some(f=>f.kind==='ToolDecision'&&f.result==='Deny'));
 expect(view,'Run the denial acceptance first.').toBeTruthy();
 expect(view!.facts.filter(f=>f.kind==='ProviderAttempt'&&f.operation==='tool.invoke')).toHaveLength(0);
 await page.goto(`/?mode=connected&project=research&template=researcher-advisor-v4#/work/${view!.requestRef}`);
 await expect(page.getByText(/web.read blocked/i)).toBeVisible();
 await page.screenshot({path:artifact('gateway-denial-header.png'),fullPage:false});
 const activity=page.getByRole('region',{name:'Worker activity',exact:true});
 const first=activity.locator('.portal-turn').first();if(!(await first.getAttribute('open'))){await first.locator('summary').click();}
 await first.getByRole('button',{name:/Inspect Tool access:/}).click();
 const inspector=page.getByRole('complementary',{name:'Selected activity'});
 await expect(inspector.getByRole('heading',{name:'Gateway decision',exact:true})).toBeVisible();
 await expect(inspector.locator('pre').last()).toContainText('Deny');
 await page.screenshot({path:artifact('gateway-denial-desktop.png'),fullPage:false});
});

test('a handled source failure retains its actual cause after the Work finishes',async({page})=>{
 const records:View[]=await (await page.request.get('/api/requests')).json();
 const view=records.find(v=>v.facts.some(f=>f.reasonCode==='source-unavailable'&&f.reason?.includes('404')));
 expect(view,'Run the failure acceptance first.').toBeTruthy();
 await page.goto(`/?mode=connected&project=research&template=researcher-advisor-v4#/work/${view!.requestRef}`);
 const activity=page.getByRole('region',{name:'Worker activity',exact:true});
 await expect(activity).toContainText('1 tool call failed');
 const first=activity.locator('.portal-turn').first();if(!(await first.getAttribute('open'))){await first.locator('summary').click();}
 await first.getByRole('button',{name:/Inspect Tool call:/}).click();
 const inspector=page.getByRole('complementary',{name:'Selected activity'});
 await expect(inspector).toContainText('source-unavailable');
 await expect(inspector).toContainText('404');
 await expect(inspector).toContainText('response');
 await page.screenshot({path:artifact('source-failure-desktop.png'),fullPage:false});
});
