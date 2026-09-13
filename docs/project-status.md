# Implementation Evidence Snapshot

Updated: 2026-09-10

This file is the evidence-backed snapshot of what the merged repository currently proves. Product vision is not implementation status, and this file does not track ticket owners, readiness, sequence, or work-in-progress; those remain in GitHub Issues and the Delivery Project.

Update this snapshot only when merged behavior, accepted evidence, or a known implementation gap changes. See the [AIDLC source-of-truth map](development/AIDLC.md).

## Position Against the Target Design

| Target path | Current state | Evidence / limitation |
| --- | --- | --- |
| Reusable agent role | Implemented in v0 contract | The backend-neutral `AgentTemplate` schema and validation are merged; runtime registration remains future work. |
| Submit a declarative task request | Partial | The `ClaimRequest` YAML/API contract and validation are merged; CLI submission is not implemented. |
| Authorize the requesting principal | Implemented in reference contracts | Trusted-local `Principal`, versioned `PolicyBundle`, and deterministic action authorization are merged; external identity-provider integration is not implemented. |
| Resolve requested access | Implemented in reference contracts | Template, request, and policy limits resolve to an effective-authority snapshot with deterministic tests; the request-to-claim run service and gateway wiring remain open. |
| Create one claim per run | Implemented in v0 contract and reference | The system-managed `SandboxClaim`/issued-state contract, `internal/operator`, and lifecycle tests are merged; request-to-claim issuance remains open. |
| Bind a runtime backend | Implemented in reference; partial on Kubernetes | The reduced five-operation reference backend and reusable contract suite are merged. Agent Sandbox still reports unsupported operations and an unverified filesystem boundary explicitly. |
| Enforce effective authority | Partial reference | Tool and Model Gateways require a `Running` claim and retain experimental parent-scope checks, but do not yet enforce resolved tool, model, or resource authority. |
| Execute through gateways | Partial | Authorization methods exist; there is no network gateway or real upstream proxy path. |
| Record claim-scoped facts | Reference only | In-memory Tool, Model, and Runtime fact store; no durable storage or denial facts. |
| End authority with the claim | Implemented in reference | Terminal negative tests pass; experimental child-out-of-scope tests also remain. |
| Query complete evidence | Fixture UI foundation only | React renders canonical v0 fixtures through a typed evidence source; no live API or complete evidence query path. See [fixture storyboard](../ui/README.md). |
| Use replaceable backends | Contract exists | Only the in-memory backend fully satisfies the contract; Agent Sandbox remains a spike with known gaps. |

## Implemented and Tested

- Claim phases and invalid-transition behavior.
- Sandbox replacement evidence kept separate from terminal claim outcome.
- In-memory `RuntimeBackend` reference implementation.
- Reusable backend contract test suite.
- Trusted-local principal boundary, reference policy bundle, action authorization, and effective-authority resolution.
- Claim-scoped filesystem semantics in the reference backend, with simulated boundary cases and a separate local Git/Go compatibility fixture.
- Tool and Model Gateway allow/deny behavior based on active claim state.
- Experimental parent/child lineage and child-out-of-scope denial, retained outside the committed MVP.
- In-memory `RuntimeEvent`, `ToolInvocation`, and `ModelInvocation` storage and claim queries.
- In-memory multi-agent reference scenario, retained as experimental behavior outside the committed MVP.
- Static check preventing known Agent Sandbox types from leaking outside its adapter package.
- Backend-neutral `agenova` composition root: `--help` and `version` work; invalid command/configuration (including a missing `--backend` value) exits non-zero; the process hosts the in-memory reference backend and accepts test doubles. Command behavior and shared contracts stay provider-neutral; the composition edge may import a concrete adapter constructor.

## Backend Spike

The Kubernetes Agent Sandbox adapter can create upstream templates, warm pools, and claims; observe allocation/readiness; and trigger cleanup. It was exercised against Agent Sandbox v0.4.6 on a local kind cluster.

It is not production-ready. Terminal outcomes are held in adapter memory, pool status is approximated, only a single-pool path was validated, and the returned claim loses its original spec. See [Agent Sandbox adapter](backends/agent-sandbox.md).

## Scaffolds or Missing Product Surfaces

- No usable `agenova run` command. The composition root exists; it does not submit ClaimRequest YAML.
- No request-to-claim run service; the `ClaimRequest` type and validation are merged but are not yet wired to issuance.
- No external identity-provider adapter; the trusted-local principal source is reference-only.
- No running operator or controller.
- No HTTP/gRPC Tool or Model Gateway.
- No real tool/model provider proxy.
- No durable claim, policy, or fact store; experimental lineage is in memory only.
- No claim token or workload identity flow.
- No verified deny-by-default network path.
- No Memory Interface implementation.
- No OpenTelemetry integration.
- No CRD generation, Helm chart, release image, or install flow.
- No read-only evidence API or live React claim console. A fixture-backed React storyboard renders request intent, Allow/Running, pre-claim Deny and missing/unknown source diagnostics; contract, component and browser checks run in the shared baseline.

## Next Delivery Slice

The next slice should make the reference governance path usable before adding more platforms:

1. Wire the merged principal, policy, action-authorization, `ClaimRequest`, and effective-authority contracts into one request-to-claim run service.
2. Persist the authorization decision and resolved authority before creating a system-managed claim.
3. Add `agenova run -f <claim-request.yaml>` as a client of that same schema for one example role.
4. Drive claim lifecycle, one allowed tool call, one allowed model call, and one denied request through the reference path.
5. Return a single claim evidence view containing lifecycle, effective authority, invocations, outcome, and backend identity.
6. Turn that path into a deterministic E2E test and quickstart.
7. Add the supported idempotent reference install and initial-policy bootstrap path for an existing test cluster.
8. Re-run the supported runtime portion through Agent Sandbox and either close or explicitly accept its durability gaps.
9. Add the minimal read-only evidence API and live claim console only after the shared evidence representation is stable.

## Immediate Contributor Opportunities

- Request-to-claim run service using the merged contracts and Team A allow / Team B deny fixtures.
- Trusted invocation-context binding and gateway mismatch denial tests.
- CLI `-f` golden path and smoke test.
- Denial facts and evidence query shape.
- Agent Sandbox restart/durability spike.
- Example engineer Agent Artifact and bounded adversarial denial scenario.
- Single-claim timeline and authority UI using stable JSON fixtures.
- Quickstart and adapter guide.
