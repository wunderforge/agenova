// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test, type APIRequestContext } from '@playwright/test';
import type { View } from '../src/connected-source';

const developerURL = process.env.AGENOVA_DEVELOPER_UI || 'http://127.0.0.1:5178';
const sreURL = process.env.AGENOVA_SRE_UI || 'http://127.0.0.1:5179';
const sreDeniedURL = process.env.AGENOVA_SRE_DENIED_UI || 'http://127.0.0.1:5180';

async function evidence(request: APIRequestContext, base: string, ref: string): Promise<View> {
  const response = await request.get(`${base}/api/requests/${encodeURIComponent(ref)}/evidence`);
  expect(response.status()).toBe(200);
  return await response.json() as View;
}

test('Developer agent opens a real PR under its granted authority', async ({ page, request }, info) => {
  const ref = 'incident-dev-pr-allowed-r9';
  const work = await evidence(request, developerURL, ref);
  expect(work.state?.principal.team).toBe('payments-development');
  expect(work.outcome?.status).toBe('Succeeded');
  expect(work.state?.effectiveAuthority?.tools).toContain('github.pr.create');
  expect(work.state?.effectiveAuthority?.tools).not.toContain('kubernetes.rollback');
  const creation = work.facts.find(f => f.kind === 'ProviderOutcome' && f.operation === 'tool.invoke' && f.target?.includes('/pull/1'));
  expect(creation).toMatchObject({ providerStatus: 'Succeeded', target: 'https://github.com/wunderforge/agenova-payment-incident-demo/pull/1' });
  expect(work.outcome?.text).toContain('https://github.com/wunderforge/agenova-payment-incident-demo/pull/1');
  await page.goto(`${developerURL}/?mode=connected#/work/${ref}`);
  await expect(page.locator('.portal-result')).toContainText('/pull/1');
  await expect(page.getByRole('heading', { name: 'Agent activity' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('developer-pr-work.png'), fullPage: true });
});

test('SRE agent performs a real isolated rollback under its granted authority', async ({ page, request }, info) => {
  const ref = 'incident-sre-rollback-allowed-r2';
  const work = await evidence(request, sreURL, ref);
  expect(work.state?.principal.team).toBe('payments-reliability');
  expect(work.outcome?.status).toBe('Succeeded');
  expect(work.state?.effectiveAuthority?.tools).toContain('kubernetes.rollback');
  expect(work.state?.effectiveAuthority?.tools).not.toContain('github.pr.create');
  const rollback = work.facts.find(f => f.kind === 'ProviderOutcome' && f.operation === 'tool.invoke' && f.target?.includes('revision 3 (v2.6)'));
  expect(rollback).toMatchObject({ providerStatus: 'Succeeded' });
  await page.goto(`${sreURL}/?mode=connected#/work/${ref}`);
  await expect(page.getByRole('heading', { name: 'Agent activity' })).toBeVisible();
  await page.getByRole('link', { name: 'View records' }).click();
  await expect(page.getByText(/revision 3 \(v2\.6\)/)).toBeVisible();
  await page.screenshot({ path: info.outputPath('sre-rollback-records.png'), fullPage: true });
});

for (const scenario of [
  { name: 'Developer', base: developerURL, ref: 'incident-dev-rollback-denied-r2', team: 'payments-development', tool: 'kubernetes.rollback' },
  { name: 'SRE', base: sreDeniedURL, ref: 'incident-sre-pr-denied-r2', team: 'payments-reliability', tool: 'github.pr.create' },
]) {
  test(`${scenario.name} forbidden operation is denied before its provider`, async ({ page, request }, info) => {
    const work = await evidence(request, scenario.base, scenario.ref);
    expect(work.state?.principal.team).toBe(scenario.team);
    const denied = work.facts.find(f => f.kind === 'ToolDecision' && f.target === scenario.tool && f.result === 'Deny');
    expect(denied).toMatchObject({ reasonCode: 'tool-not-granted' });
    expect(work.facts.some(f => f.kind === 'ProviderAttempt' && f.operation === 'tool.invoke' && f.invocationId === denied?.invocationId)).toBe(false);
    await page.goto(`${scenario.base}/?mode=connected#/work/${scenario.ref}`);
    await expect(page.locator('.portal-blocked-attempt')).toContainText(scenario.tool);
    await page.getByRole('link', { name: 'Inspect denial record' }).click();
    await expect(page.getByText('Deny', { exact: true }).first()).toBeVisible();
    await page.screenshot({ path: info.outputPath(`${scenario.name.toLowerCase()}-denied-record.png`), fullPage: true });
  });
}
