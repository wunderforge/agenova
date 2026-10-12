// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Rendered login contract; synthetic transport, not a live Google acceptance test.
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
import { chromium } from '../../../ui/node_modules/playwright/index.mjs';
const root=resolve(import.meta.dirname,'../../..');
const css=await readFile(resolve(root,'internal/websession/login.css'),'utf8');
const js=await readFile(resolve(root,'internal/websession/google-login.js'),'utf8');
const html=(await readFile(resolve(root,'internal/websession/login.html'),'utf8')).replace('{{styles}}',css).replace('>Sign in</button>','>Continue with Google</button>').replace('</body>',`<script>${js}</script></body>`);
const hash=s=>createHash('sha256').update(s).digest('base64');
const csp=`default-src 'none'; style-src 'sha256-${hash(css)}'; script-src 'sha256-${hash(js)}'; form-action 'self'; frame-ancestors 'none'`;
const out=resolve(root,'.tmp/identity/google-ui');await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:900}});
 let payload;
 await page.route('https://identity.example/**',async route=>{
  const request=route.request();assert.equal(new URL(request.url()).pathname,'/login');
  if(request.method()==='POST'){payload=new URLSearchParams(request.postData());return route.fulfill({body:'Google redirect would occur here.'});}
  assert.equal(request.url().includes('invite='),false);
  return route.fulfill({contentType:'text/html',headers:{'Content-Security-Policy':csp,'Referrer-Policy':'no-referrer'},body:html});
 });
 const proof='a'.repeat(43);
 await page.goto(`https://identity.example/login#invite=${proof}`);
 await page.getByRole('button',{name:'Continue with Google'}).waitFor();
 assert.equal(await page.locator('input[name="invitation"]').inputValue(),proof);
 assert.equal(new URL(page.url()).hash,'');
 assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
 await page.screenshot({path:resolve(out,'desktop.png'),fullPage:true});
 await page.getByRole('button',{name:'Continue with Google'}).click();
 await page.getByText('Google redirect would occur here.').waitFor();
 assert.equal(payload.get('invitation'),proof);
 await page.goto('https://identity.example/login');
 await page.reload();
 await page.goto('https://identity.example/login#invite=invalid');
 await page.getByRole('alert').waitFor();
 assert.equal(await page.getByRole('button',{name:'Continue with Google'}).isDisabled(),true);
 await page.setViewportSize({width:390,height:844});
 await page.goto('https://identity.example/login');
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 await page.screenshot({path:resolve(out,'mobile.png'),fullPage:true});
 console.log('PASS Google login label, CSP, fragment removal, same-origin invitation POST, invalid-proof rejection and mobile layout. Synthetic transport only.');
}finally{await browser.close();}
