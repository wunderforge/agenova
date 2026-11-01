// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Browser fixtures verify form behavior, not real search/LLM availability.
import {test,expect,type Page} from '@playwright/test';
import {readFileSync} from 'node:fs';
const state=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json',import.meta.url),'utf8'));
const setup={installation:{kind:'installed',platform:'browser-fixture',revision:'sha256:'+'a'.repeat(64)},principal:state.principal,project:'research',projects:['research'],submissionProjects:['research'],templates:['business-assistant'],template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'business-assistant'},spec:{artifact:{image:'example.invalid/fixture'},entrypoint:{command:['worker']},defaults:{modelProfile:'approved-coding-model'},capabilityCeiling:{tools:['web.read','crm.read'],resourceScopes:['web:public-pages','crm:customer-retention'],modelProfiles:['approved-coding-model'],runtimeProfiles:['standard-isolated'],maxTimeout:'20m'}}},policy:{ID:'research',Version:'3',Rules:[]},capabilities:{'web.read':'publicHTTPS'}};
async function form(page:Page){await page.route('**/api/**',route=>route.fulfill({json:new URL(route.request().url()).pathname==='/api/setup'?setup:[]}));await page.goto('/?mode=connected#/work/new');}
test('direct task form has no preset and exposes model-call settings next to time limit',async({page},info)=>{
 await form(page);await expect(page.getByRole('combobox',{name:'Task preset',exact:true})).toHaveCount(0);await expect(page.getByRole('combobox',{name:'Task type',exact:true})).toHaveCount(0);
 const objective=page.getByLabel('What should it do?');await expect(objective).toHaveValue('');await objective.fill('Summarize the customer usage declines.');
 const budget=page.getByLabel('Max model calls',{exact:true}),timeout=page.getByLabel('Time limit (minutes)',{exact:true});await expect(budget).toHaveValue('6');await expect(budget).toHaveAttribute('max','6');
 await timeout.fill('3');await budget.fill('2');await expect(objective).toHaveValue('Summarize the customer usage declines.');
 expect(Math.abs((await budget.boundingBox())!.y-(await timeout.boundingBox())!.y)).toBeLessThan(3);
 await budget.fill('7');expect(await budget.evaluate((el:HTMLInputElement)=>el.validity.rangeOverflow)).toBe(true);
 await budget.fill('2');await page.getByRole('heading',{name:'New work',exact:true}).click();await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:info.outputPath('direct-task-settings.png'),fullPage:true});
});

test('pointer dropdown is quiet and keyboard focus remains visible',async({page},info)=>{
 await form(page);const project=page.getByRole('combobox',{name:'Project',exact:true});await project.click();await project.press('Escape');
 expect(await project.evaluate(el=>getComputedStyle(el).outlineStyle)).toBe('none');
 await project.press('Tab');await page.getByRole('combobox',{name:'Agent',exact:true}).press('Shift+Tab');await expect(project).toBeFocused();
 expect(await project.evaluate(el=>getComputedStyle(el).outlineWidth)).toBe('2px');await page.screenshot({path:info.outputPath('keyboard-dropdown-focus.png'),fullPage:true});
});
