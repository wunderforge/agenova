# Implementation Evidence Snapshot

Updated: 2026-09-16

This file is the evidence-backed snapshot of what the merged repository currently proves. Product vision is not implementation status, and this file does not track ticket owners, readiness, sequence, or work-in-progress; those remain in GitHub Issues and the Delivery Project.

Update this snapshot only when merged behavior, accepted evidence, or a known implementation gap changes. See the [AIDLC source-of-truth map](development/AIDLC.md).

## Position Against the Target Design

| Target path | Current state | Evidence / limitation |
| --- | --- | --- |
| Reusable agent role | Implemented in v0 contract | The backend-neutral `AgentTemplate` schema and validation are merged; runtime registration remains future work. |
| Submit a declarative task request | Implemented in reference | `agenova run -f` parses the canonical YAML, uses the trusted local principal and reference policy, resolves authority, issues one claim, and runs it on the memory backend. |
| Authorize the requesting principal | Implemented in reference contracts | Trusted-local `Principal`, versioned `PolicyBundle`, and deterministic action authorization are merged; external identity-provider integration is not implemented. |
| Resolve requested access | Implemented in reference | Template, request, and policy limits resolve to an effective-authority snapshot that the CLI path consumes before issuance. |
| Create one claim per run | Implemented in reference | The allowed CLI path creates one system-managed claim and `RunService` owns its authoritative lifecycle and teardown evidence. |
| Bind a runtime backend | Implemented in reference; partial on Kubernetes | The reduced five-operation reference backend and reusable contract suite are merged. Agent Sandbox v0.4.6 translates Allocate, readiness Observe, identity, and confirmed Cleanup; ordinary Start/Terminate and filesystem evidence remain unsupported. See the bounded [#66 kind evidence](evidence/E8-S1/agent-sandbox-mapping/summary.md). |
| Enforce effective authority | Partial reference | In-process Tool and Model Gateways now read one application-owned claim/authority snapshot and exact-match the granted tool operation, resource scope, and logical model profile. Trusted workload context and network enforcement remain unimplemented. |
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
- Tool and Model Gateway allow/deny behavior based on active claim state and exact system-issued tool, resource, and logical model authority.
- Credential-bearing fields are structurally excluded from ClaimRequest task input, resolved runtime launch input, and gateway parameters by one exact-name rule. The reference Agent Sandbox template renders no environment or Secret injection surface; provider configuration remains adapter-private. This does not inspect arbitrary task prose or source files and is not a network anti-bypass guarantee.
- Experimental parent/child lineage and child-out-of-scope denial, retained outside the committed MVP.
- In-memory `RuntimeEvent`, `ToolInvocation`, and `ModelInvocation` storage and claim queries.
- In-memory multi-agent reference scenario, retained as experimental behavior outside the committed MVP.
- Static check preventing known Agent Sandbox types from leaking outside its adapter package.
- Backend-neutral `agenova` composition root: `--help`, `version`, and `run -f` work; invalid command/configuration (including a missing `--backend` value) exits non-zero; the process hosts the in-memory reference backend and accepts test doubles. Command behavior and shared contracts stay provider-neutral; the composition edge may import a concrete adapter constructor.
- `agenova run -f` submits the canonical payment-timeout ClaimRequest through admission, authority resolution, issuance, and `RunService`. Team A reaches a terminal reference-backend outcome; Team B and invalid inputs stop before allocation. Authority flags are rejected.
- `examples/adversarial` denial demo (fixture mode): an unauthorized Team B principal composed out-of-band is evaluated through the merged authorization path against the reference default-deny bundle. It prints `[DENIED]` with real `Decision` evidence (principal, policy ID/version, decision ID, reason) before any claim is created and exits non-zero; malformed input exits non-zero without panic. Focused tests assert the claim store stays empty on denial.
- The bundled adapter catalog explicitly registers Kubernetes deployment, Agent Sandbox runtime, and OpenAI-compatible model implementations by qualified ID/version, protocol, capability, schema, descriptor, and capability-owned factory. `agenova adapters catalog|list|inspect|install|init` shares one programmatic lifecycle with Platform resolution; identical activation is idempotent and initialization emits secret-free Platform fragments.

## Backend Spike

The Kubernetes Agent Sandbox adapter can create upstream templates, warm pools, and claims; translate allocation/identity/readiness; and confirm cleanup. A bounded Agent Sandbox v0.4.6 kind run verified that path and demonstrated that a Ready worker still returns `ErrUnsupported` for ordinary Start and Terminate. Readiness remains Bound-level evidence, not worker start.

It is not production-ready. Allocation/release correlation is process-local, pool status is approximated, only a single-pool path was validated, and the legacy claim view loses its original spec. The adapter returns `FilesystemEvidenceUnsupported`; worker-visible directories, mount/credential exposure, outside-boundary enforcement, and general isolation are not verified. See the [frozen adapter mapping](backends/agent-sandbox.md) and [#89 filesystem handoff](../work/0089-filesystem-boundary/handoff-0048-0051.md). The opt-in controlled-worker experiment and full filesystem proof remain separate #51 work.

## Scaffolds or Missing Product Surfaces

- No external identity-provider adapter; the trusted-local principal source is reference-only.
- No running operator or controller.
- No HTTP/gRPC Tool or Model Gateway.
- No real tool/model provider proxy.
- No durable claim, policy, or fact store; experimental lineage is in memory only.
- No claim token or workload identity flow.
- No verified deny-by-default network path.
- No Memory Interface implementation.
- No OpenTelemetry integration.
- No CRD generation, Helm chart, release image, or Platform apply/install flow. Adapter catalog and local activation state exist, but do not reconcile infrastructure.
- No read-only evidence API or live React claim console. A fixture-backed React storyboard renders request intent, Allow/Running, pre-claim Deny and missing/unknown source diagnostics; contract, component and browser checks run in the shared baseline.

## Next Delivery Slice

The next slice should make the reference governance path usable before adding more platforms:

1. Compose the merged application run service with the controlled Agent Sandbox adapter on kind.
2. Assemble one stable evidence view from the authorization, authority, lifecycle, invocation, outcome, and backend facts.
3. Expose that same view to the read-only React console without creating a UI-only governance model.
4. Compose the proven in-process allowed/denied Tool and Model Gateway calls into the real-backend golden path.
5. Return a single claim evidence view containing lifecycle, effective authority, invocations, outcome, and backend identity.
6. Turn that path into a deterministic E2E test and quickstart.
7. Add the supported idempotent reference install and initial-policy bootstrap path for an existing test cluster.
8. Re-run the supported runtime portion through Agent Sandbox and either close or explicitly accept its durability gaps.
9. Add the minimal read-only evidence API and live claim console only after the shared evidence representation is stable.

## Immediate Contributor Opportunities

- Request-to-claim run service using the merged contracts and Team A allow / Team B deny fixtures.
- Trusted invocation-context binding and gateway mismatch denial tests.
- CLI `-f` issuance once the run service exists.
- Denial facts and evidence query shape.
- Agent Sandbox restart/durability spike.
- Example engineer Agent Artifact (the bounded adversarial denial scenario is delivered in `examples/adversarial`).
- Single-claim timeline and authority UI using stable JSON fixtures.
- Quickstart and adapter guide.
