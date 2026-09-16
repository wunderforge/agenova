// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Read-only visual check of a retained run; no submission or model invocation.
import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
const url = process.argv[2];
assert.ok(url?.startsWith('http://127.0.0.1:'), 'Provide an existing local work URL');
const output = new URL('../../.tmp/work-reading-path/', import.meta.url);
await mkdir(output, { recursive: true });
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1360, height: 1000 } });
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  // The inspection is explicitly non-mutating, even if the UI regresses.
  await page.route('**/api/**', route => ['GET', 'HEAD'].includes(route.request().method()) ? route.continue() : route.abort());
  await page.goto(url);
  await page.getByRole('heading', { name: 'Result', exact: true }).waitFor();
  await page.screenshot({ path: fileURLToPath(new URL('desktop.png', output)), fullPage: true });
  assert.equal(await page.locator('.portal-turn[open]').count(), 1);
  assert.equal(await page.locator('.portal-work-inspect details[open]').count(), 0);
  assert.equal(await page.getByRole('heading', { name: 'Progress', exact: true }).count(), 0);
  assert.ok(await page.locator('.portal-result').evaluate(el => !!(el.compareDocumentPosition(document.querySelector('.portal-worker')) & Node.DOCUMENT_POSITION_FOLLOWING)));
  const report = await page.evaluate(() => ({
    turns: document.querySelectorAll('.portal-turn').length,
    openTurn: document.querySelector('.portal-turn[open] > summary')?.textContent,
    infiniteAnimations: document.getAnimations().filter(a => a.effect?.getTiming().iterations === Infinity).length,
    width: document.documentElement.scrollWidth, viewport: innerWidth,
  }));
  assert.equal(report.infiniteAnimations, 0, 'Completed work has no ongoing call motion');
  assert.ok(report.width <= report.viewport);
  await page.locator('.portal-turn > summary').first().click();
  await page.screenshot({ path: fileURLToPath(new URL('history.png', output)), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: fileURLToPath(new URL('mobile.png', output)), fullPage: true });
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  assert.deepEqual(errors, []);
  await writeFile(new URL('report.json', output), JSON.stringify({ ...report, errors }, null, 2));
  console.log(JSON.stringify(report));
} finally { await browser.close(); }
