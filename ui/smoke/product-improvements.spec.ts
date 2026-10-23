// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Browser fault injection for UI recovery; not a real-provider acceptance claim.
import {expect,test,type Page} from '@playwright/test';
import {readFileSync} from 'node:fs';
import type {ClaimRequest,IssuedState} from '../src/contracts.generated';
import type {Setup,View} from '../src/connected-source';
const state=JSON.parse(readFileSync(new URL('../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json',import.meta.url),'utf8')) as IssuedState;
const template:NonNullable<Setup['template']>={apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{artifact:{image:'example.invalid/agent:fixture'},entrypoint:{command:['worker']},defaults:{modelProfile:'approved-coding-model'},capabilityCeiling:{tools:['git.read'],resourceScopes:['repo:acme/payments','repo:acme/billing'],modelProfiles:['approved-coding-model'],runtimeProfiles:['standard-isolated'],maxTimeout:'30m'}}};
const setup:Setup={installation:{kind:'installed',platform:'browser-fixture',revision:'sha256:'+'a'.repeat(64)},principal:state.principal,project:'payments',projects:['payments','billing'],submissionProjects:['payments','billing'],templates:['engineer','reader'],template,policy:{ID:'policy',Version:'1',Rules:[{team:'team-a',action:'claim.create',project:'payments',templateRef:'engineer',constraints:{tools:{values:[]},resourceScopes:{values:['repo:acme/payments']}}}]},capabilities:{runtime:'fixture',tool:'mock'}};
function result(request:ClaimRequest):View{const s=structuredClone(state);const ref='work-server-canonical';s.requestRef=ref;s.claim!.requestRef=ref;s.claim!.phase='Succeeded';return {version:'agenova.evidence/v0',requestRef:ref,request,state:s,facts:[],outcome:{status:'Succeeded',text:'Browser fixture result; not a live inference.'}};}
async function base(page:Page,configured:Setup=setup){await page.route('**/api/**',async route=>{const path=new URL(route.request().url()).pathname;if(path==='/api/setup')return route.fulfill({json:configured});if(path==='/api/requests')return route.fulfill({json:[]});return route.fulfill({status:404,json:{code:'not_found'}});});}
for(const reload of [false,true])test(`lost response keeps one reference and reconciles canonical Work; reload=${reload}`,async({page})=>{
 await base(page);let saved:View|undefined;const posts:ClaimRequest[]=[];
 await page.route('**/api/requests',async route=>{if(route.request().method()!=='POST')return route.fulfill({json:saved?[saved]:[]});const body=route.request().postDataJSON() as ClaimRequest;posts.push(body);saved=result(body);return route.abort('failed');});
 await page.route('**/api/submissions/*/evidence',route=>route.fulfill({json:saved}));
 await page.route('**/api/requests/*/evidence',route=>route.fulfill({json:saved}));
 await page.goto('/?mode=connected#/work/new');await expect(page.getByRole('checkbox',{name:'git.read'})).not.toBeChecked();await expect(page.getByLabel('Resource scopes',{exact:true})).toHaveValue('');
 await page.getByLabel('What should it do?').fill('Inspect the permitted evidence.');await page.getByRole('button',{name:'Start work',exact:true}).click();await expect(page.getByRole('heading',{name:'Submission result not confirmed'})).toBeVisible();
 expect(posts).toHaveLength(1);expect(posts[0].spec.task?.type).toBe('investigation');expect(posts[0].spec.requestedAccess?.tools).toEqual([]);
 await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeDisabled();
 const storage=await page.evaluate(()=>JSON.stringify({...sessionStorage}));expect(storage).toContain(posts[0].metadata.name);expect(storage).not.toContain('Inspect the permitted evidence.');
 if(reload){await page.reload();await expect(page.getByRole('button',{name:'Retry identical request'})).toHaveCount(0);}
 await page.getByRole('button',{name:'Check existing request'}).click();await expect(page).toHaveURL(/work-server-canonical$/);expect(posts).toHaveLength(1);expect(await page.evaluate(()=>sessionStorage.getItem('agenova.pending-request'))).toBeNull();
});
test('same-key retry preserves the complete request instead of issuing a second logical task',async({page})=>{
 await base(page);const posts:ClaimRequest[]=[];let saved:View|undefined;
 await page.route('**/api/requests',route=>{if(route.request().method()!=='POST')return route.fulfill({json:saved?[saved]:[]});const body=route.request().postDataJSON() as ClaimRequest;posts.push(body);saved=result(body);return posts.length===1?route.fulfill({status:503,json:{code:'storage_unavailable'}}):route.fulfill({status:202,json:saved});});
 await page.route('**/api/requests/*/evidence',route=>route.fulfill({json:saved}));await page.goto('/?mode=connected#/work/new');await page.getByLabel('What should it do?').fill('Retry the exact submitted request.');await page.getByRole('button',{name:'Start work',exact:true}).click();await page.getByRole('button',{name:'Retry identical request'}).click();await expect(page).toHaveURL(/work-server-canonical$/);expect(posts).toHaveLength(2);expect(posts[1]).toEqual(posts[0]);
});
test('Policy caps are readable, and unsafe diagnostic destinations are not linked',async({page})=>{
 await base(page,{...setup,observability:{enabled:true,endpoint:'https://collector.example.test',exportFailures:2,ingestion:'notVerified',diagnosticURL:'https://user:password@bad.example.test?token=private'}});
 await page.goto('/?mode=connected#/policy');await expect(page.getByText('Explicit empty set · none allowed',{exact:true})).toBeVisible();await expect(page.locator('.portal-rule')).toContainText('repo:acme/payments');
 await page.goto('/?mode=connected#/platform');await expect(page.getByText('2 export failures recorded')).toBeVisible();await expect(page.getByRole('link',{name:'Open configured diagnostics ↗'})).toHaveCount(0);await expect(page.getByText('Not verified',{exact:true})).toBeVisible();
});
test('template changes use the selected server snapshot and clear old permissions',async({page})=>{
 await base(page);await page.route('**/api/setup*',route=>{const query=new URL(route.request().url()).searchParams;const name=query.get('template')||'engineer';return route.fulfill({json:{...setup,template:{...template,metadata:{name},spec:{...template.spec,capabilityCeiling:{...template.spec.capabilityCeiling,tools:name==='reader'?['git.read']:['git.read'],resourceScopes:name==='reader'?['repo:acme/billing']:['repo:acme/payments']}}}}});});
 await page.goto('/?mode=connected#/work/new');await page.getByRole('checkbox',{name:'git.read'}).check();await page.getByLabel('Resource scopes',{exact:true}).fill('repo:acme/payments');await page.getByRole('combobox',{name:'Agent',exact:true}).selectOption('reader');await expect(page.getByRole('combobox',{name:'Agent',exact:true})).toHaveValue('reader');await expect(page.getByRole('checkbox',{name:'git.read'})).not.toBeChecked();await expect(page.getByLabel('Resource scopes',{exact:true})).toHaveValue('');await expect(page.locator('.portal-form .portal-chips')).toContainText('repo:acme/billing');
});
