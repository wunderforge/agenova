// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';
test.beforeEach(async ({page},info) => {
  if (info.title.startsWith('connected ')) await page.route('**/api/**',route => route.fulfill({status:503,json:{code:'unavailable',message:'Service unavailable'}}));
});

test('portal journey keeps work evidence scoped and shows narrowed authority', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Work', exact: true })).toBeVisible();
  await expect(page.getByText('Example data', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('portal-work-list.png'), fullPage: true });
  await page.getByRole('link', { name: 'Update invoice retry tests' }).click();
  await expect(page.getByRole('heading', { name: 'Work status' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Progress' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Recent activity' })).toHaveCount(0);
  await page.getByText('Access & limits', { exact: true }).click();
  await expect(page.getByText('cpu-intensive · 30 min', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Compare requested and granted' }).click();
  await expect(page.locator('.portal-compare-side').first().getByText('shell.exec', { exact: true })).toBeVisible();
  await expect(page.getByText('cpu-intensive · 45 min')).toBeVisible();
  await expect(page.getByText('cpu-intensive · 30 min')).toBeVisible();
  await page.getByRole('link', { name: 'Update invoice retry tests' }).first().click();
  await page.getByRole('link', { name: 'Full activity record' }).click();
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
  await page.getByText('Access & limits', { exact: true }).click();
  await expect(page.getByText('No authority has been issued.')).toBeVisible();
  await page.getByText('Execution details').click();
  for (const label of ['Claim ID', 'Runtime backend', 'Worker ID']) {
    await expect(page.locator('.portal-details .portal-fact').filter({ hasText: label })).toContainText('Not available');
  }
  await page.getByRole('link', { name: 'View full request' }).click();
  await expect(page.getByText('No authority was issued.')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Granted' })).toBeVisible();
});

test('named demo work keeps full instructions behind a disclosure',async({page})=>{
 await page.goto('/#/work/new');
 await page.getByLabel('Work name (optional)').fill('Checkout tests');
 const objective='Improve checkout tests. Cover the edge cases and explain the changes.';
 await page.getByLabel('What should it do?').fill(objective);
 await page.getByLabel('Repository').fill('acme/checkout');
 await page.getByRole('button',{name:'Create example request'}).click();
 await expect(page.getByRole('heading',{name:'Checkout tests',exact:true})).toBeVisible();
 await page.getByText('Task instructions',{exact:true}).click();
 await expect(page.locator('.portal-task-instructions pre')).toHaveText(objective);
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
  await page.getByText('Access & limits', { exact: true }).click();
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
  await expect(page.getByRole('heading', { name: 'Work status' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('portal-mobile-work.png'), fullPage: true });
});

test('connected view keeps the route but never shows illustrative work or a false empty result', async ({ page }, info) => {
  await page.goto('/#/work/invoice-retry-tests');
  await expect(page.getByRole('heading', { name: 'Update invoice retry tests' })).toBeVisible();
  await page.getByRole('button', { name: 'Connected' }).click();
  await expect(page).toHaveURL(/\?mode=connected#\/work\/invoice-retry-tests$/);
  await expect(page.getByRole('heading', { name: 'Work details', exact: true })).toBeVisible();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByText('Update invoice retry tests')).toHaveCount(0);
  await expect(page.getByText('No work found')).toHaveCount(0);
  await expect(page.getByText('Team A engineer')).toHaveCount(0);
  await page.reload();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await page.getByRole('button', { name: 'Demo', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Update invoice retry tests' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('portal-demo-restored.png'), fullPage: true });
});

test('connected pages explain unavailable capabilities without leaking demo records', async ({ page }, info) => {
  await page.goto('/?mode=connected#/work');
  await expect(page.getByRole('heading', { name: 'Work', exact: true })).toBeVisible();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByText('Fix the payment timeout bug')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Connected' })).toHaveAttribute('aria-pressed', 'true');
  await page.screenshot({ path: info.outputPath('portal-connected-work.png'), fullPage: true });
  await page.getByRole('link', { name: 'Platform' }).click();
  await expect(page.getByRole('heading', { name: 'Platform' })).toBeVisible();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByText('Tool Gateway')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('portal-connected-platform.png'), fullPage: true });
  await page.getByRole('link', { name: 'Agents' }).click();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByText('Engineer', { exact: true })).toHaveCount(0);
  await page.getByRole('link', { name: 'Policy' }).click();
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await page.goto('/?mode=connected#/platform/activity');
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByText('Other repository blocked')).toHaveCount(0);
  await page.goto('/?mode=connected#/work/new');
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Create example request' })).toHaveCount(0);
});

test('connected status stays readable on mobile', async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/?mode=connected#/platform');
  await expect(page.getByText('The connection is unavailable. Try again.')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('portal-connected-mobile.png'), fullPage: true });
});
