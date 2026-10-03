// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import type { Fact } from '../src/contracts.generated';
import type { Setup, View } from '../src/connected-source';

// E16 Slice 3 parity on the installed MCP campaign. Run only this file:
//   npx playwright test --config playwright.installed.config.ts installed/mcp.spec.ts
// The reference spec in this directory asserts the reference Policy and
// template, so it does not apply to the E16 install.
const cliPath = process.env.AGENOVA_CLI_PATH;
const stateDir = process.env.AGENOVA_CLI_STATE_DIR;
const refs = {
  positive: process.env.AGENOVA_E16_POSITIVE_REF,
  truncation: process.env.AGENOVA_E16_TRUNCATION_REF,
  failure: process.env.AGENOVA_E16_FAILURE_REF,
  denied: process.env.AGENOVA_E16_DENIED_REF,
};
if (!cliPath || !stateDir || Object.values(refs).some(ref => !ref)) {
  throw new Error('E16 installed parity requires AGENOVA_CLI_PATH, AGENOVA_CLI_STATE_DIR and AGENOVA_E16_{POSITIVE,TRUNCATION,FAILURE,DENIED}_REF.');
}

function cliJSON(args: string[]): unknown {
  return JSON.parse(execFileSync(cliPath!, ['--state-dir', stateDir!, ...args], { encoding: 'utf8', timeout: 60_000, windowsHide: true }));
}

async function parity(request: APIRequestContext, ref: string): Promise<View> {
  const response = await request.get(`/api/requests/${encodeURIComponent(ref)}/evidence`);
  expect(response.status()).toBe(200);
  const detail = await response.json() as View;
  expect(detail.requestRef).toBe(ref);
  expect(cliJSON(['work', 'show', ref, '--json'])).toEqual(detail);
  return detail;
}

const toolFacts = (view: View) => view.facts.filter(fact => fact.operation === 'tool.invoke');

// Each invocation: Allow, then attempt, then outcome, with one stable target.
function invocations(view: View): Map<string, Fact[]> {
  const out = new Map<string, Fact[]>();
  for (const fact of toolFacts(view)) {
    if (!fact.invocationId) continue;
    out.set(fact.invocationId, [...(out.get(fact.invocationId) ?? []), fact]);
  }
  return out;
}

async function openRecord(page: Page, ref: string, fact: Fact) {
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(ref)}/activity/${encodeURIComponent(fact.id)}`);
  await expect(page.getByText(fact.invocationId!, { exact: true }).first()).toBeVisible();
}

test('installed E16 setup reports the configured MCP tool and E16 registrations', async ({ request }) => {
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

test('positive Work: real MCP reads, two model turns, matching CLI, API and Portal', async ({ page, request }, info) => {
  const view = await parity(request, refs.positive!);
  expect(view.outcome?.status).toBe('Succeeded');
  expect(view.outcome?.model?.model).toBe('llama3.1:latest');
  expect(view.facts.filter(f => f.kind === 'ProviderOutcome' && f.operation === 'model.invoke' && f.providerStatus === 'Succeeded').length).toBeGreaterThanOrEqual(2);
  expect(view.facts).toEqual(expect.arrayContaining([expect.objectContaining({ kind: 'Runtime', operation: 'CleanupSucceeded' })]));
  expect(toolFacts(view).some(f => f.reasonCode?.startsWith('mock-'))).toBe(false);
  const calls = invocations(view);
  expect(calls.size).toBeGreaterThan(0);
  for (const [, facts] of calls) {
    expect(facts.map(f => `${f.kind}:${f.result || f.providerStatus}`)).toEqual(['ToolDecision:Allow', 'ProviderAttempt:Attempted', 'ProviderOutcome:Succeeded']);
    const [, attempt, outcome] = facts;
    expect(attempt.target).toBe('repo.read');
    expect(outcome.target).toBe(attempt.target);
    expect(outcome.resultRef).toMatch(/^repo:agenova\/e16-fixture\/(README\.md|logs\/timeout\.log|src\/retry\.txt)$/);
  }
  const outcome = [...calls.values()][0][2];
  await openRecord(page, refs.positive!, outcome);
  await expect(page.getByText('Tool call finished').first()).toBeVisible();
  await expect(page.getByText('Mock tool call', { exact: false })).toHaveCount(0);
  await expect(page.getByText(outcome.resultRef!, { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-positive-tool-record.png'), fullPage: true });
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(refs.positive!)}`);
  await expect(page.getByText('Succeeded', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-positive-work.png'), fullPage: true });
});

test('truncated observation is marked in CLI, API and Portal', async ({ page, request }, info) => {
  const view = await parity(request, refs.truncation!);
  const truncated = toolFacts(view).filter(f => f.kind === 'ProviderOutcome' && f.truncated);
  expect(truncated.length).toBeGreaterThan(0);
  expect(truncated[0].providerStatus).toBe('Succeeded');
  expect(truncated[0].resultRef).toBe('repo:agenova/e16-faults/notes/incident-timeline.md');
  await openRecord(page, refs.truncation!, truncated[0]);
  await expect(page.getByText('Truncated to the configured observation limit')).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-truncated-record.png'), fullPage: true });
});

test('tool failure is explicit, never mock, and fails the Work', async ({ page, request }, info) => {
  const view = await parity(request, refs.failure!);
  expect(view.outcome?.status).toBe('Failed');
  const failed = toolFacts(view).filter(f => f.kind === 'ProviderOutcome' && f.providerStatus === 'Failed');
  expect(failed.length).toBe(1);
  expect(['tool-timeout', 'tool-response-too-large']).toContain(failed[0].reasonCode);
  expect(failed[0].resultRef).toBeUndefined();
  expect(failed[0].truncated).toBeUndefined();
  await openRecord(page, refs.failure!, failed[0]);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByText('Tool call finished').first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-tool-failure-record.png'), fullPage: true });
});

test('admission denial has no claim and no tool or model activity', async ({ page, request }, info) => {
  const view = await parity(request, refs.denied!);
  expect(view.state?.decision.result).toBe('Deny');
  expect(view.state?.claim).toBeFalsy();
  expect(view.facts.some(f => ['Runtime', 'ModelDecision', 'ToolDecision', 'ProviderAttempt', 'ProviderOutcome'].includes(f.kind))).toBe(false);
  await page.goto(`/?mode=connected#/work/${encodeURIComponent(refs.denied!)}`);
  await expect(page.getByText('Denied', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: info.outputPath('e16-denied-work.png'), fullPage: true });
});
