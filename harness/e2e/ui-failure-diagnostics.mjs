// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Read-only verification of retained real failure evidence; never submits work.
import { createRequire } from 'node:module';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { strict as assert } from 'node:assert';
const args=process.argv.slice(2);
const arg=name=>args[args.indexOf(name)+1];
if(!args.includes('--base-url')||!args.includes('--request-ref'))throw new Error('Explicit --base-url and --request-ref are required.');
const base=new URL(arg('--base-url'));
if(base.protocol!=='http:'||!['127.0.0.1','[::1]'].includes(base.hostname))throw new Error('Only literal loopback is supported.');
const ref=arg('--request-ref');
const require=createRequire(new URL('../../ui/package.json',import.meta.url));
const {chromium,expect}=require('@playwright/test');
const output=fileURLToPath(new URL('../../.tmp/failure-diagnostics/',import.meta.url));
await mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true});
try {
  const page=await browser.newPage({viewport:{width:1280,height:1100}});
  await page.goto(new URL('/?mode=connected#/work/'+encodeURIComponent(ref),base).href);
  const evidence=await page.evaluate(async ref=>{
    const r=await fetch('/api/requests/'+encodeURIComponent(ref)+'/evidence');
    if(!r.ok)throw new Error('Evidence unavailable');
    return r.json();
  },ref);
  assert.equal(evidence.outcome?.status,'Failed');
  const failure=evidence.facts.findLast(f=>f.kind==='RunOutcome');
  assert.ok(failure?.reason&&failure.reason===evidence.outcome.failure,'No correlated failure reason; rebuild/restart the backend.');
  assert.ok(evidence.facts.some(f=>f.operation==='ActionValidated'),'Current action diagnostics missing.');
  await expect(page.locator('.portal-agent-failure')).toContainText(failure.reason);
  await expect(page.locator('.portal-worker')).toContainText('Retry required');
  assert.equal(await page.evaluate(()=>document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity).length),0);
  await page.screenshot({path:output+'/work.png',fullPage:true});
  await page.getByRole('link',{name:'View failure record',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText(failure.reason);
  await page.screenshot({path:output+'/record.png',fullPage:true});
  console.log(JSON.stringify({status:'PASS',requestRef:ref,code:failure.reasonCode,reason:failure.reason,output}));
}finally{await browser.close();}
