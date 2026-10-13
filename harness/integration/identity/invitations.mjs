// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Executable rendered UI contract with synthetic API responses; not live SSO proof.
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
import { chromium } from '../../../ui/node_modules/playwright/index.mjs';
const root = resolve(import.meta.dirname, '../../..');
const html = await readFile(resolve(root, 'internal/invitations/page.html'), 'utf8');
const css = await readFile(resolve(root, 'internal/invitations/page.css'), 'utf8');
const js = await readFile(resolve(root, 'internal/invitations/page.js'), 'utf8');
const pageHTML = html.replace('{{styles}}', css).replace('{{script}}', js);
const out = resolve(root, '.tmp/identity/invitation-ui');
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
 const page = await browser.newPage({ viewport: { width: 1100, height: 950 } });
 let invitations = [];
 let memberChanges = [];
 const members = [
  {principal:'owner',email:'owner@example.com',accountRole:'owner',active:true,projects:[{project:'payments',role:'admin'}]},
  {principal:'admin',email:'admin@example.com',accountRole:'admin',active:true,projects:[{project:'payments',role:'admin'}]},
  {principal:'member',email:'member@example.com',accountRole:'member',active:true,projects:[{project:'payments',role:'member'}]}
 ];
 let deny = false;
 let google = false;
 let posts = 0;
 let loggedOut = false;
 await page.route('https://identity.example/**', async route => {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  if(path==='/auth/me') return route.fulfill({contentType:'application/json',body:JSON.stringify({subject:'owner',team:'team',authenticationContext:'oidc'})});
  if(path==='/api/members') {
   if(request.method()==='POST') {
    const change=request.postDataJSON();memberChanges.push(change);
    const target=members.find(m=>m.principal===change.principal);assert.ok(target);
    if(change.action==='disable') target.active=false;
    if(change.action==='enable') target.active=true;
    if(change.action==='account-role') target.accountRole=change.role;
    if(change.action==='grant-role') target.roles=[{role:change.role,scopeKind:change.scopeKind,scopeRef:change.scopeRef}];
    if(change.action==='revoke-role') target.roles=[];
    return route.fulfill({status:204});
   }
   return route.fulfill({contentType:'application/json',body:JSON.stringify(members)});
  }
  if(path==='/api/invitations/manage') {
   const {id,action}=request.postDataJSON();assert.equal(id,'fixture');
   const item=invitations.at(-1);item.status=action==='cancel'?'cancelled':'pending';
   return route.fulfill({contentType:'application/json',body:JSON.stringify({...item,...(action==='reissue'?{acceptanceUrl:`https://identity.example/login#invite=${'B'.repeat(43)}`}:{})})});
  }
  if (path === '/members') return route.fulfill({ contentType: 'text/html', body: pageHTML });
  if (path === '/api/invitations') {
   if (request.method() === 'POST') {
    posts++;
    const payload = request.postDataJSON();
    assert.deepEqual(Object.keys(payload).sort(), ['accountRole', 'email', 'project', 'role']);
    if (deny) return route.fulfill({ status: 403, body: 'Access denied' });
    assert.equal(payload.email, 'invited@example.com');
    assert.equal(payload.project, 'payments');
    assert.equal(payload.role, 'admin');assert.equal(payload.accountRole,'member');
    invitations.push({ ...payload, id: 'fixture', status: google ? 'pending' : 'sent',linkManaged:google });
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({...invitations.at(-1), ...(google ? {acceptanceUrl: `https://identity.example/login#invite=${"A".repeat(43)}`} : {})}) });
   }
   return route.fulfill({ contentType: 'application/json', body: JSON.stringify(invitations) });
  }
  if (path === '/logout') { assert.equal(request.method(), 'POST'); loggedOut = true; return route.fulfill({ status: 200, body: 'Signed out' }); }
  throw new Error(`Unexpected browser request: ${path}`);
 });
 await page.goto('https://identity.example/members');
 await page.getByText('No invitations yet.').waitFor();
 await page.getByRole('link',{name:'Roles & permissions',exact:true}).click();
 assert.equal(await page.locator('#roles .role-card').count(),3);
 assert.equal(await page.getByText('Governance access (Governance Admin)',{exact:false}).isVisible(),false);

 assert.equal(await page.locator('#roles input, #roles select, #roles button').count(),0);
 assert.equal(await page.getByText('Team administrator includes governance for administered projects and deployment access to this service’s installation. AWS/Kubernetes access is still required.',{exact:true}).count(),1);
 await page.screenshot({path:resolve(out,'roles-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 await page.locator('#roles').screenshot({path:resolve(out,'roles-mobile.png')});
 await page.getByRole('link',{name:'Assign roles to members ↓',exact:true}).click();
 assert.equal(new URL(page.url()).hash,'#member-section');
 await page.setViewportSize({width:1100,height:950});
 await page.getByLabel('Email address').fill('invited@example.com');
 await page.getByLabel('Project', { exact: true }).fill('payments');
 assert.equal(await page.getByLabel('Access',{exact:true}).inputValue(),'developer');
 await page.getByText('Advanced role mapping',{exact:true}).click();
 await page.getByLabel('Project role', { exact: true }).selectOption('admin');
 await page.getByRole('button', { name: 'Create invitation' }).click();
 await page.getByText('Invitation sent.', { exact: false }).waitFor();
 await page.getByText('invited@example.com', { exact: true }).waitFor();
 assert.equal(posts, 1);
 await page.screenshot({ path: resolve(out, 'desktop.png'), fullPage: true });
 google = true;
 await page.getByLabel('Email address').fill('invited@example.com');
 await page.getByRole('button', {name: 'Create invitation'}).click();
 await page.getByText('No email has been sent.', {exact:false}).waitFor();
 assert.equal(await page.getByLabel('One-time invitation link').inputValue(), `https://identity.example/login#invite=${"A".repeat(43)}`);
 assert.equal(await page.getByLabel('One-time invitation link').getAttribute('readonly'), '');
 await page.setViewportSize({width:390,height:844});
 assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
 await page.getByRole('button',{name:'Cancel invitation for invited@example.com'}).click();
 await page.getByText('Invitation cancelled.',{exact:false}).waitFor();
 await page.getByRole('button',{name:'Replace link for invited@example.com'}).click();
 await page.waitForFunction(() => document.querySelector('[aria-label="One-time invitation link"]')?.value.endsWith('B'.repeat(43)));
 const member=page.locator('article.member').filter({has:page.getByRole('heading',{name:'member@example.com',exact:true})});
 await member.getByText('Manage access',{exact:true}).click();
 await member.getByRole('button',{name:'Disable member',exact:true}).click();
 await page.getByText('Member disabled.',{exact:false}).waitFor();
 await member.getByText('Manage access',{exact:true}).click();
 await member.getByRole('button',{name:'Enable member',exact:true}).click();
 await page.getByText('Member access updated.',{exact:true}).waitFor();
 await member.getByText('Manage access',{exact:true}).click();
 await member.getByText('Advanced permissions',{exact:true}).click();
 await member.getByLabel('member@example.com account role').selectOption('admin');
 await member.getByLabel('member@example.com account role').locator('..').locator('..').getByRole('button',{name:'Save',exact:true}).click();
 await page.waitForFunction(() => [...document.querySelectorAll('article.member')].some(el => el.textContent.includes('member@example.com') && el.textContent.includes('admin · Active')));
 assert.deepEqual(memberChanges,[{principal:'member',action:'disable'},{principal:'member',action:'enable'},{principal:'member',action:'account-role',role:'admin'}]);
 await member.getByText('Advanced permissions',{exact:true}).click();
 await member.getByLabel('Scope reference for member@example.com').fill('payments');
 await member.getByRole('button',{name:'Grant advanced permission',exact:true}).click();
 await member.getByText('1 advanced permission(s) assigned',{exact:true}).waitFor();
 assert.deepEqual(memberChanges.at(-1),{principal:'member',action:'grant-role',role:'governance-admin',scopeKind:'project',scopeRef:'payments'});
 await member.getByText('Advanced permissions',{exact:true}).click();
 await member.getByRole('button',{name:'Revoke role',exact:true}).click();
 await member.getByText('governance-admin · project: payments',{exact:false}).waitFor({state:'hidden'});
 assert.equal(await page.evaluate(() => document.documentElement.scrollWidth<=innerWidth),true);
 await page.screenshot({path:resolve(out,'google-mobile.png'),fullPage:true});
 await page.setViewportSize({width:1100,height:950});
 deny = true;
 await page.getByLabel('Email address').fill('another@example.com');
 await page.getByRole('button', { name: 'Create invitation' }).click();
 await page.getByText('Your role cannot send this invitation.', { exact: false }).waitFor();
 assert.equal(posts, 3);
 invitations.push({ email: '<img src=x onerror=alert(1)>', project: 'test', role: 'viewer', status: 'uncertain' });
 await page.getByRole('button', { name: 'Refresh', exact: true }).click();
 await page.getByText('<img src=x onerror=alert(1)>', { exact: true }).waitFor();
 assert.equal(await page.locator('#invitations img').count(), 0);
 invitations.pop();
 await page.getByRole('button', { name: 'Refresh', exact: true }).click();
 await page.getByText('<img src=x onerror=alert(1)>', { exact: true }).waitFor({state:'hidden'});
 await page.setViewportSize({ width: 390, height: 844 });
 assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
 await page.screenshot({ path: resolve(out, 'mobile.png'), fullPage: true });
 await page.getByRole('button', { name: 'Log out', exact: true }).click();
 await page.waitForURL('**/logout');
 assert.equal(loggedOut, true);
 console.log('PASS invitation form payload, role denial, escaped rendering, mobile layout and POST logout. API responses are synthetic.');
} finally { await browser.close(); }
