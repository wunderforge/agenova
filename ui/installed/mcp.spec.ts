// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test, type APIRequestContext, type Page, type TestInfo } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { writeFile } from 'node:fs/promises';
import type { Fact } from '../src/contracts.generated';
import type { Setup, View } from '../src/connected-source';

// E16 Slice 3 parity for one installed Work at a time, run right after that
// Work so its in-memory evidence still exists. The campaign runner selects
// the setup test plus the Work's own test:
//   npx playwright test --config playwright.installed.config.ts installed/mcp.spec.ts --grep '@setup|@positive'
// The reference spec in this directory asserts the reference Policy and
// template, so it does not apply to the E16 install. Answer facts and the
// server log are decided by harness/integration/e16/evidence, not here.
// The Slice 4 token cases never see a token value: the runner holds them and
// scans the HTML these tests save next to their screenshots.
const cliPath = process.env.AGENOVA_CLI_PATH;
const stateDir = process.env.AGENOVA_CLI_STATE_DIR;
const workCase = process.env.AGENOVA_E16_CASE;
const ref = process.env.AGENOVA_E16_REF;
const cases = ['positive', 'n6-timeout', 'n7-oversize', 'n8-truncation', 'admission-deny', 'token-valid', 'token-missing', 'token-wrong'];
if (!cliPath || !stateDir || !workCase || !ref || !cases.includes(workCase)) {
  throw new Error(`E16 installed parity requires AGENOVA_CLI_PATH, AGENOVA_CLI_STATE_DIR, AGENOVA_E16_REF and AGENOVA_E16_CASE (${cases.join(', ')}).`);
}

function cliJSON(args: string[]): unknown {
  return JSON.parse(execFileSync(cliPath!, ['--state-dir', stateDir!, ...args], { encoding: 'utf8', timeout: 60_000, windowsHide: true }));
}

async function parity(request: APIRequestContext): Promise<View> {
  const response = await request.get(`/api/requests/${encodeURIComponent(ref!)}/evidence`);
  expect(response.status()).toBe(200);
  const detail = await response.json() as View;
  expect(detail.requestRef).toBe(ref);
  expect(cliJSON(['work', 'show', ref!, '--json'])).toEqual(detail);
  return detail;
}

const toolFacts = (view: View) => view.facts.filter(fact => fact.operation === 'tool.invoke');

// The positive Work's scope and route files (harness/integration/e16/work-positive.yaml,
// platform.yaml); a successful read's resultRef is the scope, "/" and the file.
const positiveScope = 'repo:agenova/e16-fixture';
const positiveRoute = ['README.md', 'logs/timeout.log', 'src/retry.txt'];
const diagnosticFiles = ['logs/timeout.log', 'src/retry.txt'];

function invocations(view: View): Map<string, Fact[]> {
  const out = new Map<string, Fact[]>();
  for (const fact of toolFacts(view)) {
    if (!fact.invocationId) continue;
    out.set(fact.invocationId, [...(out.get(fact.invocationId) ?? []), fact]);
  }
  return out;
}

async function openRecord(page: Page, fact: Fact) {
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ref!)}/activity/${encodeURIComponent(fact.id)}`);
  await expect(page.getByText(fact.invocationId!, { exact: true }).first()).toBeVisible();
}

// A configured call's outcome record. The heading must match exactly, so
// "Mock tool call finished" cannot satisfy it.
async function expectConfiguredRecord(page: Page) {
  await expect(page.getByRole('heading', { level: 1, name: 'Tool call finished', exact: true })).toBeVisible();
  await expect(page.getByText('Mock tool call', { exact: false })).toHaveCount(0);
}

// The Work page lists its one configured call under the plain label, with the
// call's own state, and shows "(mock)" nowhere.
async function expectConfiguredWork(page: Page, state: string) {
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ref!)}`);
  const call = page.locator('.portal-worker-actions li').filter({ has: page.getByText('Tool call', { exact: true }) });
  await expect(call).toHaveCount(1);
  // Only the latest turn starts open; open the call's turn for the screenshot.
  const turn = page.locator('.portal-turn').filter({ has: call });
  if (await turn.getAttribute('open') === null) await turn.locator(':scope > summary').click();
  await expect(call).toBeVisible();
  await expect(call.locator('.portal-worker-state')).toHaveText(state);
  await expect(page.getByText('(mock)', { exact: false })).toHaveCount(0);
}

// PNGs cannot be searched, so the rendered HTML goes next to each screenshot
// for the runner's token scan.
async function capture(page: Page, info: TestInfo, name: string) {
  await writeFile(info.outputPath(`${name}.html`), await page.content());
  await page.screenshot({ path: info.outputPath(`${name}.png`), fullPage: true });
}

test('installed E16 setup reports the configured MCP tool and E16 registrations @setup', async ({ request }) => {
  const response = await request.get('/api/setup');
  expect(response.status()).toBe(200);
  const setup = await response.json() as Setup;
  expect(setup.installation.kind).toBe('installed');
  expect(setup.installation.revision).toMatch(/^sha256:[a-f0-9]{64}$/);
  expect(setup.capabilities.tool).toBe('configured');
  expect(setup.policy.ID).toBe('e16-mcp-acceptance');
  expect(setup.template.metadata.name).toBe('e16-engineer');
  expect(setup.template.spec.capabilityCeiling?.tools).toEqual(['repo.read']);
});

test('positive Work: real MCP reads, two model turns, matching CLI, API and Portal @positive', async ({ page, request }, info) => {
  const view = await parity(request);
  expect(view.outcome?.status).toBe('Succeeded');
  expect(view.outcome?.model?.model).toBe('qwen2.5:7b');
  expect(view.facts.filter(f => f.kind === 'ProviderOutcome' && f.operation === 'model.invoke' && f.providerStatus === 'Succeeded').length).toBeGreaterThanOrEqual(2);
  expect(view.facts).toEqual(expect.arrayContaining([expect.objectContaining({ kind: 'Runtime', operation: 'CleanupSucceeded' })]));
  expect(toolFacts(view).some(f => f.reasonCode?.startsWith('mock-'))).toBe(false);
  const calls = invocations(view);
  expect(calls.size).toBeGreaterThan(0);
  const outcomes: Fact[] = [];
  for (const [, facts] of calls) {
    expect(facts.map(f => `${f.kind}:${f.result || f.providerStatus}`)).toEqual(['ToolDecision:Allow', 'ProviderAttempt:Attempted', 'ProviderOutcome:Succeeded']);
    const [, attempt, outcome] = facts;
    expect(attempt.target).toBe('repo.read');
    expect(outcome.target).toBe(attempt.target);
    expect(positiveRoute.map(file => `${positiveScope}/${file}`)).toContain(outcome.resultRef);
    outcomes.push(outcome);
  }
  // Both diagnostic files must have been read, each shown with its own
  // result; the evidence checker joins each result to the server's file.
  for (const file of diagnosticFiles) {
    const outcome = outcomes.find(f => f.resultRef === `${positiveScope}/${file}`);
    expect(outcome, `a successful read of ${file}`).toBeDefined();
    await openRecord(page, outcome!);
    await expectConfiguredRecord(page);
    await expect(page.getByText(outcome!.resultRef!, { exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath(`e16-positive-tool-record-${file.replace(/[^a-z0-9]+/gi, '-')}.png`), fullPage: true });
  }
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ref!)}`);
  await expect(page.getByText('Succeeded', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-positive-work.png'), fullPage: true });
});

test('truncated observation is marked in CLI, API and Portal @n8-truncation', async ({ page, request }, info) => {
  const view = await parity(request);
  const truncated = toolFacts(view).filter(f => f.kind === 'ProviderOutcome' && f.truncated);
  expect(truncated).toHaveLength(1);
  expect(truncated[0].providerStatus).toBe('Succeeded');
  expect(truncated[0].resultRef).toBe('repo:agenova/e16-faults/notes/incident-timeline.md');
  await openRecord(page, truncated[0]);
  await expect(page.getByText('Truncated to the configured observation limit')).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-truncated-record.png'), fullPage: true });
});

for (const [name, reason] of [['n6-timeout', 'tool-timeout'], ['n7-oversize', 'tool-response-too-large']] as const) {
  test(`${name}: the tool failure is explicit, never mock, and fails the Work @${name}`, async ({ page, request }, info) => {
    const view = await parity(request);
    expect(view.outcome?.status).toBe('Failed');
    const failed = toolFacts(view).filter(f => f.kind === 'ProviderOutcome' && f.providerStatus === 'Failed');
    expect(failed).toHaveLength(1);
    expect(failed[0].reasonCode).toBe(reason);
    expect(failed[0].resultRef).toBeUndefined();
    expect(failed[0].truncated).toBeUndefined();
    expect(toolFacts(view).some(f => f.reasonCode?.startsWith('mock-'))).toBe(false);
    await openRecord(page, failed[0]);
    await expect(page.getByRole('alert')).toBeVisible();
    await expectConfiguredRecord(page);
    await page.screenshot({ path: info.outputPath(`e16-${name}-record.png`), fullPage: true });
  });
}

// Slice 4 token-required backends (platform.yaml): each has one profile on its
// own scope that allows only README.md.
test('token-valid: the resolved token reads README.md through the configured MCP tool @token-valid', async ({ page, request }, info) => {
  const view = await parity(request);
  expect(view.outcome?.status).toBe('Succeeded');
  expect(view.facts).toEqual(expect.arrayContaining([expect.objectContaining({ kind: 'Runtime', operation: 'CleanupSucceeded' })]));
  expect(toolFacts(view).some(f => f.reasonCode?.startsWith('mock-'))).toBe(false);
  const calls = [...invocations(view).values()];
  expect(calls).toHaveLength(1);
  expect(calls[0].map(f => `${f.kind}:${f.result || f.providerStatus}`)).toEqual(['ToolDecision:Allow', 'ProviderAttempt:Attempted', 'ProviderOutcome:Succeeded']);
  const [, attempt, outcome] = calls[0];
  expect(attempt.reasonCode).toBe('configured-tool');
  expect(attempt.target).toBe('repo.read');
  expect(outcome.target).toBe(attempt.target);
  expect(outcome.resultRef).toBe('repo:agenova/e16-token/README.md');
  expect(outcome.truncated).toBeUndefined();
  await openRecord(page, outcome);
  await expectConfiguredRecord(page);
  await expect(page.getByText(outcome.resultRef!, { exact: true })).toBeVisible();
  await capture(page, info, 'e16-token-valid-record');
  await expectConfiguredWork(page, 'Succeeded');
  await expect(page.getByText('Succeeded', { exact: true }).first()).toBeVisible();
  await capture(page, info, 'e16-token-valid-work');
});

for (const [name, reason] of [['token-missing', 'tool-credential-unavailable'], ['token-wrong', 'tool-credential-rejected']] as const) {
  test(`${name}: the credential failure is explicit, never mock, never retried, and fails the Work @${name}`, async ({ page, request }, info) => {
    const view = await parity(request);
    expect(view.outcome?.status).toBe('Failed');
    expect(toolFacts(view).some(f => f.reasonCode?.startsWith('mock-'))).toBe(false);
    const calls = [...invocations(view).values()];
    expect(calls).toHaveLength(1);
    expect(calls[0].map(f => `${f.kind}:${f.result || f.providerStatus}`)).toEqual(['ToolDecision:Allow', 'ProviderAttempt:Attempted', 'ProviderOutcome:Failed']);
    const failed = toolFacts(view).filter(f => f.kind === 'ProviderOutcome' && f.providerStatus === 'Failed');
    expect(failed).toHaveLength(1);
    expect(failed[0].reasonCode).toBe(reason);
    expect(failed[0].resultRef).toBeUndefined();
    expect(failed[0].truncated).toBeUndefined();
    await openRecord(page, failed[0]);
    await expect(page.getByRole('alert')).toBeVisible();
    await expectConfiguredRecord(page);
    await capture(page, info, `e16-${name}-record`);
    await expectConfiguredWork(page, 'Failed');
    await capture(page, info, `e16-${name}-work`);
  });
}

test('admission denial has no claim and no tool or model activity @admission-deny', async ({ page, request }, info) => {
  const view = await parity(request);
  expect(view.state?.decision.result).toBe('Deny');
  expect(view.state?.claim).toBeFalsy();
  expect(view.facts.some(f => ['Runtime', 'ModelDecision', 'ToolDecision', 'ProviderAttempt', 'ProviderOutcome'].includes(f.kind))).toBe(false);
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ref!)}`);
  await expect(page.getByText('Denied', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-denied-work.png'), fullPage: true });
});
