// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('dark portal keeps state motion, input feedback and original navigation', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/#/work/fix-payment-timeout');
  await expect(page.locator('.portal-flow-track')).toHaveClass(/in-flight/);
  await expect(page.locator('.portal-flow-node.active')).toContainText('Model');
  expect(await page.locator('.portal-flow-track').evaluate(el => {
    const distance=parseFloat((el as HTMLElement).style.getPropertyValue('--packet-distance'));
    const width=el.firstElementChild!.getBoundingClientRect().width;
    return distance >= 0 && Math.abs(distance-(width-38)) < 1;
  })).toBe(true);
  expect(await page.locator('.portal-shell').evaluate(el => getComputedStyle(el).colorScheme)).toBe('dark');
  await expect(page.getByRole('button', { name: /Replay|Light|Motion on/ })).toHaveCount(0);
  await page.locator('.portal-flow').hover();
  await expect(page.locator('.portal-flow')).toHaveClass(/pointer-lit/);
  await page.waitForTimeout(450);
  await page.screenshot({ path: info.outputPath('react-running.png'), fullPage: true });
  await page.evaluate(() => window.scrollTo({ top: 220, behavior: 'instant' }));
  await expect(page.locator('.portal-topbar')).toHaveClass(/is-scrolled/);
  await page.getByText('Execution details', { exact: true }).click();
  await expect(page.locator('details[open]')).toHaveCount(1);
  await page.goto('/#/work/review-checkout-change');
  await expect(page.locator('.portal-flow-node.blocked')).toContainText('Access');
  await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
  expect(await page.locator('.portal-head .portal-badge.denied').evaluate(el => getComputedStyle(el, '::before').animationName)).toBe('portal-warning-beacon');
  await page.waitForTimeout(450);
  await page.screenshot({ path: info.outputPath('react-denied.png'), fullPage: true });
  await page.goto('/#/work/auth-failure-investigation');
  await expect(page.locator('.portal-flow-node.blocked')).toContainText('Worker');
  await expect(page.locator('.portal-flow-track')).not.toHaveClass(/in-flight/);
  await page.waitForTimeout(450);
  await page.screenshot({ path: info.outputPath('react-failed.png'), fullPage: true });
  await page.goto('/#/work/invoice-retry-tests');
  await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
  await expect(page.locator('.portal-flow-node').last()).toContainText('No record');
  await expect(page.locator('.portal-flow-node').nth(3)).toContainText('Request recorded');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.waitForTimeout(450);
  await page.screenshot({ path: info.outputPath('react-mobile.png'), fullPage: true });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  expect(await page.locator('.portal-head .portal-badge').evaluate(el => getComputedStyle(el, '::before').animationName)).toBe('none');
  await page.goto('/#/work/fix-payment-timeout');
  expect(await page.locator('.portal-flow-node.active .portal-flow-orb').evaluate(el => getComputedStyle(el, '::after').animationName)).toBe('none');
  await page.getByRole('link', { name: 'Work', exact: true }).first().click();
  const search = page.getByRole('searchbox');
  await search.fill('invoice');
  await expect(page.getByRole('link', { name: 'Update invoice retry tests' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Fix the payment timeout bug' })).toHaveCount(0);
  expect(errors).toEqual([]);
});
