// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';
import type { Setup, View } from '../src/connected-source';

const allowedRef = process.env.AGENOVA_LIVE_REQUEST_REF;
const deniedRef = process.env.AGENOVA_LIVE_DENIED_REF;

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
  expect(detail.requestRef).toBe(allowedRef);
  expect(detail.state?.decision.result).toBe('Allow');
  expect(detail.state?.claim?.id).toBeTruthy();
  expect(detail.outcome?.status).toBe('Succeeded');

  const listResponse = await request.get('/api/requests');
  expect(listResponse.status()).toBe(200);
  const list = await listResponse.json() as View[];
  expect(list.find(item => item.requestRef === allowedRef)?.state?.claim?.id).toBe(detail.state?.claim?.id);

  await page.goto(`/?mode=connected#/work/${encodeURIComponent(allowedRef!)}`);
  await expect(page.getByText(allowedRef!, { exact: true }).first()).toBeVisible();
  await expect(page.getByText(detail.state!.claim!.id, { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Succeeded', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('installed-allowed-work.png'), fullPage: true });
});

test('installed API and Portal show denial without an invented claim', async ({ page, request }, info) => {
  test.skip(!deniedRef, 'Set AGENOVA_LIVE_DENIED_REF after a real denied CLI run.');
  const response = await request.get(`/api/requests/${encodeURIComponent(deniedRef!)}/evidence`);
  expect(response.status()).toBe(200);
  const detail = await response.json() as View;
  expect(detail.state?.decision.result).toBe('Deny');
  expect(detail.state?.claim).toBeFalsy();
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(deniedRef!)}`);
  await expect(page.getByText('Denied', { exact: true }).first()).toBeVisible();
  await expect(page.getByText(deniedRef!, { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('installed-denied-work.png'), fullPage: true });
});
