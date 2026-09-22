// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { afterEach, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { IncidentRoleDemo } from './IncidentRoleDemo';

afterEach(cleanup);

it('keeps the request fixed while a mock principal changes the proposed grant', () => {
  render(<IncidentRoleDemo/>);
  const sameWork = screen.getByRole('heading', { name: '不变的请求与模板' }).closest('section');
  if (!sameWork) throw new Error('missing same-work section');
  const requestBefore = sameWork.textContent;
  expect(screen.getByText(/演示模拟/)).toBeTruthy();
  expect(screen.getByText('demo:payments-developer · Payments Development')).toBeTruthy();
  expect(screen.getByText('准备修复 PR')).toBeTruthy();
  expect(screen.getByText('deploy.rollback', { selector: '.incident-demo-decision strong' })).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: /值班 SRE/ }));
  expect(screen.getByText('demo:on-call-sre · Reliability')).toBeTruthy();
  expect(screen.getByText('执行受控回滚')).toBeTruthy();
  expect(screen.getByText('github.pull-request', { selector: '.incident-demo-decision strong' })).toBeTruthy();
  expect(within(sameWork).getByText('payment-incident-responder')).toBeTruthy();
  expect(sameWork.textContent).toBe(requestBefore);
});
