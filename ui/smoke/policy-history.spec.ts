// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Browser fixtures prove presentation, not provider or infrastructure behavior.
import { expect,test } from '@playwright/test';
const current={ID:'research-governance',Version:'2',Rules:[{team:'team-a',action:'claim.create',project:'research',templateRef:'current-only-agent'}]};
const old={ID:'research-governance',Version:'1',Rules:[{team:'team-a',action:'claim.create',project:'research',templateRef:'historical-only-agent'}]};
for(const available of [true,false])test(`historical policy never substitutes current rules; available=${available}`,async({page},info)=>{
 await page.route('**/api/**',route=>{
  const path=new URL(route.request().url()).pathname;
  if(path==='/api/setup')return route.fulfill({json:{installation:{kind:'installed',platform:'browser-fixture',revision:'sha256:'+'a'.repeat(64)},principal:{subject:'fixture',team:'team-a',authenticationContext:'oidc'},project:'research',projects:['research'],submissionProjects:['research'],template:null,templates:[],policy:current,capabilities:{}}});
  if(path==='/api/requests')return route.fulfill({json:[]});
  if(path==='/api/governance')return available?route.fulfill({json:{active:{id:current.ID,version:current.Version},versions:[{policy:old,active:false},{policy:current,active:true}],events:[]}}):route.fulfill({status:503});
  return route.fulfill({status:404});
 });
 await page.goto('/?mode=connected#/policy?project=research&id=research-governance&version=1');
 await expect(page.getByRole('heading',{name:'Policy',exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'current-only-agent',exact:true})).toHaveCount(0);
 if(available)await expect(page.getByRole('heading',{name:'historical-only-agent',exact:true})).toBeVisible();
 else {await expect(page.getByText('Policy history is unavailable for this project.')).toBeVisible();await expect(page.locator('.portal-rule')).toHaveCount(0);}
 await page.screenshot({path:info.outputPath('historical-policy.png'),fullPage:true});
});
