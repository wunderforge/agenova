// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Actual local Go APIs + disposable PostgreSQL. Synthetic identity/runtime/model
// are explicit, and these tests are not AWS, Google or native-network evidence.
import {expect,test,type Page} from '@playwright/test';
const stamp=Date.now().toString(36);
async function signIn(page:Page,identity='owner'){
 await page.goto('/login');await page.locator('select[name=identity]').selectOption(identity);await page.getByRole('button',{name:'Enter local workspace'}).click();
 await expect(page.getByText(/Local acceptance · synthetic identity/)).toBeVisible();
}
async function create(page:Page,name:string,scope:string,objective='Investigate the retry incident using only the requested evidence.',waitResult=true){
 await page.goto('/?mode=connected&project=payments#/work/new');
 await page.getByLabel('Work name (optional)').fill(name);await page.getByLabel('What should it do?').fill(objective);
 await page.getByLabel('Resource scopes',{exact:true}).fill(scope);await page.getByRole('checkbox',{name:'git.read',exact:true}).check();
 await page.getByRole('button',{name:'Start work',exact:true}).click();
 await expect(page.getByRole('heading',{name,exact:true})).toBeVisible();
 if(waitResult)await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible();
 return decodeURIComponent(page.url().split('#/work/')[1]);
}
test('actual Go/PostgreSQL journey: same agent, distinct task scopes, Gateway refusal and comparison',async({page},info)=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));await signIn(page);
 await page.goto('/?mode=connected&project=payments#/agents');await expect(page.getByRole('table',{name:'Agent catalog'})).toContainText('incident-reader');await page.getByRole('button',{name:'engineer',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('example.invalid/agenova/local-synthetic:acceptance');await page.getByRole('button',{name:'Close details'}).click();
 const a=await create(page,'Payments investigation '+stamp,'repo:acme/payments','Investigate the payment retry incident; perform the local cross-scope acceptance probe.');
 await expect(page.getByRole('link',{name:'View evidence'})).toBeVisible();const refusalEvidence=await(await page.request.get(`/api/requests/${encodeURIComponent(a)}/evidence`)).json();expect(refusalEvidence.facts.some((f:any)=>f.kind==='ToolDecision'&&f.result==='Deny'&&f.target==='git.read')).toBe(true);await expect(page.locator('.portal-blocked-event')).toContainText('git.read');
 await page.screenshot({path:info.outputPath('work-a-blocked.png'),fullPage:true});
 await page.getByRole('link',{name:'Copy as new Work'}).click();await expect(page.getByLabel('What should it do?')).toHaveValue(/cross-scope/);await expect(page.getByLabel('Resource scopes',{exact:true})).toHaveValue('repo:acme/payments');
 const b=await create(page,'Billing investigation '+stamp,'repo:acme/billing');
 await page.goto(`/?mode=connected&project=payments#/compare?left=${encodeURIComponent(a)}&right=${encodeURIComponent(b)}`);
 await expect(page.getByRole('heading',{name:'Compare two Work requests'})).toBeVisible();await expect(page.locator('tr[data-different=true]').filter({hasText:'Granted resources'})).toContainText('repo:acme/payments');await expect(page.locator('tr[data-different=true]').filter({hasText:'Granted resources'})).toContainText('repo:acme/billing');
 await page.screenshot({path:info.outputPath('two-work-comparison.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:info.outputPath('comparison-mobile.png'),fullPage:true});expect(errors).toEqual([]);
});
test('actual administration: display metadata, separate grants, immutable publish/deny/rollback without restart',async({page},info)=>{
 await signIn(page);
 const name='Policy switch '+stamp,objective='Summarize the payment retry incident using only its permitted scope.';
 const before=await create(page,name,'repo:acme/payments',objective);
 const evidenceBefore=await(await page.request.get(`/api/requests/${encodeURIComponent(before)}/evidence`)).json();
 await page.goto('/admin#organization');await expect(page.getByRole('heading',{name:'Organization',exact:true})).toBeVisible();
 await page.getByLabel('Display name',{exact:true}).fill('Agenova local acceptance team');await page.getByRole('button',{name:'Save display information'}).click();await expect(page.locator('#organization-notice')).toContainText('Identity and permissions are unchanged');
 const org=await(await page.request.get('/api/organization')).json();expect(org.id).toBe('team-a');
 await page.goto('/admin');await page.getByRole('button',{name:'Edit local-member',exact:true}).click();const member=page.locator('article.member').filter({has:page.getByRole('heading',{name:'local-member',exact:true})});await expect(member.getByRole('combobox',{name:'Project role payments for local-member'})).toBeVisible();await expect(member.getByRole('combobox',{name:'Project role billing for local-member'})).toBeVisible();
 const billingGrant=member.locator('form').filter({has:page.getByRole('combobox',{name:'Project role billing for local-member'})});
 await billingGrant.getByRole('combobox').selectOption('viewer');await billingGrant.getByRole('button',{name:'Save project access'}).click();await expect(page.locator('#member-notice')).toContainText('User access updated');
 let members=await(await page.request.get('/api/members')).json();expect(members.find((m:{principal:string})=>m.principal==='local-member').projects).toEqual(expect.arrayContaining([{project:'payments',role:'member'},{project:'billing',role:'viewer'}]));
 await page.getByRole('button',{name:'Edit local-member',exact:true}).click();await billingGrant.getByRole('combobox').selectOption('member');await billingGrant.getByRole('button',{name:'Save project access'}).click();
 await page.goto('/admin#governance');
 await page.locator('#governance-project').selectOption('payments');await page.locator('#governance-load').click();await expect(page.locator('#governance-document')).toBeEnabled();
 const original=JSON.parse(await page.locator('#governance-document').inputValue());const changed=structuredClone(original);changed.metadata.version='deny-'+stamp;changed.spec.rules=[];
 await page.locator('#governance-document').fill(JSON.stringify(changed,null,2));await page.locator('#governance-save').click();await expect(page.locator('#governance-preview')).toBeVisible();await expect(page.locator('#governance-diff')).toContainText('Before:');await page.locator('#governance-confirm').click();await expect(page.locator('#governance-notice')).toContainText('activated for new Work');
 await page.screenshot({path:info.outputPath('administration-published.png'),fullPage:true});
 const denied=await create(page,name,'repo:acme/payments',objective,false);await expect(page.locator('.portal-work-permissions tr[data-permission-state=not-granted]')).toHaveCount(5);await expect(page.locator('.portal-head .portal-badge')).toHaveText('Denied');
 const deniedEvidence=await(await page.request.get(`/api/requests/${encodeURIComponent(denied)}/evidence`)).json();expect(deniedEvidence.request.spec).toEqual(evidenceBefore.request.spec);expect(deniedEvidence.state.policyRef.version).toBe(changed.metadata.version);expect(deniedEvidence.state.decision.result).toBe('Deny');await page.screenshot({path:info.outputPath('policy-denied.png'),fullPage:true});
 await page.goto('/admin#governance');await page.locator('#governance-project').selectOption('payments');await page.locator('#governance-load').click();await expect(page.locator('#governance-history')).toContainText(original.metadata.version);
 await page.locator('#governance-document').fill(JSON.stringify(original,null,2));await page.locator('#governance-save').click();await page.locator('#governance-confirm').click();await expect(page.locator('#governance-notice')).toContainText('activated for new Work');
 const history=await(await page.request.get('/api/governance?project=payments')).json();expect(history.active.version).toBe(original.metadata.version);expect(history.versions.some((v:{policy:{Version:string}})=>v.policy.Version===changed.metadata.version)).toBe(true);
 const oldEvidence=await(await page.request.get(`/api/requests/${encodeURIComponent(before)}/evidence`)).json();expect(oldEvidence.state.policyRef).toEqual(evidenceBefore.state.policyRef);expect(oldEvidence.state.effectiveAuthority).toEqual(evidenceBefore.state.effectiveAuthority);
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:info.outputPath('administration-mobile.png'),fullPage:true});
 await page.setViewportSize({width:1440,height:1000});const restored=await create(page,name,'repo:acme/payments',objective);const restoredEvidence=await(await page.request.get(`/api/requests/${encodeURIComponent(restored)}/evidence`)).json();expect(restoredEvidence.request.spec).toEqual(evidenceBefore.request.spec);expect(restoredEvidence.state.decision.result).toBe('Allow');
});
test('viewer and anonymous boundaries, unavailable telemetry and expanded marketing',async({page},info)=>{
 await page.goto('/');await expect(page.getByRole('heading',{name:/Give agents work/})).toBeVisible();await expect(page.getByRole('heading',{name:/An open tool today/})).toBeVisible();await expect(page.getByText('PLANNED · NOT AVAILABLE YET',{exact:true})).toBeVisible();
 expect((await page.request.get('/api/requests')).status()).toBe(401);await page.screenshot({path:info.outputPath('landing-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:info.outputPath('landing-mobile.png'),fullPage:true});await page.setViewportSize({width:1440,height:1000});
 await signIn(page,'viewer');expect((await page.request.post('/api/organization',{data:{displayName:'forged',description:''}})).status()).toBe(403);await page.goto('/?mode=connected&project=payments#/work/new');await expect(page.getByRole('button',{name:'Start work',exact:true})).toBeEnabled();
 await page.goto('/?mode=connected&project=payments#/platform');await expect(page.getByRole('heading',{name:'Observability',exact:true})).toBeVisible();await expect(page.getByText('No receiver configured')).toBeVisible();await page.screenshot({path:info.outputPath('platform-observability.png'),fullPage:true});
});
