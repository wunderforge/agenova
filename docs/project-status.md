# Implementation Evidence Snapshot

Updated: 2026-08-30

This file is the evidence-backed snapshot of what the merged repository currently proves. Product vision is not implementation status, and this file does not track ticket owners, readiness, sequence, or work-in-progress; those remain in GitHub Issues and the Delivery Project.

Update this snapshot only when merged behavior, accepted evidence, or a known implementation gap changes. See the [AIDLC source-of-truth map](development/AIDLC.md).

## Position Against the Target Design

| Target path | Current state | Evidence / limitation |
| --- | --- | --- |
| Reusable agent role | Partial | `AgentSandboxTemplate` models only image and command; the broader Agent Template contract is not defined. |
| Submit a declarative task request | Not implemented | The target `ClaimRequest` schema, YAML/API validation, and CLI submission path do not exist. |
| Authorize the requesting principal | Not implemented | There is no trusted `Principal` contract, reference `PolicyBundle`, action authorizer, or authorization-decision evidence. |
| Resolve requested access | Not implemented | Current gateway tests cover lifecycle and parent scope only; there is no template/policy/request intersection or persisted effective-authority snapshot. |
| Create one claim per run | Implemented in reference | `api/v1alpha1`, `internal/operator`, and lifecycle tests. |
| Bind a runtime backend | Implemented in reference; spike on Kubernetes | In-memory backend passes the shared contract. Agent Sandbox allocation and readiness were exercised on kind. |
| Enforce effective authority | Partial reference | Tool and Model Gateways require a `Running` claim and active parent scope, but do not yet enforce resolved tool, model, or resource authority. |
| Execute through gateways | Partial | Authorization methods exist; there is no network gateway or real upstream proxy path. |
| Record claim-scoped facts | Reference only | In-memory Tool, Model, and Runtime fact store; no durable storage or denial facts. |
| End authority with the claim | Implemented in reference | Terminal and child-out-of-scope negative tests pass. |
| Query complete evidence | Not implemented as a product surface | Tests can inspect facts; CLI/API/UI query output does not exist. |
| Use replaceable backends | Contract exists | Only the in-memory backend fully satisfies the contract; Agent Sandbox remains a spike with known gaps. |

## Implemented and Tested

- Claim phases and invalid-transition behavior.
- Sandbox replacement evidence kept separate from terminal claim outcome.
- In-memory `RuntimeBackend` reference implementation.
- Reusable backend contract test suite.
- Tool and Model Gateway allow/deny behavior based on active claim state.
- Parent/child lineage and child-out-of-scope denial.
- In-memory `RuntimeEvent`, `ToolInvocation`, and `ModelInvocation` storage and claim queries.
- In-memory engineer/orchestrator-style multi-agent reference scenario.
- Static check preventing known Agent Sandbox types from leaking outside its adapter package.
- Backend-neutral `agenova` composition root: `--help` and `version` work; invalid command/configuration exits non-zero; the process hosts the in-memory reference backend and accepts test doubles.

## Backend Spike

The Kubernetes Agent Sandbox adapter can create upstream templates, warm pools, and claims; observe allocation/readiness; and trigger cleanup. It was exercised against Agent Sandbox v0.4.6 on a local kind cluster.

It is not production-ready. Terminal outcomes are held in adapter memory, pool status is approximated, only a single-pool path was validated, and the returned claim loses its original spec. See [Agent Sandbox adapter](backends/agent-sandbox.md).

## Scaffolds or Missing Product Surfaces

- No usable `agenova run` command. The composition root exists; it does not submit ClaimRequest YAML.
- No `ClaimRequest` API type, YAML validator, or request-to-claim resolver.
- No trusted-principal, static-policy, control-plane action-authorization, or decision-evidence path.
- No running operator or controller.
- No HTTP/gRPC Tool or Model Gateway.
- No real tool/model provider proxy.
- No durable claim, policy, lineage, or fact store.
- No claim token or workload identity flow.
- No verified deny-by-default network path.
- No Memory Interface implementation.
- No OpenTelemetry integration.
- No CRD generation, Helm chart, release image, or install flow.
- No read-only evidence API or React claim console.

## Next Delivery Slice

The next slice should make the reference governance path usable before adding more platforms:

1. Define the smallest backend-neutral principal, reference-policy, authorization-decision, `ClaimRequest`, effective-authority, fact, and evidence-output schemas.
2. Authorize the trusted principal's action/project/template, then resolve requested access against template and policy limits before creating a system-managed claim.
3. Add `agenova run -f <claim-request.yaml>` as a client of that same schema for one example role.
4. Drive claim lifecycle, one allowed tool call, one allowed model call, and one denied request through the reference path.
5. Return a single claim evidence view containing lifecycle, effective authority, lineage, invocations, outcome, and backend identity.
6. Turn that path into a deterministic E2E test and quickstart.
7. Add the supported idempotent reference install and initial-policy bootstrap path for an existing test cluster.
8. Re-run the supported runtime portion through Agent Sandbox and either close or explicitly accept its durability gaps.
9. Add the minimal read-only evidence API and live claim console only after the shared evidence representation is stable.

## Immediate Contributor Opportunities

- `ClaimRequest` schema, fixtures, and validation tests.
- Trusted-principal, reference-policy, and Team A allow / Team B deny fixtures.
- Request-to-effective-authority resolver and negative tests.
- CLI `-f` golden path and smoke test.
- Denial facts and evidence query shape.
- Agent Sandbox restart/durability spike.
- Example engineer/reviewer agents.
- Claim graph/timeline UI using stable JSON fixtures.
- Quickstart and adapter guide.
