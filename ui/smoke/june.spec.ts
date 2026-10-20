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
