// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState, type MouseEvent } from 'react';
import type { ClaimRequestedAccess, EffectiveAuthority } from './contracts.generated';
import type { EvidenceSource } from './evidence-source';
import { loadConsole, type ConsoleResult, type ConsoleReferenceKeys } from './console-load';
import { consoleHref, scenarios, type ConsoleRoute } from './console-route';

export function navigate(href: string) {
  window.history.pushState(null, '', href);
  window.dispatchEvent(new PopStateEvent('popstate'));
}
function follow(event: MouseEvent<HTMLAnchorElement>) {
  if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
  event.preventDefault(); navigate(event.currentTarget.href);
}
export function ConsoleNavigation() {
  return <nav aria-label="Fixture stories" className="console-nav">
    <a onClick={follow} href={consoleHref('requests', 'fix-payment-timeout')}>Allowed request</a>
    <a onClick={follow} href={consoleHref('requests', 'fix-payment-timeout', 'narrowed')}>Derived narrowed request</a>
    <a onClick={follow} href={consoleHref('requests', 'fix-payment-timeout-team-b')}>Denied request</a>
    <a href="/fixtures">Contract storyboard</a>
  </nav>;
}

const list = (values: string[] | null | undefined) => values == null ? 'Not supplied' : values.length ? values.join(', ') : 'None recorded';
function difference(requested: string[] | null | undefined, effective: string[] | null | undefined) {
  if (!requested || !effective) return 'Comparison unavailable';
  const absent = requested.filter(value => !effective.includes(value));
  const extra = effective.filter(value => !requested.includes(value));
  if (extra.length) return `Recorded effective values outside request: ${extra.join(', ')}`;
  return absent.length ? `Requested, not granted: ${absent.join(', ')}` : 'Same recorded access';
}
function AuthorityComparison({ requested, effective }: { requested?: ClaimRequestedAccess; effective?: EffectiveAuthority | null }) {
  const fields = [
    { name: 'Tools', requested: requested?.tools, effective: effective?.tools },
    { name: 'Resource scopes', requested: requested?.resourceScopes, effective: effective?.resourceScopes },
    { name: 'Memory scopes', requested: requested?.memoryScopes, effective: effective?.memoryScopes },
    { name: 'Model profile', requested: requested?.modelProfile ? [requested.modelProfile] : undefined,
      effective: effective?.modelProfile ? [effective.modelProfile] : undefined },
  ];
  return <section aria-labelledby="access-title" className="console-wide"><h2 id="access-title">Requested vs effective authority</h2>
    <p>Requested access is intent. Only the issued snapshot describes granted access.</p>
    {!effective && <p className="console-notice">No authority issued.</p>}
    <div className="access-table"><table><caption>Recorded access comparison</caption>
      <thead><tr><th scope="col">Access</th><th scope="col">Requested</th><th scope="col">Effective</th><th scope="col">Recorded difference</th></tr></thead>
      <tbody>{fields.map(field => <tr key={field.name}><th scope="row">{field.name}</th><td>{list(field.requested)}</td>
        <td>{effective ? list(field.effective) : 'No authority issued'}</td><td>{difference(field.requested, field.effective)}</td></tr>)}</tbody>
    </table></div>
    <p className="note">This view compares recorded values; it does not evaluate policy or grant access.</p>
  </section>;
}

export function ConsoleEvidence({ result }: { result: ConsoleResult }) {
  if (result.status === 'not-found') return <section role="status"><h2>Reference not found</h2><p>No evidence supplied for this request or claim.</p></section>;
  if (result.status === 'unavailable') return <section role="alert"><h2>Evidence unavailable</h2><p>The fixture source is unavailable. No previous claim is displayed.</p></section>;
  if (result.status === 'invalid') return <section role="alert"><h2>Malformed evidence</h2><p>Validated claim and authority withheld.</p>
    <ul>{result.diagnostics.map((item, index) => <li key={index}>{item.category}: <code>{item.fieldPath}</code></li>)}</ul></section>;
  const { issued: state, request, requestGap } = result;
  return <div className="console-grid">
    <section aria-labelledby="request-title"><h2 id="request-title">Request & principal</h2>
      <dl><dt>Request reference</dt><dd>{state.requestRef}</dd><dt>Trusted principal</dt><dd>{state.principal.subject}</dd>
        <dt>Team</dt><dd>{state.principal.team || 'Not supplied'}</dd><dt>Template</dt><dd>{state.action.templateRef}</dd></dl>
      {requestGap && <p role="status" className="console-notice">{requestGap}</p>}
      {request && <><p>Task: {request.spec.task?.type}</p><details><summary>Task input</summary><pre>{JSON.stringify(request.spec.task?.input ?? null, null, 2)}</pre></details>
        <p>Requested runtime: {request.spec.runtime?.profileRef} / {request.spec.runtime?.timeout}</p></>}
    </section>
    <section aria-labelledby="decision-title"><h2 id="decision-title">Policy decision</h2>
      <p className={`decision-badge ${state.decision.result === 'Deny' ? 'decision-deny' : ''}`}>{state.decision.result}</p>
      <p>{state.decision.reason}</p><dl><dt>Action</dt><dd>{state.decision.action}</dd>
        <dt>Policy</dt><dd>{state.decision.policyRef.id} / {state.decision.policyRef.version}</dd>
        <dt>Decision ID</dt><dd>{state.decision.id}</dd></dl>
    </section>
    <AuthorityComparison requested={request?.spec.requestedAccess} effective={state.effectiveAuthority}/>
    <section aria-labelledby="lifecycle-title"><h2 id="lifecycle-title">Recorded lifecycle</h2>
      {state.claim ? <><p>Current phase: <strong>{state.claim.phase}</strong></p><p className="note">Claim: {state.claim.id}</p></> : <p>No claim issued.</p>}
      {state.evidence.runtimeEvents?.length ? <ul>{state.evidence.runtimeEvents.map((event, index) => <li key={index}>{event.kind}</li>)}</ul> : <p>No runtime events recorded.</p>}
      <p className="note">No timestamps or intermediate lifecycle steps are supplied.</p>
    </section>
    <section aria-labelledby="backend-title"><h2 id="backend-title">Backend identity</h2>
      {state.claim?.backendIdentity ? <dl><dt>Backend</dt><dd>{state.claim.backendIdentity.backend}</dd><dt>Worker</dt><dd>{state.claim.backendIdentity.workerId}</dd></dl>
        : <p>{state.claim ? 'Backend identity not supplied.' : 'No backend allocation in this snapshot.'}</p>}
      <p className="note">Backend identity is recorded evidence, not proof of agent outcome.</p>
    </section>
    <section aria-labelledby="invocations-title"><h2 id="invocations-title">Governed invocations</h2>
      <p>Tool calls recorded: {state.evidence.toolInvocations?.length ?? 'Not supplied'}</p>
      <p>Model calls recorded: {state.evidence.modelInvocations?.length ?? 'Not supplied'}</p>
      <p className="console-notice">Per-call allow/deny decisions and results are not supplied by these v0 fixtures.</p>
    </section>
    <section aria-labelledby="outcome-title"><h2 id="outcome-title">Outcome & evidence gaps</h2>
      <p>Agent outcome not supplied.</p><p>Revocation evidence not supplied.</p>
      <p className="note">A Running phase is not a successful outcome. No terminal progression or revocation is inferred.</p>
    </section>
  </div>;
}

export function ConsolePanel({ source, route, keys }: { source: EvidenceSource; route: ConsoleRoute; keys: ConsoleReferenceKeys }) {
  const [loaded, setLoaded] = useState<{ route: ConsoleRoute; source: EvidenceSource; keys: ConsoleReferenceKeys; result: ConsoleResult }>();
  useEffect(() => {
    let active = true;
    loadConsole(source, route, keys).then(result => { if (active) setLoaded({ route, source, keys, result }); });
    return () => { active = false; };
  }, [source, route, keys]);
  return <div aria-live="polite" aria-busy={!loaded || loaded.route !== route || loaded.source !== source || loaded.keys !== keys}>
    {loaded && loaded.route === route && loaded.source === source && loaded.keys === keys ? <ConsoleEvidence result={loaded.result}/>
      : <section role="status"><h2>Loading evidence</h2><p>Waiting for this fixture source. No previous claim is displayed.</p></section>}
  </div>;
}

export function ClaimConsole({ source, route, keys }: { source: EvidenceSource; route: ConsoleRoute; keys: ConsoleReferenceKeys }) {
  return <><a className="skip-link" href="#console-main">Skip to claim evidence</a><main id="console-main" className="console" tabIndex={-1}>
    <header><p className="eyebrow">AGENOVA / SINGLE CLAIM / FIXTURE STAGE</p><h1>Claim Console</h1>
      <p>Inspect one assignment: intent, decision, authority and recorded execution.</p></header>
    <ConsoleNavigation/>
    <div className="console-source"><label htmlFor="console-scenario">Fixture presentation scenario</label>
      <select id="console-scenario" value={route.scenario} onChange={event => navigate(consoleHref(route.kind, route.reference, event.target.value as ConsoleRoute['scenario']))}>
        {scenarios.map(scenario => <option key={scenario} value={scenario}>{scenario}</option>)}
      </select>
      <p>Selected {route.kind === 'requests' ? 'request' : 'claim'}: <code>{route.reference}</code></p>
      {route.scenario === 'narrowed' ? <p className="console-notice">Derived fixture demonstration: Team A effective tools exclude github.pull-request. Revalidated by canonical Go; no executed policy is claimed.</p>
        : route.scenario === 'canonical' ? <p className="note">Canonical fixture evidence. No live connection.</p>
        : <p className="console-notice">Simulated fixture-source state: {route.scenario}. No live API behavior is claimed.</p>}
    </div>
    <ConsolePanel source={source} route={route} keys={keys}/>
    <footer>Fixture-driven portion only. Live evidence, polling and API/CLI equality remain blocked on #38/#68.</footer>
  </main></>;
}
