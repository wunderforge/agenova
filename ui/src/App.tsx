// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';
import type { ClaimRequestedAccess, Evidence, EffectiveAuthority } from './contracts.generated';
import type { EvidenceResult, EvidenceSource } from './evidence-source';

function Values({ values }: { values?: string[] | null }) {
  return <span>{values == null ? 'Not supplied' : values.length ? values.join(', ') : 'None recorded'}</span>;
}
function Access({ access }: { access: ClaimRequestedAccess | EffectiveAuthority }) {
  return <dl><dt>Tools</dt><dd><Values values={access.tools}/></dd>
    <dt>Resource scopes</dt><dd><Values values={access.resourceScopes}/></dd>
    <dt>Model profile</dt><dd>{access.modelProfile || 'Not supplied'}</dd>
    <dt>Memory scopes</dt><dd><Values values={access.memoryScopes}/></dd></dl>;
}
function Facts({ evidence }: { evidence: Evidence }) {
  return <section><h2>Recorded evidence</h2>
    <p>Request reference: {evidence.requestRef}</p>
    <p>Decision references: <Values values={evidence.decisionIds}/></p>
    <p>Runtime events: <Values values={evidence.runtimeEvents?.map(event => event.kind)}/></p>
    <p>Tool invocations: {evidence.toolInvocations?.length ?? 'Not supplied'}</p>
    <p>Model invocations: {evidence.modelInvocations?.length ?? 'Not supplied'}</p>
    <p className="note">The v0 fixtures contain no detailed invocation history or lineage.</p>
  </section>;
}
export function EvidenceView({ result }: { result: EvidenceResult }) {
  switch (result.status) {
    case 'not-found': return <p role="status">No evidence found for this selection.</p>;
    case 'unavailable': return <p role="alert">Evidence source unavailable. No state inferred.</p>;
    case 'invalid': return <section role="alert"><h2>Invalid or incomplete source</h2>
      <p>No validated claim or authority is displayed.</p>
      <ul>{result.diagnostics.map((d, i) => <li key={i}>{d.category}: <code>{d.fieldPath}</code></li>)}</ul></section>;
    case 'request': {
      const request = result.data;
      return <><section><h2>Request intent only</h2><p>No issued decision or claim is supplied by this case.</p>
        <dl><dt>Name</dt><dd>{request.metadata.name}</dd><dt>Template</dt><dd>{request.spec.templateRef}</dd>
          <dt>Task type</dt><dd>{request.spec.task?.type || 'Not supplied'}</dd>
          <dt>Runtime profile</dt><dd>{request.spec.runtime?.profileRef || 'Not supplied'}</dd>
          <dt>Requested timeout</dt><dd>{request.spec.runtime?.timeout || 'Not supplied'}</dd></dl>
        <h3>Task input</h3><pre>{JSON.stringify(request.spec.task?.input ?? null, null, 2)}</pre></section>
        <section><h2>Requested access</h2><p>Intent does not grant authority.</p>
          {request.spec.requestedAccess ? <Access access={request.spec.requestedAccess}/> : <p>Not supplied</p>}</section></>;
    }
    case 'issued': {
      const state = result.data;
      return <><section><h2>Decision: {state.decision.result}</h2><p>{state.decision.reason}</p>
        <dl><dt>Principal</dt><dd>{state.principal.subject}</dd><dt>Action</dt><dd>{state.decision.action}</dd>
          <dt>Policy</dt><dd>{state.decision.policyRef.id} / {state.decision.policyRef.version}</dd></dl></section>
        <section><h2>Claim</h2>{state.claim ? <dl><dt>Claim ID</dt><dd>{state.claim.id}</dd>
          <dt>Phase</dt><dd>{state.claim.phase}</dd><dt>Authority reference</dt><dd>{state.claim.authorityRef}</dd>
          <dt>Backend</dt><dd>{state.claim.backendIdentity?.backend || 'Not supplied'}</dd>
          <dt>Worker</dt><dd>{state.claim.backendIdentity?.workerId || 'Not supplied'}</dd></dl> : <p>No claim issued.</p>}</section>
        <section><h2>Effective authority</h2>{state.effectiveAuthority ? <><p>{state.effectiveAuthority.id}</p>
          <Access access={state.effectiveAuthority}/><p>Runtime: {state.effectiveAuthority.runtime.profileRef} · {state.effectiveAuthority.runtime.timeout}</p></> : <p>No authority issued.</p>}
          <p className="note">Requested access is not supplied by this issued-state case.</p></section>
        <Facts evidence={state.evidence}/></>;
    }
  }
}

export function EvidencePanel({ source, selection }: { source: EvidenceSource; selection: string }) {
  const [loaded, setLoaded] = useState<{ key: string; source: EvidenceSource; result: EvidenceResult }>();
  useEffect(() => {
    let active = true;
    Promise.resolve().then(() => source.load(selection)).catch((): EvidenceResult => ({ status: 'unavailable' }))
      .then(result => { if (active) setLoaded({ key: selection, source, result }); });
    return () => { active = false; };
  }, [source, selection]);
  if (!loaded || loaded.key !== selection || loaded.source !== source) return <p role="status">Loading evidence…</p>;
  return <div aria-live="polite"><EvidenceView result={loaded.result}/></div>;
}

export interface Selection { id: string; provenance: string }
export function App({ source, selections }: { source: EvidenceSource; selections: Selection[] }) {
  const [selected, setSelected] = useState(selections[0]?.id ?? '');
  return <main><header><p className="eyebrow">AGENOVA / FIXTURE STORYBOARD</p><h1>Inspect the evidence</h1>
    <p>One worker assignment. Canonical v0 contracts. Read-only fixture data.</p></header>
    <label htmlFor="case">Fixture or derived display case</label>
    <select id="case" value={selected} onChange={event => setSelected(event.target.value)}>
      {selections.map(item => <option key={item.id} value={item.id}>{item.id}</option>)}
    </select>
    <p className="provenance">{selections.find(item => item.id === selected)?.provenance}</p>
    <EvidencePanel source={source} selection={selected}/>
    <footer>Fixture / Mock · No live API connection or operational controls.</footer>
  </main>;
}
