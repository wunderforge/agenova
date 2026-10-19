// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import type { View } from '../src/connected-source';

const ownRef = process.env.E14_OWN_REF;
const otherRef = process.env.E14_OTHER_REF;
const subject = process.env.E14_SUBJECT;
const timeout = process.env.E14_EFFECTIVE_TIMEOUT;
const cliPath = process.env.AGENOVA_CLI_PATH;

test('installed E14 owner sees identical API, CLI and Portal evidence', async ({ page, request }, info) => {
  test.skip(!ownRef || !otherRef || !subject || !timeout || !cliPath, 'Set E14_OWN_REF, E14_OTHER_REF, E14_SUBJECT, E14_EFFECTIVE_TIMEOUT and AGENOVA_CLI_PATH.');

  const response = await request.get(`/api/requests/${encodeURIComponent(ownRef!)}/evidence`);
  expect(response.status()).toBe(200);
  const api = await response.json() as View;
  const cli = JSON.parse(execFileSync(cliPath!, ['work', 'show', ownRef!, '--json'], { encoding: 'utf8', timeout: 60_000 })) as View;
  expect(cli).toEqual(api);
  expect(api.state?.principal.subject).toBe(subject);
  expect(api.state?.policyRef.version).toBe('1');
  expect(api.state?.effectiveAuthority?.runtime.timeout).toBe(timeout);
  expect(api.facts.some(fact => fact.kind === 'AuthorityResolved')).toBe(true);

  const hidden = await request.get(`/api/requests/${encodeURIComponent(otherRef!)}/evidence`);
  expect(hidden.status()).toBe(404);
  const list = await request.get('/api/requests');
  expect(list.status()).toBe(200);
  const ownList = await list.json() as View[];
  expect(ownList.some(work => work.requestRef === ownRef)).toBe(true);
  expect(ownList.some(work => work.requestRef === otherRef)).toBe(false);

  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ownRef!)}`);
  await expect(page.getByText(subject!, { exact: true }).first()).toBeVisible();
  await page.locator('summary').filter({ hasText: 'Execution details' }).click();
  await expect(page.getByRole('main').getByText(ownRef!, { exact: true }).first()).toBeVisible();
  await expect(page.getByText('e14-team-policy / 1', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath(`e14-${subject}.png`), fullPage: true });
});
