// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('portal journey keeps work evidence scoped and shows narrowed authority', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Work', exact: true })).toBeVisible();
  await expect(page.getByText('Example data · no live connection')).toBeVisible();
  await page.screenshot({ path: info.outputPath('portal-work-list.png'), fullPage: true });
  await page.getByRole('link', { name: 'Update invoice retry tests' }).click();
  await expect(page.getByRole('heading', { name: 'Progress' })).toBeVisible();
  await expect(page.getByText('cpu-intensive · 30 min', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Compare requested and granted' }).click();
  await expect(page.locator('.portal-compare-side').first().getByText('shell.exec', { exact: true })).toBeVisible();
  await expect(page.getByText('cpu-intensive · 45 min')).toBeVisible();
  await expect(page.getByText('cpu-intensive · 30 min')).toBeVisible();
  await page.getByRole('link', { name: 'Update invoice retry tests' }).first().click();
  await page.getByRole('link', { name: 'View all activity' }).click();
  await expect(page.getByRole('link', { name: 'Other repository blocked' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Request resolution' }).click();
  await page.getByRole('link', { name: 'Access narrowed' }).click();
  await expect(page.getByRole('heading', { name: 'Effective authority at resolution' })).toBeVisible();
  await expect(page.getByText('repo:acme/billing', { exact: true })).toBeVisible();
  await expect(page.getByText('cpu-intensive · 30 min', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('portal-resolution.png'), fullPage: true });
  expect(errors).toEqual([]);
});

test('new example request stays pending with no invented grant or worker', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('link', { name: 'New work' }).click();
  await page.getByLabel('What should it do?').fill('Improve checkout tests');
  await page.getByLabel('Repository').fill('acme/checkout');
  await page.getByRole('button', { name: 'Create example request' }).click();
  await expect(page.getByRole('heading', { name: 'Improve checkout tests' })).toBeVisible();
  await expect(page.getByText('No authority has been issued.')).toBeVisible();
  await page.getByText('Execution details').click();
  for (const label of ['Claim ID', 'Runtime backend', 'Worker ID']) {
    await expect(page.locator('.portal-details .portal-fact').filter({ hasText: label })).toContainText('Not available');
  }
  await page.getByRole('link', { name: 'View full request' }).click();
  await expect(page.getByText('No authority was issued.')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Granted' })).toBeVisible();
});

test('research work uses a topic, not a repository field', async ({ page }) => {
  await page.goto('/#/work/new?agent=researcher');
  await expect(page.getByLabel('Topic')).toBeVisible();
  await expect(page.getByLabel('Repository')).toHaveCount(0);
  await page.getByLabel('What should it do?').fill('Investigate login failures');
  await page.getByLabel('Topic').fill('Customer authentication');
  await page.getByRole('button', { name: 'Create example request' }).click();
  await expect(page.getByRole('heading', { name: 'Investigate login failures' })).toBeVisible();
  await expect(page.locator('.portal-top-facts').getByText('Customer authentication')).toBeVisible();
  await expect(page.getByText('No authority has been issued.')).toBeVisible();
});

test('platform activity aggregates example records and links back to one work', async ({ page }, info) => {
  await page.goto('/#/platform');
  await expect(page.getByRole('heading', { name: 'What is connected today?' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Live connection' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('portal-platform.png'), fullPage: true });
  await page.getByRole('link', { name: 'View tool activity' }).click();
  await expect(page.getByRole('heading', { name: 'Governance activity' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Other repository blocked' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Model request' })).toHaveCount(0);
  await page.getByRole('link', { name: 'Other repository blocked' }).click();
  await expect(page.locator('.portal-panel > p').first()).toContainText('No external call was made.');
  await expect(page.getByRole('heading', { name: 'Fix the payment timeout bug' })).toHaveCount(0);
});

test('portal remains usable at mobile width', async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Work', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Update invoice retry tests' }).click();
  await expect(page.getByRole('heading', { name: 'Progress' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('portal-mobile-work.png'), fullPage: true });
});
