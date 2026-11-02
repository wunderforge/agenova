// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Synthetic catalog rendering; not a live authorization or image-health claim.
import {expect,test} from '@playwright/test';
import type {AgentEntry,Setup,View} from '../src/connected-source';

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

for(const historical of [false,true]) test(`New Work picker offers latest and preserves explicit historical copy: historical=${historical}`,async({page},info)=>{
  const names=['engineer','researcher-advisor-v4','researcher-advisor-v5','researcher-advisor-v6','researcher-v2','researcher-v7'];
  const template=(name:string):NonNullable<Setup['template']>=>({apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name},spec:{artifact:{image:'registry.example.test/'+name},entrypoint:{command:['/agent']},defaults:{modelProfile:'general-model'},capabilityCeiling:{tools:['crm.read'],resourceScopes:['crm:customer-retention'],modelProfiles:['general-model'],runtimeProfiles:['standard-isolated'],maxTimeout:'5m'}}});
  const setup:Setup={installation:{kind:'installed',platform:'browser-fixture',revision:'sha256:'+'a'.repeat(64)},principal:{subject:'fixture-admin',team:'team-a',authenticationContext:'browser-fixture'},project:'research',projects:['research'],submissionProjects:['research'],templates:names,template:template('researcher-advisor-v6'),policy:{ID:'fixture-policy',Version:'1',Rules:[]},capabilities:{runtime:'configured'}};
  const copy:View={version:'agenova.evidence/v0',requestRef:'work-historical',request:{apiVersion:'agenova.io/v1alpha1',kind:'ClaimRequest',metadata:{name:'work-historical'},spec:{projectRef:'research',templateRef:'researcher-advisor-v4',task:{input:{objective:'Identify at-risk customers.'}},requestedAccess:{tools:['crm.read'],resourceScopes:['crm:customer-retention'],modelProfile:'general-model'},runtime:{profileRef:'standard-isolated',timeout:'3m'}}},facts:[]};
  let posted:View['request']|undefined;
  await page.route('**/api/**',route=>{
    const url=new URL(route.request().url());
    if(url.pathname==='/api/setup')return route.fulfill({json:{...setup,template:template(url.searchParams.get('template')||'researcher-advisor-v6')}});
    if(url.pathname==='/api/requests/work-historical/evidence')return route.fulfill({json:copy});
    if(url.pathname==='/api/requests'&&route.request().method()==='POST'){posted=route.request().postDataJSON();return route.fulfill({status:400,json:{code:'invalid_request'}});}
    return route.fulfill({json:[]});
  });
  await page.goto('/?mode=connected&project=research&template=researcher-advisor-v6#/work/new'+(historical?'?copy=work-historical':''));
  const agent=page.getByRole('combobox',{name:'Agent',exact:true});
  await expect(agent).toHaveValue(historical?'researcher-advisor-v4':'researcher-advisor-v6');
  await expect(agent.locator('option')).toHaveCount(historical?4:3);
  await expect(agent.locator('option[value="researcher-v7"]')).toHaveCount(0);
  await expect(agent.locator('option[value="researcher-advisor-v5"]')).toHaveCount(0);
  await expect(agent.locator('option[value="engineer"]')).toHaveCount(1);
  if(historical){
    await expect(agent.locator('option[value="researcher-advisor-v4"]')).toHaveText('researcher-advisor-v4 · previous version');
    await expect(page.getByLabel('crm.read',{exact:true})).toBeChecked();
    await page.getByRole('button',{name:'Start work',exact:true}).click();
    await expect.poll(()=>posted?.spec.templateRef).toBe('researcher-advisor-v4');
    await agent.selectOption('researcher-advisor-v6');
    await expect(agent).toHaveValue('researcher-advisor-v6');
    await expect(page.getByLabel('crm.read',{exact:true})).not.toBeChecked();
    await expect(agent.locator('option')).toHaveCount(3);
    expect(copy.request.spec.templateRef).toBe('researcher-advisor-v4');
  }
  await page.screenshot({path:info.outputPath(historical?'new-work-historical-selection.png':'new-work-latest-picker.png'),fullPage:true});
  await page.goto('/?mode=connected&project=research&template=researcher-advisor-v6#/policy');
  await expect(page.getByRole('combobox',{name:'Registered agent',exact:true}).locator('option')).toHaveCount(2);
});
