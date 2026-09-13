// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import rows from 'virtual:agenova-fixtures';
import { createFixtureSource, storyboardRows } from './fixture-source';
import { EvidencePanel, EvidenceView } from './App';
import type { EvidenceResult, EvidenceSource } from './evidence-source';

afterEach(cleanup);
const source = createFixtureSource(storyboardRows(rows));
it.each(rows)('renders canonical case $id', async row => {
  render(<EvidencePanel source={source} selection={row.id}/>);
  if (row.expected.outcome === 'invalid') {
    expect((await screen.findByRole('alert')).textContent).toContain(row.expected.category);
    expect(screen.queryByText('Effective authority')).toBeNull();
  } else if (row.subject === 'ClaimRequest') {
    expect(await screen.findByRole('heading', { name: 'Request intent only' })).toBeTruthy();
    expect(screen.getByText('Intent does not grant authority.')).toBeTruthy();
  } else {
    expect(await screen.findByRole('heading', { name: /Decision:/ })).toBeTruthy();
    if (row.id.endsWith('team-b-denial')) {
      expect(screen.getByText('No claim issued.')).toBeTruthy();
      expect(screen.getByText('No authority issued.')).toBeTruthy();
      expect(screen.queryByText('Running')).toBeNull();
    } else {
      expect(screen.getByText('Running')).toBeTruthy();
      expect(screen.getByText('ClaimRunning')).toBeTruthy();
    }
  }
});
it.each([['derived.missing-field', 'evidence'], ['derived.unknown-field', 'futureField']])('renders %s as incomplete', async (id, path) => {
  render(<EvidencePanel source={source} selection={id}/>);
  expect((await screen.findByRole('alert')).textContent).toContain(path);
  expect(screen.queryByRole('heading', { name: 'Effective authority' })).toBeNull();
});
it('replaces the fixture source without changing component semantics', async () => {
  const issued = await source.load('issued-state.valid.team-b-denial');
  const replacement: EvidenceSource = { load: vi.fn(async () => issued) };
  render(<EvidencePanel source={replacement} selection="opaque-request-key"/>);
  expect(await screen.findByRole('heading', { name: 'Decision: Deny' })).toBeTruthy();
  expect(replacement.load).toHaveBeenCalledWith('opaque-request-key');
});
it('renders derived ApprovalRequired as its own vocabulary, with no grant', async () => {
  const result = await source.load('issued-state.valid.team-b-denial');
  if (result.status !== 'issued') throw new Error('missing canonical base');
  result.data.decision.result = 'ApprovalRequired';
  render(<EvidenceView result={result}/>);
  expect(screen.getByRole('heading', { name: 'Decision: ApprovalRequired' })).toBeTruthy();
  expect(screen.getByText('No authority issued.')).toBeTruthy();
});
it('renders not-found and rejected source without exposing error details', async () => {
  const view = render(<EvidencePanel source={source} selection="not-found"/>);
  expect(await screen.findByText('No evidence found for this selection.')).toBeTruthy();
  view.rerender(<EvidencePanel source={{ load: async () => { throw new Error('private internal value'); } }} selection="error"/>);
  expect((await screen.findByRole('alert')).textContent).toBe('Evidence source unavailable. No state inferred.');
  expect(screen.queryByText('private internal value')).toBeNull();
});
it('does not show a stale grant while loading or after a late response', async () => {
  let finish!: (result: EvidenceResult) => void;
  const slow: EvidenceSource = { load: () => new Promise(resolve => { finish = resolve; }) };
  const view = render(<EvidencePanel source={slow} selection="old"/>);
  await waitFor(() => expect(finish).toBeTypeOf('function'));
  view.rerender(<EvidencePanel source={source} selection="issued-state.valid.team-b-denial"/>);
  expect(await screen.findByRole('heading', { name: 'Decision: Deny' })).toBeTruthy();
  finish(await source.load('issued-state.valid.team-a-engineer'));
  await waitFor(() => expect(screen.queryByRole('heading', { name: 'Decision: Allow' })).toBeNull());
});
