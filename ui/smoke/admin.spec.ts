// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

// Render the real embedded page assets with local synthetic management APIs.
// This is a presentation/interaction test, not an admin-auth bypass or cloud test.
const read = (path: string) => readFileSync(new URL('../../' + path, import.meta.url), 'utf8');
const css = read('internal/invitations/page.css');
const js = read('internal/invitations/page.js');
const html = read('internal/invitations/page.html').replace('{{styles}}', css).replace('{{script}}', js).replace('{{registration}}', 'true');
const hash = (source: string) => createHash('sha256').update(source).digest('base64');
const csp = `default-src 'none'; connect-src 'self'; style-src 'sha256-${hash(css)}'; script-src 'sha256-${hash(js)}'; form-action 'self'; frame-ancestors 'none'`;
const owner = { principal: 'owner', email: 'owner@example.test', accountRole: 'owner', active: true, projects: [{ project: 'payments', role: 'admin' }], roles: [] };
const member = { principal: 'member', email: 'member@example.test', accountRole: 'member', active: true, projects: [{ project: 'payments', role: 'member' }], roles: [] };

function contrast(foreground: string, background: string) {
  const luminance = (color: string) => {
    const values = color.match(/[\d.]+/g)!.slice(0, 3).map(value => {
      const channel = Number(value) / 255;
      return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    });
    return values[0] * 0.2126 + values[1] * 0.7152 + values[2] * 0.0722;
  };
  const a = luminance(foreground), b = luminance(background);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

async function admin(page: Page, state: 'populated' | 'empty' | 'error' = 'populated') {
  const posts: { path: string; body: unknown }[] = [];
  await page.route('**/admin', route => route.fulfill({ contentType: 'text/html', body: html, headers: { 'Content-Security-Policy': csp } }));
  await page.route('**/auth/me', route => route.fulfill({ json: { subject: 'owner' } }));
  await page.route('**/api/**', route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === 'POST') {
      posts.push({ path, body: request.postDataJSON() });
      return route.fulfill({ json: { name: 'demo-policy' } });
    }
    if (state === 'error') return route.fulfill({ status: 503, json: { code: 'unavailable' } });
    if (path === '/api/members') return route.fulfill({ json: [owner, member] });
    if (path === '/api/access-requests') return route.fulfill({ json: state === 'empty' ? [] : [{ id: 'pending', email: 'pending@example.test' }] });
    if (path === '/api/setup') return route.fulfill({ json: { project: 'payments', policy: { ID: 'demo-policy', Version: '1', Rules: [] } } });
    return route.fulfill({ status: 404 });
  });
  await page.goto('/admin');
  return posts;
}

test('Administration and every management section use June in both themes and on mobile', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 1040 });
  await admin(page);
  await expect(page.getByRole('button', { name: 'Edit member@example.test' })).toBeVisible();
  for (const colorScheme of ['dark', 'light'] as const) {
    await page.emulateMedia({ colorScheme });
    await expect(page.locator('.admin-shell')).toHaveCSS('background-color', 'rgb(250, 249, 252)');
    await expect(page.locator('.admin-sidebar')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
    await expect(page.locator('.card').first()).toHaveCSS('background-color', 'rgb(255, 255, 255)');
  }
  await page.screenshot({ path: info.outputPath('june-administration-desktop.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  for (const name of ['Users', 'Access requests', 'Policy & Templates']) {
    await page.getByRole('navigation', { name: 'Administration', exact: true }).getByRole('link', { name, exact: true }).click();
    await expect(page.getByRole('heading', { name, exact: true })).toBeInViewport();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.screenshot({ path: info.outputPath('june-administration-mobile.png'), fullPage: true });
  expect(errors).toEqual([]);
});

test('management controls still load, validate and submit only their existing API shapes', async ({ page }) => {
  const posts = await admin(page);
  await page.getByRole('button',{name:'Edit member@example.test',exact:true}).click();
  await page.getByLabel('Role for member@example.test',{exact:true}).selectOption('platform-operator');
  await page.getByRole('button', { name: 'Save role', exact: true }).click();
  await expect(page.locator('#member-notice')).toContainText('User access updated.');
  expect(posts[0]).toEqual({ path: '/api/members', body: { principal: 'member', action: 'platform-role', role: 'platform-operator', project: 'payments' } });
  await page.getByRole('link',{name:'Access requests',exact:true}).click();
  await page.getByLabel('Project for pending@example.test').fill('payments');
  await page.getByRole('button', { name: 'Approve access', exact: true }).click();
  await expect.poll(() => posts.length).toBe(2);
  expect(posts[1]).toEqual({ path: '/api/access-requests', body: { id: 'pending', action: 'approve', platformRole: 'member', project: 'payments' } });
  await page.getByRole('link',{name:'Policy & Templates',exact:true}).click();
  await page.locator('#governance-load').click();
  await expect(page.locator('#governance-document')).toBeEnabled();
  await page.locator('#governance-document').fill('not json');
  await page.locator('#governance-save').click();
  await expect(page.locator('#governance-notice')).toContainText('Invalid JSON');
  expect(posts).toHaveLength(2);
  await expect(page.locator('.admin-sidebar form')).toHaveAttribute('action', '/logout');
  await expect(page.locator('.admin-sidebar form')).toHaveAttribute('method', 'post');
});

test('empty and failed management states keep safe controls and readable June cards', async ({ page }, info) => {
  await admin(page, 'empty');
  await expect(page.locator('#access-notice')).toHaveText('No pending access requests.');
  await expect(page.locator('#governance-save')).toBeDisabled();
  await page.screenshot({ path: info.outputPath('june-administration-empty.png'), fullPage: true });
  await page.unrouteAll({ behavior: 'wait' });
  await admin(page, 'error');
  await expect(page.locator('#members-loading')).toHaveText('Administrator access is required.');
  await expect(page.locator('#members .member')).toHaveCount(0);
  await expect(page.locator('#governance-save')).toBeDisabled();
  await page.screenshot({ path: info.outputPath('june-administration-unavailable.png'), fullPage: true });
});

test('legacy fixture and evidence pages now share the fixed June palette', async ({ page }, info) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.setViewportSize({ width: 390, height: 844 });
  for (const path of ['/fixtures', '/console/requests/fix-payment-timeout', '/console/unknown/reference', '/console/requests/not-recorded']) {
    await page.goto(path);
    await expect(page.locator('html')).toHaveCSS('background-color', 'rgb(250, 249, 252)');
    await expect(page.locator('html')).toHaveCSS('color-scheme', 'light only');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.screenshot({ path: info.outputPath('june-evidence-mobile.png'), fullPage: true });
});

test('all Portal route families retain June styling including missing records', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.setViewportSize({ width: 390, height: 844 });
  for (const route of ['work', 'work/new', 'work/invoice-retry-tests', 'work/invoice-retry-tests/access', 'work/invoice-retry-tests/activity', 'agents', 'agents/engineer', 'policy', 'platform', 'platform/activity', 'identity', 'work/not-recorded']) {
    await page.goto('/#/' + route);
    await expect(page.locator('.portal-shell')).toHaveCSS('background-color', 'rgb(250, 249, 252)');
    await expect(page.locator('.portal-sidebar')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
});

test('administration placeholders and denied evidence labels meet normal-text contrast', async ({ page }) => {
  await admin(page);
  const field = await page.locator('#governance-project').evaluate(element => ({
    foreground: getComputedStyle(element, '::placeholder').color,
    background: getComputedStyle(element).backgroundColor,
  }));
  expect(contrast(field.foreground, field.background)).toBeGreaterThanOrEqual(4.5);
  await page.unrouteAll({ behavior: 'wait' });
  await page.goto('/console/requests/fix-payment-timeout-team-b');
  await expect(page.locator('.decision-deny')).toBeVisible();
  const badge = await page.locator('.decision-deny').evaluate(element => ({
    foreground: getComputedStyle(element).color,
    background: getComputedStyle(element).backgroundColor,
  }));
  expect(contrast(badge.foreground, badge.background)).toBeGreaterThanOrEqual(4.5);
});
