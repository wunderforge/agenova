// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('mock identity switch stays in Demo and never becomes Connected evidence', async ({ page }, info) => {
  await page.goto('/#/scenario/payment-incident');
  await expect(page.getByRole('heading', { name: '一次事故，两种临时授权' })).toBeVisible();
  await expect(page.getByText('演示模拟')).toBeVisible();
  await expect(page.getByText('demo:payments-developer · Payments Development')).toBeVisible();
  await expect(page.getByText('准备修复 PR', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: /值班 SRE/ }).click();
  await expect(page.getByText('demo:on-call-sre · Reliability')).toBeVisible();
  await expect(page.getByText('执行受控回滚', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('incident-sre-mock.png'), fullPage: true });
  await page.getByRole('button', { name: 'Connected' }).click();
  await expect(page.getByText('Server data')).toBeVisible();
  await expect(page.getByText('执行受控回滚', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /值班 SRE/ })).toHaveCount(0);
  await expect(page.getByRole('link', { name: /Incident scenario/ })).toHaveCount(0);
});
