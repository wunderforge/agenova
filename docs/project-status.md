# Current Project Status

Updated: 2026-08-13

This file is the source of truth for what the repository currently proves. Product vision is not implementation status.

## Position Against the Target Design

| Target path | Current state | Evidence / limitation |
| --- | --- | --- |
| Reusable agent role | Partial | `AgentSandboxTemplate` models only image and command; the broader Agent Template contract is not defined. |
| Submit a declarative task request | Not implemented | The target `ClaimRequest` schema, YAML/API validation, and CLI submission path do not exist. |
| Resolve requested access | Not implemented | Current gateway tests use in-process allowed sets; there is no template/policy/request intersection or persisted effective-authority snapshot. |
| Create one claim per run | Implemented in reference | `api/v1alpha1`, `internal/operator`, and lifecycle tests. |
| Bind a runtime backend | Implemented in reference; spike on Kubernetes | In-memory backend passes the shared contract. Agent Sandbox allocation and readiness were exercised on kind. |
| Enforce effective authority | Reference only | Tool and Model Gateways require a `Running` claim and active parent scope. Allowed sets are currently supplied directly and enforced in-process. |
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

## Backend Spike

The Kubernetes Agent Sandbox adapter can create upstream templates, warm pools, and claims; observe allocation/readiness; and trigger cleanup. It was exercised against Agent Sandbox v0.4.6 on a local kind cluster.

It is not production-ready. Terminal outcomes are held in adapter memory, pool status is approximated, only a single-pool path was validated, and the returned claim loses its original spec. See [Agent Sandbox adapter](backends/agent-sandbox.md).

## Scaffolds or Missing Product Surfaces

- No usable `agenova run` command.
- No `ClaimRequest` API type, YAML validator, or request-to-claim resolver.
- No running operator or controller.
- No HTTP/gRPC Tool or Model Gateway.
- No real tool/model provider proxy.
- No durable claim, policy, lineage, or fact store.
- No claim token or workload identity flow.
- No verified deny-by-default network path.
- No Memory Interface implementation.
- No OpenTelemetry integration.
- No CRD generation, Helm chart, release image, or install flow.
- No React demo UI.

## Next Delivery Slice

The next slice should make the reference governance path usable before adding more platforms:

1. Define the smallest backend-neutral `ClaimRequest`, effective-authority, fact, and evidence-output schemas.
2. Implement validation and a resolver that intersects requested access with template and policy limits before creating a system-managed claim.
3. Add `agenova run -f <claim-request.yaml>` as a client of that same schema for one example role.
4. Drive claim lifecycle, one allowed tool call, one allowed model call, and one denied request through the reference path.
5. Return a single claim evidence view containing lifecycle, effective authority, lineage, invocations, outcome, and backend identity.
6. Turn that path into a deterministic E2E test and quickstart.
7. Re-run the supported runtime portion through Agent Sandbox and either close or explicitly accept its durability gaps.
8. Add a small UI only after the evidence schema and CLI flow are stable.

## Immediate Contributor Opportunities

- `ClaimRequest` schema, fixtures, and validation tests.
- Request-to-effective-authority resolver and negative tests.
- CLI `-f` golden path and smoke test.
- Denial facts and evidence query shape.
- Agent Sandbox restart/durability spike.
- Example engineer/reviewer agents.
- Claim graph/timeline UI using stable JSON fixtures.
- Quickstart and adapter guide.
