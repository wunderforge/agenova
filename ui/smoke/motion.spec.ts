// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('lightweight dark portal keeps lifecycle static and preserves navigation', async ({page},info)=>{
  const errors:string[]=[]; page.on('pageerror',e=>errors.push(e.message));
  await page.goto('/#/work/fix-payment-timeout');
  await expect(page.locator('.portal-flow-node.active')).toContainText('Agent');
  await expect(page.locator('.portal-worker')).toContainText('No active call recorded');
  await expect(page.locator('.portal-flow-track')).not.toHaveClass(/in-flight/);
  expect(await page.locator('.portal-shell').evaluate(el=>getComputedStyle(el).colorScheme)).toBe('dark');
  expect(await page.locator('.portal-topbar').evaluate(el=>getComputedStyle(el).backdropFilter)).toBe('none');
  await page.locator('.portal-flow').hover();
  await expect(page.locator('.pointer-lit')).toHaveCount(0);
  expect(await page.evaluate(()=>document.getAnimations().filter(a=>a.effect?.getTiming().iterations===Infinity).length)).toBe(0);
  await page.screenshot({path:info.outputPath('react-running.png'),fullPage:true});
  await page.evaluate(()=>window.scrollTo({top:220,behavior:'instant'}));
  expect(await page.locator('.portal-scroll-progress').evaluate(el=>getComputedStyle(el).transform)).not.toBe('none');
  await page.getByText('Execution details',{exact:true}).click();
  await expect(page.locator('.portal-work-execution[open]')).toHaveCount(1);
  await page.goto('/#/work/review-checkout-change');
  await expect(page.locator('.portal-flow-node.blocked')).toContainText('Access');
  await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
  await page.screenshot({path:info.outputPath('react-denied.png'),fullPage:true});
  await page.goto('/#/work/auth-failure-investigation');
  await expect(page.locator('.portal-flow-node.blocked')).toContainText('Agent');
  await page.screenshot({path:info.outputPath('react-failed.png'),fullPage:true});
  await page.goto('/#/work/invoice-retry-tests');
  await expect(page.locator('.portal-flow-node.active')).toHaveCount(0);
  await expect(page.locator('.portal-flow-node').last()).toContainText('No record');
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath('react-mobile.png'),fullPage:true});
  await page.getByRole('link',{name:'Work',exact:true}).first().click();
  await page.getByRole('searchbox').fill('invoice');
  await expect(page.getByRole('link',{name:'Update invoice retry tests'})).toBeVisible();
  await expect(page.getByRole('link',{name:'Fix the payment timeout bug'})).toHaveCount(0);
  expect(errors).toEqual([]);
});
