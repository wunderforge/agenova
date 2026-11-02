// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Opt-in read-only verification against the same installed Work as CLI/API.
import {createRequire} from 'node:module';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {execFileSync} from 'node:child_process';
import assert from 'node:assert/strict';
const argv=process.argv.slice(2);
const option=name=>{const i=argv.indexOf(name);if(i<0||!argv[i+1])throw new Error(`Required ${name}`);return argv[i+1];};
const base=new URL(option('--base-url'));
if(base.protocol!=='http:'||base.hostname!=='127.0.0.1')throw new Error('Explicit loopback UI required');
const evidence=resolve(option('--evidence-dir'));
const state=resolve(option('--state-dir'));
const cli=resolve(option('--cli'));
const require=createRequire(new URL('../../ui/package.json',import.meta.url));
const {chromium,expect}=require('@playwright/test');
await mkdir(`${evidence}/ui`,{recursive:true});
const browser=await chromium.launch({headless:true});
const page=await browser.newPage({viewport:{width:1440,height:1100}});
page.setDefaultTimeout(30_000);
const pageErrors=[];page.on('pageerror',e=>pageErrors.push(e.message));
const results=[];
try{
 const setupResponse=await page.request.get(new URL('/api/setup',base).href);
 assert.equal(setupResponse.status(),200);
 const setup=await setupResponse.json();
 assert.equal(setup.installation.kind,'installed');
 assert.equal(setup.installation.platform,'e15-198-supplement');
 for(const [name,status] of [['control','Succeeded'],['profile','Failed'],['denied','Denied']]){
  const captured=JSON.parse(await readFile(`${evidence}/${name}.cli.stdout`,'utf8'));
  const ref=captured.requestRef;
  const live=JSON.parse(execFileSync(cli,['work','show',ref,'--state-dir',state,'--json'],{encoding:'utf8',timeout:60_000}));
  const response=await page.request.get(new URL(`/api/requests/${encodeURIComponent(ref)}/evidence`,base).href);
  assert.equal(response.status(),200);
  const api=await response.json();
  assert.deepEqual(api,live);assert.deepEqual(api,captured);
  await page.goto(new URL(`/?mode=connected#/work/${encodeURIComponent(ref)}`,base).href);
  await page.locator('summary').filter({hasText:'Execution details'}).click();
  await expect(page.getByText(ref,{exact:true}).first()).toBeVisible();
  await expect(page.getByText(status,{exact:true}).first()).toBeVisible();
  if(status==='Denied'){
   assert.equal(api.state.claim,undefined);
   await page.locator('summary').filter({hasText:'Access & limits'}).click();
   await expect(page.getByRole('heading',{name:'No access issued',exact:true})).toBeVisible();
   await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
  }else{
   await expect(page.getByText(api.state.claim.id,{exact:true}).first()).toBeVisible();
   assert.equal(api.state.claim.phase,status);
   if(status==='Succeeded'){
    await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible();
    await expect(page.locator('.portal-result pre')).toHaveText(api.outcome.text);
   }else{
    await expect(page.getByText(api.outcome.failure,{exact:true}).first()).toBeVisible();
    await expect(page.getByRole('heading',{name:'Result',exact:true})).toHaveCount(0);
   }
  }
  await page.screenshot({path:`${evidence}/ui/${name}.png`,fullPage:true});
  await writeFile(`${evidence}/ui/${name}.visible.txt`,await page.locator('body').innerText());
  results.push({case:name,requestRef:ref,status,cliApiExactMatch:true,renderedIdentifiersAndOutcome:true,screenshot:`${name}.png`});
  if(name==='profile'){
   await page.goto(new URL(`/?mode=connected#/work/${encodeURIComponent(ref)}/access`,base).href);
   await expect(page.getByRole('heading',{name:'Access for this work',exact:true})).toBeVisible();
   const requested=page.locator('.portal-compare-side').nth(0);
   const granted=page.locator('.portal-compare-side').nth(1);
   await expect(requested).toContainText('git.write');
   await expect(requested).toContainText('10m0s');
   await expect(granted).not.toContainText('git.write');
   await expect(granted).toContainText('git.read');
   await expect(granted).toContainText('5m0s');
   await page.screenshot({path:`${evidence}/ui/template-narrowing.png`,fullPage:true});
  }
 }
 assert.deepEqual(pageErrors,[]);
 await writeFile(`${evidence}/ui/result.json`,JSON.stringify({status:'PASS',source:setup.installation,checks:results,pageErrors,authorityNarrowing:'template ceiling only; not Policy constraints'},null,2)+'\n');
 console.log(JSON.stringify({status:'PASS',cases:results.length,cliApiParity:true,screenshots:`${evidence}/ui` }));
}finally{await browser.close();}
