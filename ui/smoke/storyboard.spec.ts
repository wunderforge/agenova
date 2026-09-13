// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

const cases = [
  ['request-only', 'claim-request.valid.team-a-engineer-json', 'Request intent only'],
  ['allow-running', 'issued-state.valid.team-a-engineer', 'Decision: Allow'],
  ['deny-no-claim', 'issued-state.valid.team-b-denial', 'Decision: Deny'],
  ['invalid-caller-state', 'issued-state.invalid.caller-effective-authority', 'Invalid or incomplete source'],
  ['missing-field', 'derived.missing-field', 'Invalid or incomplete source'],
  ['unknown-field', 'derived.unknown-field', 'Invalid or incomplete source'],
] as const;
for (const [name, id, heading] of cases) {
  test(`${name}: ${id}`, async ({ page }, info) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto('/');
    await page.getByLabel('Fixture or derived display case').selectOption(id);
    await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible();
    if (name === 'allow-running') {
      await expect(page.getByText('Running', { exact: true })).toBeVisible();
      await expect(page.getByText('ClaimRunning', { exact: true })).toBeVisible();
    }
    if (name === 'deny-no-claim') {
      await expect(page.getByText('No claim issued.')).toBeVisible();
      await expect(page.getByText('No authority issued.')).toBeVisible();
      await expect(page.getByText('Running', { exact: true })).toHaveCount(0);
    }
    if (name === 'missing-field' || name === 'unknown-field') {
      await expect(page.getByRole('alert')).toContainText(name === 'missing-field' ? 'missing-source-field: evidence' : 'unknown-field: futureField');
      await expect(page.getByText(/Not a canonical fixture/)).toBeVisible();
    }
    await page.screenshot({ path: info.outputPath(`${name}.png`), fullPage: true });
    expect(errors).toEqual([]);
  });
}
test('mobile selection stays usable without horizontal overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await page.getByLabel('Fixture or derived display case').selectOption('issued-state.valid.team-a-engineer');
  await expect(page.getByRole('heading', { name: 'Decision: Allow' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
