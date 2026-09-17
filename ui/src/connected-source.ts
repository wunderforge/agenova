// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { AgentTemplate, ClaimRequest, Fact, Principal, View as CanonicalView } from './contracts.generated';
import { shapeDiagnostics } from './shape-check';
import { displayWorkName } from './work-name';

// Live views must contain the canonical request and a fact array.
export type View = Omit<CanonicalView, 'request' | 'facts'> & {
  request: ClaimRequest;
  facts: Fact[];
};
export interface Setup {
  principal: Principal;
  template: AgentTemplate;
  policy: {
    ID: string;
    Version: string;
    Rules: { team: string; action: string; project: string; templateRef: string }[];
  };
  capabilities: Record<string, string>;
}
export class SourceError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
  }
}

async function json(path: string, signal?: AbortSignal, body?: ClaimRequest): Promise<unknown> {
  const timeout = AbortSignal.timeout(8000);
  const response = await fetch(path, {
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
    cache: 'no-store',
    ...(body ? {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    } : {}),
  });
  if (!response.ok) {
    const message = response.status === 404 ? 'This work was not found.'
      : body ? 'The request could not be started. Check the connection and try again.'
      : 'The connection is unavailable. Try again.';
    throw new SourceError(response.status, message);
  }
  try { return await response.json(); }
  catch { throw new SourceError(502, 'The connection returned an incomplete response.'); }
}

function view(data: unknown): View {
  if (shapeDiagnostics('View', data).length || (data as View).version !== 'agenova.evidence/v0') {
    throw new SourceError(502, 'The connection returned an incomplete work record.');
  }
  return data as View;
}

export const connectedSource = {
  async setup(signal?: AbortSignal): Promise<Setup> {
    const data = await json('/api/setup', signal);
    if (!data || typeof data !== 'object') throw new SourceError(502, 'Platform setup is unavailable.');
    const setup = data as Setup;
    if (shapeDiagnostics('Principal', setup.principal).length ||
        shapeDiagnostics('AgentTemplate', setup.template).length ||
        !setup.policy || typeof setup.policy.ID !== 'string' ||
        typeof setup.policy.Version !== 'string' || !Array.isArray(setup.policy.Rules) ||
        !setup.policy.Rules.every(rule => rule &&
          [rule.team, rule.action, rule.project, rule.templateRef].every(value => typeof value === 'string')) ||
        !setup.capabilities || typeof setup.capabilities !== 'object' ||
        !Object.values(setup.capabilities).every(value => typeof value === 'string')) {
      throw new SourceError(502, 'Platform setup is incomplete.');
    }
    return setup;
  },
  async list(signal?: AbortSignal): Promise<View[]> {
    const data = await json('/api/requests', signal);
    if (!Array.isArray(data)) throw new SourceError(502, 'The connection returned an incomplete work list.');
    return data.map(view);
  },
  async request(ref: string, signal?: AbortSignal): Promise<View> {
    return view(await json(`/api/requests/${encodeURIComponent(ref)}/evidence`, signal));
  },
  async submit(request: ClaimRequest, signal?: AbortSignal): Promise<View> {
    return view(await json('/api/requests', signal, request));
  },
};

export function workTitle(work: View): string {
  const input = work.request.spec.task?.input;
  return displayWorkName(input?.workName, input?.objective, work.requestRef);
}
export function workStatus(work: View): string {
  if (work.state?.decision.result === 'Deny') return 'Denied';
  if (work.state?.decision.result === 'ApprovalRequired') return 'Approval required';
  // Queued work can be cancelled before allocation, leaving the canonical
  // claim Pending. The final outcome is authoritative for that work status.
  if (work.outcome?.status === 'Cancelled') return 'Cancelled';
  const phase = work.state?.claim?.phase;
  if (phase === 'Bound') return 'Starting';
  // Terminal claim authority can precede the final work/cleanup report.
  if (phase === 'Succeeded' && !work.outcome) return 'Finishing';
  return phase || 'Pending';
}
export function isTerminal(work: View): boolean {
  return ['Succeeded', 'Failed', 'Expired', 'Denied', 'Approval required', 'Cancelled'].includes(workStatus(work));
}
