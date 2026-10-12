// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Rendered contract with synthetic identities, not real Google acceptance.
import {readFile,mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import assert from 'node:assert/strict';
import {chromium} from '../../../ui/node_modules/playwright/index.mjs';
const root=resolve(import.meta.dirname,'../../..');
const html=(await readFile(resolve(root,'internal/invitations/page.html'),'utf8')).replace('{{styles}}',await readFile(resolve(root,'internal/invitations/page.css'),'utf8')).replace('{{script}}',await readFile(resolve(root,'internal/invitations/page.js'),'utf8')).replace('{{registration}}','true');
const out=resolve(root,'.tmp/identity/access-ui');await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:900}});let approved=false;
 await page.route('https://identity.example/**',async route=>{
  const req=route.request();const path=new URL(req.url()).pathname;
  if(path==='/members')return route.fulfill({contentType:'text/html',body:html});
  if(path==='/auth/me')return route.fulfill({json:{subject:'owner',team:'team'}});
  if(path==='/api/members')return route.fulfill({json:[{principal:'owner',email:'owner@example.com',accountRole:'owner',active:true,projects:[{project:'payments',role:'admin'}]}]});
  if(path==='/api/access-requests'){
   if(req.method()==='POST'){assert.deepEqual(req.postDataJSON(),{id:'request',action:'approve',accountRole:'admin',project:'payments',role:'admin'});approved=true;return route.fulfill({status:204});}
   return route.fulfill({json:approved?[]:[{id:'request',email:'pending@example.com',status:'pending'}]});
  }
  throw new Error(`Unexpected request ${path}`);
 });
 await page.goto('https://identity.example/members');await page.getByText('1 pending requests.',{exact:true}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Create invitation'}).isVisible(),false);
 await page.getByLabel('pending@example.com Account role').selectOption('admin');
 await page.getByLabel('pending@example.com project',{exact:true}).fill('payments');
 await page.getByLabel('pending@example.com Project role').selectOption('admin');
 await page.locator('#access-section').screenshot({path:resolve(out,'approval-desktop.png')});
 await page.setViewportSize({width:390,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 await page.locator('#access-section').screenshot({path:resolve(out,'approval-mobile.png')});
 await page.getByRole('button',{name:'Approve access'}).click();await page.getByText('Access approved.',{exact:false}).waitFor();assert.equal(approved,true);
 console.log('PASS access request UI: pending list, explicit role approval, no invitation, mobile layout');
}finally{await browser.close();}
