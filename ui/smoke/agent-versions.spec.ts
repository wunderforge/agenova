// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Synthetic catalog rendering; not a live authorization or image-health claim.
import {expect,test} from '@playwright/test';
import type {AgentEntry,Setup} from '../src/connected-source';

test('catalog keeps latest project-qualified registrations and offers older versions',async({page},info)=>{
  const entries:AgentEntry[]=[['researcher-v7','research'],['researcher-advisor-v4','research'],['researcher-advisor-v6','research'],['engineer','payments'],['incident-reader','payments']].map(([name,project])=>({project,canSubmit:true,template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name},spec:{artifact:{image:'registry.example.test/agents@sha256:'+'a'.repeat(64)},entrypoint:{command:['/agent']},capabilityCeiling:{tools:name.startsWith('researcher')?['web.search','web.read','crm.read']:['git.read']}}}}));
  const setup:Setup={installation:{kind:'installed',platform:'browser-fixture',revision:'sha256:'+'a'.repeat(64)},principal:{subject:'fixture-admin',team:'team-a',authenticationContext:'browser-fixture'},project:'research',projects:['research','payments'],template:entries[2].template,policy:{ID:'fixture-policy',Version:'1',Rules:[]},capabilities:{runtime:'configured'}};
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/api/**',route=>{
    const path=new URL(route.request().url()).pathname;
    if(path==='/api/setup')return route.fulfill({json:setup});
    if(path==='/api/agents')return route.fulfill({json:entries});
    if(path==='/api/requests')return route.fulfill({json:[]});
    return route.fulfill({status:404,json:{code:'not_found'}});
  });
  await page.emulateMedia({colorScheme:'dark'});
  await page.setViewportSize({width:1440,height:1000});
  await page.goto('/?mode=connected#/agents');
  const table=page.getByRole('table',{name:'Agent catalog'});
  await expect(table.locator('tbody tr')).toHaveCount(3);
  await expect(page.getByRole('button',{name:'researcher-advisor-v6',exact:true})).toBeVisible();
  await expect(page.getByRole('link',{name:'Run researcher-advisor-v6 in research'})).toHaveAttribute('href',/template=researcher-advisor-v6/);
  await expect(page.getByRole('button',{name:'researcher-advisor-v4',exact:true})).toHaveCount(0);
  await expect(page.locator('.portal-sidebar')).toHaveCSS('background-color','rgb(255, 255, 255)');
  await page.screenshot({path:info.outputPath('agents-latest-desktop.png'),fullPage:true});
  await page.getByRole('checkbox',{name:'Show older versions'}).check();
  await expect(table.locator('tbody tr')).toHaveCount(5);
  await page.getByRole('searchbox',{name:'Search agents'}).fill('project:research');
  await expect(table.locator('tbody tr')).toHaveCount(3);
  await page.getByRole('checkbox',{name:'Show older versions'}).uncheck();
  await expect(table.locator('tbody tr')).toHaveCount(1);
  await page.setViewportSize({width:390,height:844});
  await expect(page.getByRole('checkbox',{name:'Show older versions'})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath('agents-latest-mobile.png'),fullPage:true});
  expect(errors).toEqual([]);
});
