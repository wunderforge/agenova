// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';
import { consoleHref } from '../src/console-route';

const states = [
  { name: 'allowed', reference: 'fix-payment-timeout', scenario: 'canonical', heading: 'Request & principal' },
  { name: 'narrowed', reference: 'fix-payment-timeout', scenario: 'narrowed', heading: 'Requested vs effective authority' },
  { name: 'denied', reference: 'fix-payment-timeout-team-b', scenario: 'canonical', heading: 'Request & principal' },
  { name: 'loading', reference: 'fix-payment-timeout', scenario: 'loading', heading: 'Loading evidence' },
  { name: 'not-found', reference: 'fix-payment-timeout', scenario: 'not-found', heading: 'Reference not found' },
  { name: 'malformed', reference: 'fix-payment-timeout', scenario: 'malformed', heading: 'Malformed evidence' },
  { name: 'unavailable', reference: 'fix-payment-timeout', scenario: 'unavailable', heading: 'Evidence unavailable' },
] as const;

for (const viewport of [{ width: 1100, height: 1000 }, { width: 390, height: 844 }]) {
  for (const state of states) {
    test(`console ${viewport.width} ${state.name}`, async ({ page }, info) => {
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.setViewportSize(viewport);
      await page.goto(consoleHref('requests', state.reference, state.scenario));
      await expect(page.getByRole('heading', { name: 'Claim Console', exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: state.heading, exact: true })).toBeVisible();
      if (state.name === 'allowed' || state.name === 'narrowed') {
        await expect(page.getByText('Allow', { exact: true })).toBeVisible();
        await expect(page.getByText('Running', { exact: true })).toBeVisible();
        await expect(page.getByText('reference', { exact: true })).toBeVisible();
        await expect(page.getByText('Agent outcome not supplied.')).toBeVisible();
        await expect(page.getByText('Revocation evidence not supplied.')).toBeVisible();
        if (state.name === 'narrowed') {
          await expect(page.getByText(/Derived fixture demonstration/)).toBeVisible();
          await expect(page.getByRole('row', { name: /Tools/ })).toContainText('Requested, not granted: github.pull-request');
        } else await expect(page.getByText('Same recorded access')).toHaveCount(4);
      }
      if (state.name === 'denied') {
        await expect(page.getByText('Deny', { exact: true })).toBeVisible();
        await expect(page.getByText('Matching request document not supplied.')).toBeVisible();
        await expect(page.getByText('No claim issued.')).toBeVisible();
        await expect(page.getByText('No backend allocation in this snapshot.')).toBeVisible();
        await expect(page.getByText('Running', { exact: true })).toHaveCount(0);
        await expect(page.getByText('user:team-a-engineer')).toHaveCount(0);
      }
      if (state.name === 'malformed') await expect(page.getByRole('alert')).toContainText('missing-source-field: evidence');
      if (['loading', 'not-found', 'malformed', 'unavailable'].includes(state.name)) {
        await expect(page.getByText('Allow', { exact: true })).toHaveCount(0);
        await expect(page.getByRole('heading', { name: 'Requested vs effective authority' })).toHaveCount(0);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.evaluate(() => document.fonts.ready);
      await page.screenshot({ path: info.outputPath(`console-${viewport.width}-${state.name}.png`), fullPage: true, animations: 'disabled', caret: 'hide' });
      expect(errors).toEqual([]);
    });
  }
}

test('console direct claim route, reload, navigation and browser history', async ({ page }) => {
  await page.goto(consoleHref('claims', 'claim:fix-payment-timeout:1'));
  await expect(page.getByText('Allow', { exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByText('Allow', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Denied request', exact: true }).click();
  await expect(page.getByText('Deny', { exact: true })).toBeVisible();
  await page.goBack();
  await expect(page.getByText('Allow', { exact: true })).toBeVisible();
  await page.goForward();
  await expect(page.getByText('Deny', { exact: true })).toBeVisible();
  await page.goto(consoleHref('claims', 'fix-payment-timeout'));
  await expect(page.getByRole('heading', { name: 'Reference not found' })).toBeVisible();
  await page.goto('/console/unknown/reference');
  await expect(page.getByRole('heading', { name: 'Malformed console route' })).toBeVisible();
});

test('console keyboard, focus, semantic structure and source announcements', async ({ page }) => {
  await page.goto(consoleHref('requests', 'fix-payment-timeout'));
  await expect(page.getByText('Allow', { exact: true })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByRole('navigation', { name: 'Fixture stories' })).toBeVisible();
  await expect(page.getByRole('table', { name: 'Recorded access comparison' })).toBeVisible();
  await page.keyboard.press('Tab');
  const skip = page.getByRole('link', { name: 'Skip to claim evidence' });
  await expect(skip).toBeFocused();
  await expect(skip).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('main')).toBeFocused();
  await page.keyboard.press('Tab');
  const allowed = page.getByRole('link', { name: 'Allowed request', exact: true });
  await expect(allowed).toBeFocused();
  expect(await allowed.evaluate(element => getComputedStyle(element).outlineWidth)).not.toBe('0px');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Enter');
  await expect(page.getByText('Requested, not granted: github.pull-request')).toBeVisible();
  const select = page.getByLabel('Fixture presentation scenario');
  await select.focus();
  await page.keyboard.press('Home');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: 'Loading evidence' })).toBeVisible();
  await expect(page.locator('[aria-live="polite"][aria-busy="true"]')).toHaveCount(1);
  await expect(page.getByText('Allow', { exact: true })).toHaveCount(0);
});
