// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Illustrative UI stories only. These are not canonical contract fixtures,
// persisted history, policy evaluation, or observed gateway/backend calls.
export type WorkStatus = 'Running' | 'Succeeded' | 'Denied' | 'Failed' | 'Pending';
export type EventKind = 'Request' | 'Decision' | 'Claim' | 'Runtime' | 'Tool' | 'Model';
export type RecordResult = 'Recorded' | 'Allowed' | 'Denied';
export interface AccessSet {
  resourceScopes: string[];
  tools: string[];
  model: string;
  memory: string[];
}
export interface WorkEvent {
  id: string;
  kind: EventKind;
  title: string;
  time: string;
  result: RecordResult;
  source: string;
  target: string;
  description: string;
  action?: string;
  policy?: string;
  externalCall?: false;
  effective?: AccessSet & { runtime: string };
}
export interface WorkItem {
  id: string;
  title: string;
  status: WorkStatus;
  agent: string;
  team: string;
  principal: string;
  context?: { label: string; value: string };
  branch?: string;
  submitted: string;
  updated: string;
  requestRef: string;
  claimId?: string;
  backend?: string;
  worker?: string;
  policy?: string;
  decision: string;
  decisionReason: string;
  outcome?: string;
  requestedRuntime: string;
  effectiveRuntime?: string;
  requested: AccessSet;
  ceiling?: AccessSet & { runtime: string };
  granted?: AccessSet;
  events: WorkEvent[];
}
export interface AgentSummary {
  id: string;
  name: string;
  purpose: string;
  model: string;
  memory: string;
  runtime: string;
  runtimeProfiles: string[];
  tools: string[];
}

export const agents: AgentSummary[] = [
  { id: 'engineer', name: 'Engineer', purpose: 'Change code, run tests, and prepare a pull request.', model: 'coding-standard', memory: 'team-docs', runtime: 'general-purpose', runtimeProfiles: ['general-purpose', 'cpu-intensive'], tools: ['git.read', 'git.write', 'github.pull-request'] },
  { id: 'reviewer', name: 'Reviewer', purpose: 'Inspect a proposed change and report findings.', model: 'coding-standard', memory: 'team-docs', runtime: 'general-purpose', runtimeProfiles: ['general-purpose'], tools: ['git.read', 'github.review'] },
  { id: 'researcher', name: 'Researcher', purpose: 'Gather and summarize information from approved sources.', model: 'research-standard', memory: 'team-docs', runtime: 'general-purpose', runtimeProfiles: ['general-purpose'], tools: ['web.search', 'web.read'] },
];

export const demoIdentity = { displayName: 'Team A engineer', subject: 'user:team-a-engineer', team: 'team-a', authenticationContext: 'upstream:test' };
export const demoPolicy = { id: 'reference-default-deny', version: '1', rules: [
  { team: 'Team A', scope: 'Engineer · payments', result: 'Allowed' },
  { team: 'Team A', scope: 'Engineer · billing', result: 'Allowed' },
  { team: 'Team A', scope: 'Researcher · identity', result: 'Allowed' },
] };

const evt = (id: string, kind: EventKind, title: string, time: string, result: RecordResult, source: string, target: string, description: string, extra: Partial<WorkEvent> = {}): WorkEvent =>
  ({ id, kind, title, time, result, source, target, description, ...extra });
const paymentAccess: AccessSet = { resourceScopes: ['repo:acme/payments'], tools: ['git.read', 'git.write', 'github.pull-request'], model: 'coding-standard', memory: ['team-docs'] };
const billingAccess: AccessSet = { resourceScopes: ['repo:acme/billing'], tools: ['git.read', 'git.write', 'github.pull-request'], model: 'coding-standard', memory: ['team-docs'] };

export const exampleWorks: WorkItem[] = [
  {
    id: 'fix-payment-timeout', title: 'Fix the payment timeout bug', status: 'Running', agent: 'engineer', team: 'Team A', principal: demoIdentity.subject,
    context: { label: 'Repository', value: 'acme/payments' }, branch: 'main', submitted: '14 Sep 2026, 09:42', updated: '09:47', requestRef: 'fix-payment-timeout',
    claimId: 'claim:fix-payment-timeout:1', backend: 'reference', worker: 'worker:fix-payment-timeout:1', policy: 'reference-default-deny / v1', decision: 'Allowed',
    decisionReason: 'Team A may create engineer work for the payments project.', requestedRuntime: 'general-purpose · 20 min', requested: paymentAccess, granted: paymentAccess,
    events: [
      evt('payment-request', 'Request', 'Work requested', '09:42:16', 'Recorded', 'Agenova', 'acme/payments', 'Team A submitted a request for the engineer agent.'),
      evt('payment-decision', 'Decision', 'Work authorized', '09:42:17', 'Allowed', 'Policy', 'claim.create', 'Team A may create this engineer assignment before a claim is issued.', { policy: 'reference-default-deny / v1', action: 'claim.create' }),
      evt('payment-claim', 'Claim', 'Claim issued', '09:42:18', 'Recorded', 'Agenova', 'claim:fix-payment-timeout:1', 'A claim was issued with resolved authority.'),
      evt('payment-running', 'Runtime', 'Worker started', '09:44:08', 'Recorded', 'Runtime', 'reference backend', 'The worker began the task.'),
      evt('payment-read', 'Tool', 'Repository read', '09:44:21', 'Allowed', 'Tool Gateway', 'repo:acme/payments', 'git.read matched the active claim scope.', { action: 'git.read', policy: 'effective claim authority' }),
      evt('payment-model', 'Model', 'Model request', '09:45:03', 'Allowed', 'Model Gateway', 'coding-standard', 'The request matched the granted model profile.', { action: 'model.invoke', policy: 'effective claim authority' }),
      evt('payment-block', 'Tool', 'Other repository blocked', '09:46:12', 'Denied', 'Tool Gateway', 'repo:acme/billing', 'acme/billing is outside this claim scope. No external call was made.', { action: 'git.read', policy: 'effective claim authority', externalCall: false }),
    ],
  },
  {
    id: 'invoice-retry-tests', title: 'Update invoice retry tests', status: 'Succeeded', agent: 'engineer', team: 'Team A', principal: demoIdentity.subject,
    context: { label: 'Repository', value: 'acme/billing' }, branch: 'main', submitted: '13 Sep 2026, 15:10', updated: '15:29', requestRef: 'invoice-retry-tests',
    claimId: 'claim:invoice-retry-tests:1', backend: 'reference', worker: 'worker:invoice-retry-tests:1', policy: 'reference-default-deny / v1', decision: 'Allowed',
    decisionReason: 'Team A may create engineer work for the billing project.', outcome: 'Tests updated; pull request #482 prepared.',
    requestedRuntime: 'cpu-intensive · 45 min', effectiveRuntime: 'cpu-intensive · 30 min',
    requested: { ...billingAccess, tools: [...billingAccess.tools, 'shell.exec'] },
    ceiling: { resourceScopes: ['repo:acme/*'], tools: billingAccess.tools, model: 'coding-standard', memory: ['team-docs'], runtime: 'general-purpose or cpu-intensive · max 30 min' },
    granted: billingAccess,
    events: [
      evt('invoice-request', 'Request', 'Work requested', '15:10:04', 'Recorded', 'Agenova', 'acme/billing', 'Team A submitted the request.'),
      evt('invoice-decision', 'Decision', 'Work authorized', '15:10:05', 'Allowed', 'Policy', 'claim.create', 'The request passed assignment authorization.', { action: 'claim.create', policy: 'reference-default-deny / v1' }),
      evt('invoice-authority', 'Decision', 'Access narrowed', '15:10:06', 'Recorded', 'Agenova', 'engineer template', 'shell.exec was outside the template ceiling; the 45-minute request was capped at 30 minutes.', { action: 'authority.resolve', policy: 'reference-default-deny / v1', effective: { ...billingAccess, runtime: 'cpu-intensive · 30 min' } }),
      evt('invoice-claim', 'Claim', 'Claim issued', '15:10:07', 'Recorded', 'Agenova', 'claim:invoice-retry-tests:1', 'A claim was issued with the resolved authority.'),
      evt('invoice-running', 'Runtime', 'Worker started', '15:11:20', 'Recorded', 'Runtime', 'reference backend', 'The worker began the task.'),
      evt('invoice-model', 'Model', 'Model request', '15:12:42', 'Allowed', 'Model Gateway', 'coding-standard', 'The agent used the granted model profile.', { action: 'model.invoke', policy: 'effective claim authority' }),
      evt('invoice-write', 'Tool', 'Repository change', '15:18:16', 'Allowed', 'Tool Gateway', 'repo:acme/billing', 'git.write matched the active claim scope.', { action: 'git.write', policy: 'effective claim authority' }),
      evt('invoice-pr', 'Tool', 'Pull request prepared', '15:25:49', 'Allowed', 'Tool Gateway', 'acme/billing #482', 'github.pull-request matched the active claim.', { action: 'github.pull-request', policy: 'effective claim authority' }),
      evt('invoice-end', 'Runtime', 'Work completed', '15:29:02', 'Recorded', 'Runtime', 'claim:invoice-retry-tests:1', 'The claim reached Succeeded; further governed calls would be denied.'),
    ],
  },
  {
    id: 'review-checkout-change', title: 'Review checkout change', status: 'Denied', agent: 'reviewer', team: 'Team B', principal: 'user:team-b-reviewer',
    context: { label: 'Repository', value: 'acme/payments' }, branch: 'feature/checkout', submitted: '13 Sep 2026, 11:33', updated: '11:33', requestRef: 'review-checkout-change',
    policy: 'reference-default-deny / v1', decision: 'Denied', decisionReason: 'Team B cannot create reviewer work for payments.', requestedRuntime: 'general-purpose · 15 min',
    requested: { resourceScopes: ['repo:acme/payments'], tools: ['git.read', 'github.review'], model: 'coding-standard', memory: ['team-docs'] },
    events: [
      evt('review-request', 'Request', 'Work requested', '11:33:02', 'Recorded', 'Agenova', 'acme/payments', 'Team B submitted a reviewer request.'),
      evt('review-decision', 'Decision', 'Request denied', '11:33:03', 'Denied', 'Policy', 'claim.create', 'Team B cannot create reviewer work here. No claim or worker was created.', { action: 'claim.create', policy: 'reference-default-deny / v1', externalCall: false }),
    ],
  },
  {
    id: 'auth-failure-investigation', title: 'Investigate login failures', status: 'Failed', agent: 'researcher', team: 'Team A', principal: 'user:team-a-analyst',
    context: { label: 'Topic', value: 'Customer authentication' }, submitted: '12 Sep 2026, 10:06', updated: '10:11', requestRef: 'auth-failure-investigation',
    claimId: 'claim:auth-failure-investigation:1', backend: 'reference', worker: 'worker:auth-failure-investigation:1', policy: 'reference-default-deny / v1', decision: 'Allowed',
    decisionReason: 'Team A may create researcher work for identity.', outcome: 'The worker stopped before it produced a result.', requestedRuntime: 'general-purpose · 20 min',
    requested: { resourceScopes: [], tools: ['web.search', 'web.read'], model: 'research-standard', memory: ['team-docs'] },
    granted: { resourceScopes: [], tools: ['web.search', 'web.read'], model: 'research-standard', memory: ['team-docs'] },
    events: [
      evt('auth-request', 'Request', 'Work requested', '10:06:10', 'Recorded', 'Agenova', 'Login failures', 'Team A submitted the request.'),
      evt('auth-decision', 'Decision', 'Work authorized', '10:06:11', 'Allowed', 'Policy', 'claim.create', 'The request passed assignment authorization.', { action: 'claim.create', policy: 'reference-default-deny / v1' }),
      evt('auth-claim', 'Claim', 'Claim issued', '10:06:12', 'Recorded', 'Agenova', 'claim:auth-failure-investigation:1', 'A claim was issued.'),
      evt('auth-running', 'Runtime', 'Worker started', '10:07:02', 'Recorded', 'Runtime', 'reference backend', 'The worker began the task.'),
      evt('auth-failed', 'Runtime', 'Worker stopped', '10:11:19', 'Recorded', 'Runtime', 'claim:auth-failure-investigation:1', 'The claim entered Failed before a result was produced.'),
    ],
  },
];
