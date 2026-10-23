// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {expect,test} from '@playwright/test';
import {readFileSync} from 'node:fs';
const state=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json',import.meta.url),'utf8'));
const request={apiVersion:'agenova.io/v1alpha1',kind:'ClaimRequest',metadata:{name:'w'},spec:{templateRef:'engineer',projectRef:'payments',task:{type:'investigation',input:{workName:'A long public-page investigation that must keep the controls together',objective:'Read a public page'}},requestedAccess:{tools:[],resourceScopes:[]},runtime:{profileRef:'standard-isolated',timeout:'5m'}}};
const setup={installation:{kind:'installed',platform:'Browser fixture',revision:'sha256:'+'a'.repeat(64)},principal:state.principal,template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{artifact:{image:'example.invalid/image'},entrypoint:{command:['worker']},defaults:{modelProfile:'approved-coding-model'},capabilityCeiling:{tools:[],resourceScopes:[],modelProfiles:['approved-coding-model'],runtimeProfiles:['standard-isolated'],maxTimeout:'30m'}}},project:'payments',projects:['payments'],policy:{ID:'p',Version:'1',Rules:[]},capabilities:{runtime:'fixture'}};
for(const width of [1440,390])test(`incomplete reason and grouped actions at width ${width}`,async({page},info)=>{
 await page.setViewportSize({width,height:1000});const s=structuredClone(state);s.claim.phase='Succeeded';s.effectiveAuthority.tools=[];
 const view={version:'agenova.evidence/v0',requestRef:'w',request,state:s,facts:[],outcome:{status:'Succeeded',text:'I could not read the page because web.read was unavailable.',completion:{status:'incomplete',reason:'missing-capability',target:'web.read'}}};
 await page.route('**/api/**',route=>{const path=new URL(route.request().url()).pathname;return route.fulfill({json:path==='/api/setup'?setup:path==='/api/requests'?[view]:view});});
 await page.goto('/?mode=connected#/work/w');await expect(page.getByRole('alert')).toHaveText('web.read unavailableDetails');await expect(page.locator('.portal-title-line')).toContainText('Incomplete');await expect(page.locator('.portal-title-line')).not.toContainText('Succeeded');await expect(page.locator('.portal-head-actions').getByRole('link',{name:'Copy as new Work'})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:info.outputPath(`incomplete-${width}.png`),fullPage:true});
 await page.getByRole('link',{name:'Details',exact:true}).click();await expect(page.getByRole('heading',{name:'Task completion',exact:true})).toBeVisible();await expect(page.getByText('I could not read the page because web.read was unavailable.',{exact:true})).toBeVisible();
 await page.goto('/?mode=connected#/work');const group=page.locator('.portal-head-actions');await expect(group.getByRole('link',{name:'Compare two Work'})).toBeVisible();await expect(group.getByRole('link',{name:'New work',exact:true})).toBeVisible();
 await expect(group).toHaveCSS('gap','10px');const spacing=await group.getByRole('link').evaluateAll(links=>links[1].getBoundingClientRect().left-links[0].getBoundingClientRect().right);if(width>650)expect(spacing).toBeLessThanOrEqual(16);
});
test('incomplete task and unconfirmed cleanup both retain their evidence entries',async({page},info)=>{
 const s=structuredClone(state);s.claim.phase='Succeeded';s.effectiveAuthority.tools=[];
 const view={version:'agenova.evidence/v0',requestRef:'w',request,state:s,facts:[{id:'finish',sequence:1,timestamp:'2026-10-23T09:00:00Z',kind:'RunOutcome',requestRef:'w',claimId:s.claim.id,operation:'Succeeded',reason:'Worker cleanup was not confirmed.'}],outcome:{status:'Succeeded',text:'No web access available.',failure:'Worker cleanup was not confirmed.',completion:{status:'incomplete',reason:'missing-capability',target:'web.read'}}};
 await page.route('**/api/**',route=>{const path=new URL(route.request().url()).pathname;return route.fulfill({json:path==='/api/setup'?setup:path==='/api/requests'?[view]:view});});
 await page.goto('/?mode=connected#/work/w');
 const notice=page.getByRole('alert');await expect(notice).toContainText('web.read unavailable');await expect(notice).toContainText('Cleanup needs attention');await expect(notice.locator('.portal-blocked-event')).toHaveCount(2);await expect(page.locator('.portal-title-line')).toContainText('Incomplete');
 await expect(notice.getByRole('link',{name:'Details'})).toHaveAttribute('href','#/work/w/completion');await expect(notice.getByRole('link',{name:'Evidence'})).toHaveAttribute('href','#/work/w/activity/finish');
 await page.screenshot({path:info.outputPath('incomplete-and-cleanup-warning.png'),fullPage:true});
});
