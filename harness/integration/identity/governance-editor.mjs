// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Browser proof uses synthetic configuration; live deployment is verified separately.
import {readFile,mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import assert from 'node:assert/strict';
import {chromium} from '../../../ui/node_modules/playwright/index.mjs';
const root=resolve(import.meta.dirname,'../../..');
const html=(await readFile(resolve(root,'internal/invitations/page.html'),'utf8')).replace('{{styles}}',await readFile(resolve(root,'internal/invitations/page.css'),'utf8')).replace('{{script}}',await readFile(resolve(root,'internal/invitations/page.js'),'utf8')).replace('{{registration}}','true');
const out=resolve(root,'.tmp/identity/governance-editor');await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1280,height:1000}});let status=200,posts=[];
 await page.route('https://identity.example/**',async route=>{const r=route.request(),u=new URL(r.url());
 if(u.pathname==='/admin')return route.fulfill({contentType:'text/html',body:html});
 if(u.pathname==='/auth/me')return route.fulfill({json:{subject:'admin',team:'team'}});
 if(u.pathname==='/api/members')return route.fulfill({json:[{principal:'admin',accountRole:'admin',email:'admin@example.com',active:true,projects:[{project:'payments',role:'admin'}],roles:[]}]});
 if(u.pathname==='/api/access-requests')return route.fulfill({json:[]});
 if(u.pathname==='/api/setup')return route.fulfill({json:{project:u.searchParams.get('project'),policy:{ID:'policy',Version:'1',Rules:[{team:'team',project:'payments',action:'claim.create',templateRef:'engineer'}]},template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name:'engineer'},spec:{}}}});
 if(u.pathname==='/api/governance'){posts.push(r.postDataJSON());return route.fulfill({status,json:status===200?{name:'saved'}:{}});}
 throw new Error(u.pathname);
 });
 await page.goto('https://identity.example/admin');await page.getByText('1 users.',{exact:false}).waitFor();
 const load=page.getByRole('button',{name:'Load current configuration'}),save=page.getByRole('button',{name:'Save configuration'}),editor=page.getByLabel('Configuration JSON');
 assert.equal(await save.isEnabled(),false);await load.click();await page.getByText('Ready to edit.',{exact:false}).waitFor();
 let doc=JSON.parse(await editor.inputValue());doc.metadata.version='2';await editor.fill(JSON.stringify(doc,null,2));await save.click();await page.getByText('Configuration saved and activated for new Work.',{exact:true}).waitFor();assert.equal(posts.at(-1).project,'payments');assert.equal(JSON.parse(posts.at(-1).document).metadata.version,'2');
 await editor.fill('{');await save.click();await page.getByText('Invalid JSON.',{exact:false}).waitFor();assert.equal(posts.length,1);
 await editor.fill(JSON.stringify(doc));status=409;await save.click();await page.getByText('Version conflict.',{exact:false}).waitFor();
 status=403;await save.click();await page.getByText('Permission denied.',{exact:false}).waitFor();
 await page.getByLabel('Document type').selectOption('template');assert.equal(await save.isEnabled(),false);await load.click();await page.getByText('Ready to edit.',{exact:false}).waitFor();doc=JSON.parse(await editor.inputValue());doc.metadata.name='engineer-v2';await editor.fill(JSON.stringify(doc,null,2));status=200;await save.click();await page.getByText('Configuration saved. Select this template', {exact:false}).waitFor();assert.equal(posts.at(-1).kind,'template');
 await page.screenshot({path:resolve(out,'desktop.png'),fullPage:true});await page.setViewportSize({width:390,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);await page.screenshot({path:resolve(out,'mobile.png'),fullPage:true});
 await page.locator('#governance-project').fill('other');assert.equal(await save.isEnabled(),false);assert.equal(await editor.inputValue(),'');
 console.log('PASS governance editor: policy/template saves, invalid JSON, conflict, denial, target reset, dark desktop/mobile');
}finally{await browser.close();}
