// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Rendered login contract; synthetic transport, not a live Google acceptance test.
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { chromium } from '../../../ui/node_modules/playwright/index.mjs';
const root=resolve(import.meta.dirname,'../../..');
const css=await readFile(resolve(root,'internal/websession/login.css'),'utf8');
const js=await readFile(resolve(root,'internal/websession/google-login.js'),'utf8');
const html=(await readFile(resolve(root,'internal/websession/login.html'),'utf8')).replace('{{styles}}',css).replace('>Sign in</button>','>Continue with Google</button>').replace('</body>',`<script>${js}</script></body>`);
const hash=s=>createHash('sha256').update(s).digest('base64');
const out=resolve(root,'.tmp/identity/google-ui');await mkdir(out,{recursive:true});
let payload, origin, provider;
const server=createServer(async(req,res)=>{
 if(req.url==='/authorize'){res.writeHead(200,{'Content-Type':'text/plain'});res.end('Google authorization reached.');return;}
 assert.equal(req.url,'/login');
 if(req.method==='POST'){
  assert.equal(req.headers.origin,origin,'same-origin form must preserve Origin for CSRF validation');
  let body='';for await(const chunk of req)body+=chunk;
  payload=new URLSearchParams(body);res.writeHead(303,{Location:`${provider}/authorize`});res.end();return;
 }
 const csp=`default-src 'none'; style-src 'sha256-${hash(css)}'; script-src 'sha256-${hash(js)}'; form-action 'self' ${provider}; frame-ancestors 'none'`;
 res.writeHead(200,{'Content-Type':'text/html','Content-Security-Policy':csp,'Referrer-Policy':'same-origin'});res.end(html);
});
await new Promise(resolve=>server.listen(0,'0.0.0.0',resolve));
origin=`http://127.0.0.1:${server.address().port}`;
provider=`http://localhost:${server.address().port}`;
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:900}});
 page.setDefaultTimeout(5000);
 const proof='a'.repeat(43);
 await page.goto(`${origin}/login#invite=${proof}`);
 await page.getByRole('button',{name:'Continue with Google'}).waitFor();
 assert.equal(await page.locator('input[name="invitation"]').inputValue(),proof);
 assert.equal(new URL(page.url()).hash,'');
 assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
 await page.screenshot({path:resolve(out,'desktop.png'),fullPage:true});
 await page.getByRole('button',{name:'Continue with Google'}).click();
 await page.getByText('Google authorization reached.').waitFor({timeout:5000});
 assert.equal(payload.get('invitation'),proof);
 await page.goto(`${origin}/login`);
 await page.reload();
 await page.goto(`${origin}/login#invite=invalid`);
 await page.getByRole('alert').waitFor();
 assert.equal(await page.getByRole('button',{name:'Continue with Google'}).isDisabled(),true);
 await page.setViewportSize({width:390,height:844});
 await page.goto(`${origin}/login`);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 await page.screenshot({path:resolve(out,'mobile.png'),fullPage:true});
 console.log('PASS Google login label, CSP, fragment removal, same-origin invitation POST, invalid-proof rejection and mobile layout. Synthetic transport only.');
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
