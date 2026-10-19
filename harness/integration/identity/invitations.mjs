// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Administration UI proof with synthetic API responses, not live Google proof.
import {readFile,mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import assert from 'node:assert/strict';
import {chromium} from '../../../ui/node_modules/playwright/index.mjs';
const root=resolve(import.meta.dirname,'../../..');
const html=(await readFile(resolve(root,'internal/invitations/page.html'),'utf8')).replace('{{styles}}',await readFile(resolve(root,'internal/invitations/page.css'),'utf8')).replace('{{script}}',await readFile(resolve(root,'internal/invitations/page.js'),'utf8')).replace('{{registration}}','true');
const out=resolve(root,'.tmp/identity/admin-ui');await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:900}});let actor='owner',changes=[],deny=false;
 const rows=[{principal:'owner',email:'owner@example.com',accountRole:'owner',active:true,projects:[{project:'payments',role:'admin'}],roles:[]},{principal:'admin',email:'admin@example.com',accountRole:'admin',active:true,projects:[{project:'payments',role:'admin'}],roles:[]},{principal:'member',email:'member@example.com',accountRole:'member',active:true,projects:[{project:'payments',role:'member'}],roles:[]}];
 await page.route('https://identity.example/**',async route=>{const r=route.request(),path=new URL(r.url()).pathname;
 if(path==='/admin')return route.fulfill({contentType:'text/html',body:html});
 if(path==='/auth/me')return route.fulfill({json:{subject:actor,team:'team'}});
 if(path==='/api/access-requests')return route.fulfill({json:[]});
 if(path==='/api/members'){if(r.method()==='POST'){changes.push(r.postDataJSON());return route.fulfill({status:deny?403:204});}return route.fulfill({json:rows});}
 throw new Error(path);});
 await page.goto('https://identity.example/admin');await page.getByText('3 users.',{exact:false}).waitFor();
 assert.equal(await page.getByRole('heading',{name:'Administration',exact:true}).count(),1);
 assert.deepEqual(await page.getByLabel('Role for member@example.com').locator('option').allTextContents(),['Member','Platform Operator','Platform Admin']);
 assert.equal(await page.getByLabel('Role for owner@example.com').count(),0);
 await page.getByLabel('Role for member@example.com').selectOption('platform-operator');
 await page.locator('article').filter({has:page.getByRole('heading',{name:'member@example.com',exact:true})}).getByRole('button',{name:'Save role'}).click();
 await page.getByText('User access updated.').waitFor();assert.deepEqual(changes.at(-1),{principal:'member',action:'platform-role',role:'platform-operator',project:'payments'});
 await page.screenshot({path:resolve(out,'desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);await page.screenshot({path:resolve(out,'mobile.png'),fullPage:true});
 actor='admin';await page.reload();await page.getByText('3 users.',{exact:false}).waitFor();assert.equal(await page.getByLabel('Role for admin@example.com').count(),0);assert.equal(await page.getByLabel('Role for member@example.com').locator('option[value="platform-admin"]').count(),1);
 deny=true;await page.locator('article').filter({has:page.getByRole('heading',{name:'member@example.com',exact:true})}).getByRole('button',{name:'Save role'}).click();await page.getByText('Change not confirmed.',{exact:false}).waitFor();
 console.log('PASS administration UI: canonical roles, protected Owner, denied writes, desktop/mobile');
}finally{await browser.close();}
