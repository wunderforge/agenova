// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { WorkOverview } from './WorkOverview';
afterEach(cleanup);
it('counts only supplied work outcomes, not grants or global platform traffic', () => {
  const view = render(<WorkOverview statuses={['Succeeded', 'Running', 'Denied', 'Failed', 'Finishing']}/>);
  expect(screen.getByLabelText('Loaded work summary').textContent).toContain('In this list');
  expect([...view.container.querySelectorAll('strong')].map(el => el.textContent)).toEqual(['5', '1', '1', '1']);
  view.rerender(<WorkOverview statuses={[]}/>);
  expect([...view.container.querySelectorAll('strong')].map(el => el.textContent)).toEqual(['0', '0', '0', '0']);
});
