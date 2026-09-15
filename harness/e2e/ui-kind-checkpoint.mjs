// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Opt-in real browser -> local console -> kind -> Ollama acceptance. Not part
// of no-cost synthetic CI: the explicit flag prevents accidental live calls.
import { createRequire } from 'node:module';
import { mkdir,writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { strict as assert } from 'node:assert';

const args=process.argv.slice(2);
const baseIndex=args.indexOf('--base-url');
if(!args.includes('--live-model')||baseIndex<0||!args[baseIndex+1])throw new Error('Explicit --live-model and --base-url are required.');
const base=new URL(args[baseIndex+1]);
if(base.protocol!=='http:'||!['127.0.0.1','[::1]'].includes(base.hostname))throw new Error('Only an explicit literal loopback console is supported.');
const require=createRequire(new URL('../../ui/package.json',import.meta.url));
const {chromium,expect}=require('@playwright/test');
const output=fileURLToPath(new URL('../../.tmp/ui-kind-checkpoint/',import.meta.url));
await mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true});
const page=await browser.newPage({viewport:{width:1280,height:1000}});
page.setDefaultTimeout(180_000);
const objective='Explain in one short sentence why retries need a time limit.';
try{
  await page.goto(new URL('/?mode=connected#/work',base).href);
  await expect(page.getByRole('heading',{name:'Work',exact:true})).toBeVisible();
  await page.getByRole('link',{name:'New work',exact:true}).click();
  await page.getByLabel('What should it do?').fill(objective);
  await page.getByLabel('Time limit (minutes)').fill('45');
  const responsePromise=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/requests'&&r.request().method()==='POST');
  await page.getByRole('button',{name:'Start work',exact:true}).click();
  const response=await responsePromise;
  assert.equal(response.status(),202,'Canonical task submission was rejected.');
  const posted=response.request().postDataJSON();
  assert.equal(posted.spec.task.input.objective,objective);
  assert.equal(posted.spec.requestedAccess.resourceScopes[0],'repo:acme/payments');
  assert.equal(JSON.stringify(posted).includes('principal'),false);
  const initial=await response.json();
  await expect(page.getByRole('heading',{name:objective,exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'Result',exact:true})).toBeVisible({timeout:180_000});
  const final=await page.evaluate(async ref=>{
    const response=await fetch(`/api/requests/${encodeURIComponent(ref)}/evidence`);
    if(!response.ok)throw new Error('Final evidence query failed.');
    return response.json();
  },initial.requestRef);
  assert.equal(final.state.claim.phase,'Succeeded');
  assert.equal(final.outcome.status,'Succeeded');
  assert.equal(final.outcome.failure,undefined);
  assert.ok(final.outcome.text.trim());
  assert.equal(final.outcome.model.model,'llama3.1:latest');
  assert.ok(final.outcome.model.inputTokens>0&&final.outcome.model.outputTokens>0);
  assert.equal(final.state.effectiveAuthority.runtime.timeout,'30m0s');
  assert.ok(final.facts.some(f=>f.kind==='Runtime'&&f.operation==='CleanupSucceeded'));
  assert.ok(final.facts.some(f=>f.kind==='ProviderOutcome'&&f.providerStatus==='Succeeded'&&f.invocationId===final.outcome.model.invocationId));
  await page.screenshot({path:`${output}/real-result.png`,fullPage:true});
  await page.getByRole('link',{name:'Compare requested and granted',exact:true}).click();
  await expect(page.getByText('standard-isolated · 45m0s',{exact:true})).toBeVisible();
  await expect(page.getByText('standard-isolated · 30m0s',{exact:true})).toBeVisible();
  await page.screenshot({path:`${output}/real-authority.png`,fullPage:true});
  await page.goto(new URL('/?mode=connected#/platform',base).href);
  await expect(page.getByRole('row',{name:/Model Gateway/})).toContainText('Activity recorded');
  await expect(page.getByRole('row',{name:/Tool Gateway/})).toContainText('Not connected');
  await page.screenshot({path:`${output}/real-platform.png`,fullPage:true});
  await writeFile(`${output}/evidence.json`,`${JSON.stringify(final,null,2)}\n`);
  console.log(JSON.stringify({status:'PASS',requestRef:final.requestRef,claimId:final.state.claim.id,worker:final.state.claim.backendIdentity,model:final.outcome.model,result:final.outcome.text,facts:final.facts.length,cleanup:'confirmed',screenshots:output}));
}finally{await page.close();await browser.close();}
