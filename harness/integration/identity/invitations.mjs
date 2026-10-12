// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Executable rendered UI contract with synthetic API responses; not live SSO proof.
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
import { chromium } from '../../../ui/node_modules/playwright/index.mjs';
const root = resolve(import.meta.dirname, '../../..');
const html = await readFile(resolve(root, 'internal/invitations/page.html'), 'utf8');
const css = await readFile(resolve(root, 'internal/invitations/page.css'), 'utf8');
const js = await readFile(resolve(root, 'internal/invitations/page.js'), 'utf8');
const pageHTML = html.replace('{{styles}}', css).replace('{{script}}', js);
const out = resolve(root, '.tmp/identity/invitation-ui');
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
 const page = await browser.newPage({ viewport: { width: 1100, height: 950 } });
 let invitations = [];
 let deny = false;
 let posts = 0;
 let loggedOut = false;
 await page.route('https://identity.example/**', async route => {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  if (path === '/members') return route.fulfill({ contentType: 'text/html', body: pageHTML });
  if (path === '/api/invitations') {
   if (request.method() === 'POST') {
    posts++;
    const payload = request.postDataJSON();
    assert.deepEqual(Object.keys(payload).sort(), ['email', 'project', 'role']);
    if (deny) return route.fulfill({ status: 403, body: 'Access denied' });
    assert.equal(payload.email, 'invited@example.com');
    assert.equal(payload.project, 'payments');
    assert.equal(payload.role, 'admin');
    invitations.push({ ...payload, id: 'fixture', status: 'sent' });
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(invitations[0]) });
   }
   return route.fulfill({ contentType: 'application/json', body: JSON.stringify(invitations) });
  }
  if (path === '/logout') { assert.equal(request.method(), 'POST'); loggedOut = true; return route.fulfill({ status: 200, body: 'Signed out' }); }
  throw new Error(`Unexpected browser request: ${path}`);
 });
 await page.goto('https://identity.example/members');
 await page.getByText('No invitations yet.').waitFor();
 await page.getByLabel('Email address').fill('invited@example.com');
 await page.getByLabel('Project', { exact: true }).fill('payments');
 await page.getByLabel('Role', { exact: true }).selectOption('admin');
 await page.getByRole('button', { name: 'Send invitation' }).click();
 await page.getByText('Invitation sent.', { exact: false }).waitFor();
 await page.getByText('invited@example.com', { exact: true }).waitFor();
 assert.equal(posts, 1);
 await page.screenshot({ path: resolve(out, 'desktop.png'), fullPage: true });
 deny = true;
 await page.getByLabel('Email address').fill('another@example.com');
 await page.getByRole('button', { name: 'Send invitation' }).click();
 await page.getByText('Your role cannot send this invitation.', { exact: false }).waitFor();
 assert.equal(posts, 2);
 invitations.push({ email: '<img src=x onerror=alert(1)>', project: 'test', role: 'viewer', status: 'uncertain' });
 await page.getByRole('button', { name: 'Refresh', exact: true }).click();
 await page.getByText('<img src=x onerror=alert(1)>', { exact: true }).waitFor();
 assert.equal(await page.locator('#invitations img').count(), 0);
 invitations.pop();
 await page.getByRole('button', { name: 'Refresh', exact: true }).click();
 await page.getByText('<img src=x onerror=alert(1)>', { exact: true }).waitFor({state:'hidden'});
 await page.setViewportSize({ width: 390, height: 844 });
 assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
 await page.screenshot({ path: resolve(out, 'mobile.png'), fullPage: true });
 await page.getByRole('button', { name: 'Log out', exact: true }).click();
 await page.waitForURL('**/logout');
 assert.equal(loggedOut, true);
 console.log('PASS invitation form payload, role denial, escaped rendering, mobile layout and POST logout. API responses are synthetic.');
} finally { await browser.close(); }
