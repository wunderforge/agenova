// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { Fact, MemoryMetadata, PolicyReference } from './contracts.generated';
import type { View } from './connected-source';

const operations = ['memory.read', 'memory.write', 'memory.invalid'];
const denialReasons = ['memory-denied', 'memory-context-mismatch', 'memory-claim-inactive', 'memory-context-cancelled',
  'memory-invalid-input', 'memory-operation-not-granted', 'memory-scope-not-granted', 'memory-ownership-denied'];
const memoryFact = (f: Fact) => f.kind === 'MemoryDecision' || !!f.operation?.startsWith('memory.');
const samePolicy = (a?: PolicyReference | null, b?: PolicyReference | null) => !!a && !!b && a.id === b.id && a.version === b.version;
const sameList = (a?: string[] | null, b?: string[] | null) => (a || []).length === (b || []).length && (a || []).every((v, i) => v === b?.[i]);

function validMetadata(m: MemoryMetadata | null | undefined, operation: string, provider: string | undefined): boolean {
  if (!m || !Number.isSafeInteger(m.count) || m.count < 0 || m.count > 10 ||
      !Number.isSafeInteger(m.durationMilliseconds) || m.durationMilliseconds < 0) return false;
  const refs = m.references || [];
  if (refs.length !== m.count || new Set(refs).size !== refs.length || !refs.every(r => /^memory:[a-f0-9]{32}$/.test(r))) return false;
  if (m.status === 'Written') return operation === 'memory.write' && provider === 'Succeeded' && m.count === 1 && !m.truncated;
  if (m.status === 'Found') return operation === 'memory.read' && provider === 'Succeeded' && (m.count > 0 || m.truncated);
  if (m.status === 'Empty') return operation === 'memory.read' && provider === 'Succeeded' && m.count === 0 && !m.truncated;
  if (m.status === 'Cancelled') return provider === 'Cancelled' && m.count === 0 && !m.truncated;
  return ['Unsupported', 'Unavailable', 'Timeout', 'Failed', 'WriteUncertain'].includes(m.status) &&
    provider === 'Failed' && m.count === 0 && !m.truncated && (m.status !== 'WriteUncertain' || operation === 'memory.write');
}

function validFact(f: Fact): boolean {
  if (f.reason || f.decision || f.effectiveAuthority || f.authorityChanges?.length || f.backendIdentity || !f.policyRef) return false;
  if (f.kind === 'MemoryDecision') {
    if (!operations.includes(f.operation || '') || f.memory || f.providerStatus) return false;
    if (f.result === 'Allow') return f.operation !== 'memory.invalid' && !!f.target?.trim() && f.reasonCode === 'memory-allowed';
    return f.result === 'Deny' && denialReasons.includes(f.reasonCode || '');
  }
  if (f.kind === 'ProviderAttempt') return f.operation !== 'memory.invalid' && !f.memory && !f.result && f.providerStatus === 'Attempted' && f.reasonCode === 'memory-allowed';
  if (f.kind !== 'ProviderOutcome' || f.result || !validMetadata(f.memory, f.operation!, f.providerStatus)) return false;
  const status = f.memory!.status;
  const reason = ['Written', 'Found', 'Empty'].includes(status) ? 'memory-completed'
    : status === 'WriteUncertain' ? 'memory-write-uncertain' : `memory-${status.toLowerCase()}`;
  return f.reasonCode === reason;
}

// Display integrity only. This validates observed grants; it never issues authority.
export function validMemoryEvidence(work: View, expectedRef = work.requestRef): boolean {
  if (!work.facts.some(f => memoryFact(f) || f.memory)) return true;
  const state = work.state, claim = state?.claim, grant = state?.effectiveAuthority, access = work.request.spec.requestedAccess;
  if (!state || !claim?.backendIdentity || !grant || !access || !(access.memoryScopes?.length || access.memoryOperations?.length) ||
      work.requestRef !== expectedRef || !claim.id || !grant.id || !claim.backendIdentity.backend || !claim.backendIdentity.workerId ||
      state.requestRef !== work.requestRef || state.evidence.requestRef !== work.requestRef || work.request.metadata.name !== work.requestRef || claim.requestRef !== work.requestRef ||
      claim.authorityRef !== grant.id || state.decision.result !== 'Allow' || !samePolicy(state.decision.policyRef, state.policyRef) ||
      state.action.name !== 'claim.create' || state.action.project !== work.request.spec.projectRef ||
      state.action.templateRef !== work.request.spec.templateRef || !state.principal.team || !state.principal.authenticationContext ||
      !state.policyRef.id || !state.policyRef.version || !state.principal.subject || state.decision.principalRef !== state.principal.subject) return false;
  for (const values of [grant.memoryOperations,grant.memoryScopes,access.memoryOperations,access.memoryScopes]) {
    if (values && (new Set(values).size !== values.length || values.some(value => !value.trim()))) return false;
  }
  if ((grant.memoryOperations || []).some(op => !['read', 'write'].includes(op) || !access.memoryOperations?.includes(op)) ||
      (grant.memoryScopes || []).some(scope => !access.memoryScopes?.includes(scope))) return false;
  const ids = new Set<string>(), decisions = new Set<string>();
  const calls = new Map<string, { operation: string; target?: string; stage: number }>();
  const runtime: string[] = [];
  let sequence = 0, authority = false, bound = false, ready = false, running = false, terminal = '', ended = false;
  for (const f of work.facts) {
    if (ended || !f.id || ids.has(f.id) || !Number.isSafeInteger(f.sequence) || f.sequence <= sequence ||
        !Number.isFinite(Date.parse(f.timestamp)) || f.requestRef !== work.requestRef || f.claimId && f.claimId !== claim.id) return false;
    ids.add(f.id); sequence = f.sequence;
    if (f.kind === 'AuthorityResolved') {
      if (authority || f.claimId !== claim.id || !samePolicy(f.policyRef, state.policyRef) || f.effectiveAuthority?.id !== grant.id ||
          !sameList(f.effectiveAuthority.memoryScopes, grant.memoryScopes) || !sameList(f.effectiveAuthority.memoryOperations, grant.memoryOperations)) return false;
      authority = true;
    }
    if (f.kind === 'Runtime') {
      if (f.claimId !== claim.id) return false;
      if (f.operation !== 'Pending') runtime.push(f.operation || '');
      if (f.operation === 'Bound') {
        if (!authority || bound || f.backendIdentity?.backend !== claim.backendIdentity.backend || f.backendIdentity?.workerId !== claim.backendIdentity.workerId) return false;
        bound = true;
      }
      if (f.operation === 'BackendReady') { if (!bound || ready) return false; ready = true; }
      if (f.operation === 'Running') { if (!ready || running || terminal) return false; running = true; }
      if (['Succeeded', 'Failed', 'Expired', 'Cancelled', 'StartFailed', 'AllocationFailed'].includes(f.operation || '')) {
        if (terminal || !bound) return false;
        terminal = f.operation!;
      }
    }
    if (f.kind === 'RunOutcome') ended = true;
    if (['MemoryDecision', 'ModelDecision', 'ToolDecision'].includes(f.kind) && f.invocationId) {
      if (decisions.has(f.invocationId)) return false;
      decisions.add(f.invocationId);
    }
    if (!memoryFact(f)) {
      if (f.memory || f.invocationId && calls.has(f.invocationId)) return false;
      continue;
    }
    if (!authority || !bound || !running || f.claimId !== claim.id || !f.invocationId || !samePolicy(f.policyRef, state.policyRef) || !validFact(f)) return false;
    if (f.kind === 'MemoryDecision') {
      if (f.result === 'Allow' && (!!terminal || !grant.memoryOperations?.includes(f.operation!.slice(7)) || !grant.memoryScopes?.includes(f.target!))) return false;
      calls.set(f.invocationId, {operation: f.operation!, target: f.target, stage: f.result === 'Allow' ? 1 : 0});
      continue;
    }
    const call = calls.get(f.invocationId);
    if (!call || call.operation !== f.operation || (call.target || '') !== (f.target || '') || !running) return false;
    if (terminal && (!['Cancelled', 'Expired'].includes(terminal) || f.kind !== 'ProviderOutcome' ||
        f.operation === 'memory.read' && !['Cancelled', 'Timeout'].includes(f.memory!.status))) return false;
    if (f.kind === 'ProviderAttempt') { if (call.stage !== 1) return false; call.stage = 2; }
    else { if (call.stage !== 2) return false; call.stage = 3; }
  }
  if (!sameList(runtime, state.evidence.runtimeEvents?.map(event => event.kind))) return false;
  if (terminal && (terminal === 'Succeeded' ? claim.phase !== 'Succeeded' : terminal === 'Expired' ? claim.phase !== 'Expired' : claim.phase !== 'Failed')) return false;
  return !work.outcome || !!terminal && [...calls.values()].every(call => call.stage === 0 || call.stage === 3);
}

export function validMemoryList(works: View[]): boolean {
  if (!works.some(work => work.facts.some(f => memoryFact(f) || f.memory))) return true;
  const refs = new Set<string>(), owners = new Map<string, string>(), sequences = new Set<number>();
  for (const work of works) {
    if (refs.has(work.requestRef)) return false;
    refs.add(work.requestRef);
    const identities = [work.state?.claim?.id && `claim:${work.state.claim.id}`, work.state?.effectiveAuthority?.id && `authority:${work.state.effectiveAuthority.id}`,
      work.state?.claim?.backendIdentity && `worker:${JSON.stringify([work.state.claim.backendIdentity.backend,work.state.claim.backendIdentity.workerId])}`,
      ...work.facts.flatMap(f => [`fact:${f.id}`, f.invocationId && `invocation:${f.invocationId}`])];
    for (const id of identities) {
      if (!id) continue;
      if (owners.has(id) && owners.get(id) !== work.requestRef) return false;
      owners.set(id, work.requestRef);
    }
    for (const f of work.facts) { if (sequences.has(f.sequence)) return false; sequences.add(f.sequence); }
  }
  return true;
}
