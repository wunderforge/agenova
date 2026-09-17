// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import type { Setup, View } from '../src/connected-source';

const allowedRef = process.env.AGENOVA_LIVE_REQUEST_REF;
const deniedRef = process.env.AGENOVA_LIVE_DENIED_REF;
const cliPath = process.env.AGENOVA_CLI_PATH;

function cliJSON(args: string[]): unknown {
  if (!cliPath) throw new Error('Set AGENOVA_CLI_PATH to the CLI binary used for the installed Work run.');
  return JSON.parse(execFileSync(cliPath, args, { encoding: 'utf8', timeout: 60_000, windowsHide: true }));
}

test('installed API, CLI reference and Portal show the same allowed Work', async ({ page, request }, info) => {
  test.skip(!allowedRef, 'Set AGENOVA_LIVE_REQUEST_REF after a real CLI run.');
  const setupResponse = await request.get('/api/setup');
  expect(setupResponse.status()).toBe(200);
  const setup = await setupResponse.json() as Setup;
  expect(setup.installation.kind).toBe('installed');
  expect(setup.installation.revision).toMatch(/^sha256:[a-f0-9]{64}$/);

  const detailResponse = await request.get(`/api/requests/${encodeURIComponent(allowedRef!)}/evidence`);
  expect(detailResponse.status()).toBe(200);
  const detail = await detailResponse.json() as View;
  const cli = cliJSON(['work', 'show', allowedRef!, '--json']) as View;
  expect(detail.requestRef).toBe(allowedRef);
  expect(cli).toEqual(detail);
  expect(detail.state?.decision.result).toBe('Allow');
  expect(detail.state?.claim?.id).toBeTruthy();
  expect(detail.state?.claim?.backendIdentity?.backend).toBe('agent-sandbox');
  expect(detail.state?.claim?.backendIdentity?.workerId).toMatch(/^agenova-pool-/);
  expect(detail.outcome?.status).toBe('Succeeded');
  expect(detail.outcome?.model?.model).toBe('llama3.1:latest');
  expect(detail.facts).toEqual(expect.arrayContaining([
    expect.objectContaining({ kind: 'ModelDecision', operation: 'model.invoke', result: 'Allow' }),
    expect.objectContaining({ kind: 'ProviderOutcome', operation: 'model.invoke' }),
    expect.objectContaining({ kind: 'Runtime', operation: 'CleanupSucceeded' }),
  ]));

  const listResponse = await request.get('/api/requests');
  expect(listResponse.status()).toBe(200);
  const list = await listResponse.json() as View[];
  expect(list.find(item => item.requestRef === allowedRef)?.state?.claim?.id).toBe(detail.state?.claim?.id);
  const cliList = cliJSON(['work', 'list', '--json']) as View[];
  expect(cliList.find(item => item.requestRef === allowedRef)).toEqual(detail);

  await page.goto(`/?mode=connected#/work/${encodeURIComponent(allowedRef!)}`);
  await page.locator('summary').filter({ hasText: 'Execution details' }).click();
  await expect(page.getByText(allowedRef!, { exact: true }).first()).toBeVisible();
  await expect(page.getByText(detail.state!.claim!.id, { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Succeeded', { exact: true }).first()).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath('installed-allowed-work.png'), fullPage: true });
});

test('installed API and Portal show denial without an invented claim', async ({ page, request }, info) => {
  test.skip(!deniedRef, 'Set AGENOVA_LIVE_DENIED_REF after a real denied CLI run.');
  const response = await request.get(`/api/requests/${encodeURIComponent(deniedRef!)}/evidence`);
  expect(response.status()).toBe(200);
  const detail = await response.json() as View;
  const cli = cliJSON(['work', 'show', deniedRef!, '--json']) as View;
  expect(cli).toEqual(detail);
  expect(detail.state?.decision.result).toBe('Deny');
  expect(detail.state?.claim).toBeFalsy();
  expect(detail.facts.some(fact => ['ModelDecision', 'ProviderAttempt', 'ProviderOutcome'].includes(fact.kind))).toBe(false);
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(deniedRef!)}`);
  await page.locator('summary').filter({ hasText: 'Execution details' }).click();
  await expect(page.getByText('Denied', { exact: true }).first()).toBeVisible();
  await expect(page.getByText(deniedRef!, { exact: true }).first()).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath('installed-denied-work.png'), fullPage: true });
});
