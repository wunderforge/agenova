// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { activityBuckets, WorkActivityChart } from './WorkActivityChart';

afterEach(cleanup);
it('buckets supplied requests once by UTC date/hour, not execution calls or local time', () => {
  const buckets = activityBuckets([
    { timestamp: '2026-10-19T23:30:00-02:00', status: 'Succeeded' },
    { timestamp: '2026-10-20T01:20:00Z', status: 'Denied' },
    { timestamp: '2026-10-20T01:40:00Z', status: 'Running' },
    { timestamp: '2026-10-20T01:50:00Z', status: 'Expired' },
    { timestamp: '2026-10-19T01:20:00Z', status: 'Failed' },
    { timestamp: 'invalid', status: 'Failed' },
    { status: 'Succeeded' },
  ], '2026-10-20');
  expect(buckets[1]).toEqual({ Succeeded: 1, Denied: 1, Failed: 0, 'In progress': 1, Other: 1 });
  expect(buckets.flatMap(Object.values).reduce((a, b) => a + b, 0)).toBe(4);
});
it('shows recorded dates and discloses missing timestamps rather than inventing traffic', () => {
  const view = render(<WorkActivityChart entries={[
    { timestamp: '2026-10-20T09:00:00Z', status: 'Succeeded' },
    { timestamp: '2026-10-19T08:00:00Z', status: 'Failed' },
    { status: 'Denied' },
  ]}/>);
  expect(screen.getByRole('img').getAttribute('aria-label')).toContain('1 loaded requests on 2026-10-20');
  expect(screen.getByText(/Loaded list only/).textContent).toContain('1 without a recorded request time');
  fireEvent.change(screen.getByLabelText('Activity date'), { target: { value: '2026-10-19' } });
  expect(screen.getByRole('img').getAttribute('aria-label')).toContain('2026-10-19');
  view.rerender(<WorkActivityChart entries={[]}/>);
  expect(screen.queryByRole('img')).toBeNull();
  expect(screen.getByText('No timestamped requests in this loaded list.')).toBeDefined();
});
