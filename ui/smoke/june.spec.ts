// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('June layout stays light in either system theme and fits mobile', async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  for (const colorScheme of ['dark', 'light'] as const) {
    await page.emulateMedia({ colorScheme });
    await page.goto('/#/work');
    await expect(page.locator('.portal-shell')).toHaveCSS('background-color', 'rgb(250, 249, 252)');
    await expect(page.locator('.portal-sidebar')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
    await expect(page.getByLabel('Loaded work summary')).toBeVisible();
  }
  await page.screenshot({ path: info.outputPath('june-explorer-desktop.png'), fullPage: true });
  await page.goto('/#/work/invoice-retry-tests');
  await page.getByRole('button', { name: /^Inspect / }).first().click();
  const inspector = page.getByRole('complementary', { name: 'Selected activity' });
  await expect(inspector).toBeVisible();
  await expect(inspector).toContainText('Access decisions are not proof of execution');
  await expect(inspector.getByRole('link', { name: 'Open evidence record' })).toHaveAttribute('href', /\/activity\//);
  await page.screenshot({ path: info.outputPath('june-detail-desktop.png'), fullPage: true });
  await page.getByRole('button', { name: /^Inspect / }).last().focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: /^Inspect / }).last()).toHaveAttribute('aria-pressed', 'true');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('june-detail-mobile.png'), fullPage: true });
  await page.getByRole('link', { name: 'Policy', exact: true }).first().click();
  await page.screenshot({ path: info.outputPath('june-policy-mobile.png'), fullPage: true });
});

test('approved chart, profile, guide and settings are real accessible controls', async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 1040 });
  await page.goto('/#/work');
  await expect(page.getByRole('region', { name: 'Work activity', exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: /loaded requests on/ })).toBeVisible();
  await page.getByLabel('Activity date').selectOption('2026-09-13');
  await expect(page.getByRole('img')).toHaveAttribute('aria-label', /2 loaded requests on 2026-09-13/);
  await page.getByLabel('Open profile menu', { exact: true }).click();
  const account = page.locator('.portal-topbar .portal-profile-popover');
  await expect(account).toContainText('Team A engineer');
  await expect(account.getByRole('link', { name: 'View identity' })).toHaveAttribute('href', '#/identity');
  await page.keyboard.press('Escape');
  await expect(account).not.toBeVisible();
  await page.getByRole('button', { name: 'Quick guide', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Describe the work.');
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('button', { name: 'View settings', exact: true }).click();
  await page.getByRole('radio', { name: 'Compact', exact: true }).check();
  await page.getByRole('button', { name: 'Close dialog' }).click();
  await expect(page.locator('.portal-shell')).toHaveAttribute('data-density', 'compact');
  await page.keyboard.press('/');
  await expect(page.getByRole('searchbox', { name: 'Search work' })).toBeFocused();
  await page.getByRole('searchbox').fill('literal?query');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('searchbox').fill('');
  await page.getByRole('heading', { level: 1 }).click();
  await page.screenshot({ path: info.outputPath('june-complete-explorer-desktop.png'), fullPage: true });
  await page.getByLabel('Open workspace account menu').click();
  await expect(page.locator('.sidebar-profile .portal-profile-popover')).toBeVisible();
  await page.keyboard.press('Escape');
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByLabel('Open profile menu', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Help & shortcuts', exact: true }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.keyboard.press('Escape');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('june-complete-explorer-mobile.png'), fullPage: true });
});
