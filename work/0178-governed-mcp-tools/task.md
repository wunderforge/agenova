# Task: Real governed MCP tools through the installed service

- Ticket: [#178](https://github.com/wunderforge/agenova/issues/178)
- Mission: Execute a configured read-only MCP operation through the installed Tool Gateway and prove that unauthorized invocations never reach the MCP tool handler.
- Target: Platform backend/profile schema and resolution, adapterregistry tool-capability/lock/inspect/init support, bundled tool adapter, installed control plane, Tool Gateway/provider contract, controlled-worker tool catalog, shared facts/evidence and kind acceptance harness.
- User value: A Work receives a real external observation under its own temporary authority; CLI and Portal distinguish permission, invocation, result and Work outcome.
- PRD outcome: [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).
- Packet status: Independent packet review (Claude, 2026-09-26, F1–F5) incorporated; assignee self-review by Tom on 2026-09-26. Slice 1 authorized on 2026-09-26 against main `c56ba3a21a43ba550185e991cecede796af4f054`. D1 B is selected; D2 is required before Slice 2. Implementation and acceptance evidence are recorded per slice.

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, this packet.

- [Feature specification](spec.md) and [technical design / D1](design.md#d1-mcp-client-option-b-selected).
- Architecture contract: [backend neutrality](../../docs/product/architecture-contract.md#backend-neutrality), [authority and credentials](../../docs/product/architecture-contract.md#authority-and-credentials), [evidence](../../docs/product/architecture-contract.md#evidence-surfaces), [installed service](../../docs/product/architecture-contract.md#reference-installation-and-bootstrap).
- [Platform API](../../api/v1alpha1/platform.go), [resolution](../../internal/platform/resolve.go), [adapter registry](../../internal/adapterregistry/types.go), [bundled registration](../../internal/adapters/bundled/registry.go), [Kubernetes install adapter](../../internal/adapters/bundled/kubernetes.go).
- [Installed composition](../../cmd/agenova-control-plane/main.go), [installed tests](../../cmd/agenova-control-plane/main_test.go), [shared service](../../internal/console/service.go), [current synthetic adapter](../../internal/console/mock_tool.go).
- [Tool Gateway](../../internal/toolgateway/gateway.go), [Gateway tests](../../internal/toolgateway/gateway_test.go), [run-service authority](../../internal/app/run_service.go).
- [Worker protocol](../../internal/workerprotocol/protocol.go), [ReAct schema/parser/prompt](../../internal/workerprotocol/react.go), [worker](../../cmd/agenova-demo-worker/main.go), [runtime execution validation](../../internal/runtime/agentsandbox/execution.go), [model schema override](../../internal/modelprovider/adapter.go).
- [Fact journal](../../internal/facts/journal.go), [CLI evidence validation](../../internal/connectedclient/run.go), [installed Portal smoke](../../ui/installed/installed.spec.ts), [installed runbook](../../docs/reference-cli-kind-ollama.md).
- [Registry schema/capability validation](../../internal/adapterregistry/registry.go), [adapter lifecycle/init](../../internal/adapterregistry/lifecycle.go), [registry lifecycle tests](../../internal/adapterregistry/lifecycle_test.go) and [CLI adapter smoke](../../cmd/agenova/smoke_test.go).
- [#121](https://github.com/wunderforge/agenova/issues/121) owns Worker-to-Gateway caller authentication; E16 only reuses the existing correlation-only binding.
- [AIDLC](../../docs/development/AIDLC.md#from-existing-ticket-to-task-packet) and [core-contract playbook](../../docs/harness/playbooks.md#change-a-core-contract).
- [E14](https://github.com/wunderforge/agenova/issues/176) owns shared identity/credential contracts. [PR #189](https://github.com/wunderforge/agenova/pull/189) is reference material only, not a required base or accepted Provider contract.

## Scope

In scope:

- One real read-only operation over **Streamable HTTP**, backed by a version-pinned MCP Deployment and ClusterIP Service on an explicitly selected kind cluster, with server-side request/tool-call logs.
- `Platform.spec.services.toolBackends` plus `services.toolProfiles` mirroring model backends/profiles. Each profile binds one logical-operation/resource pair to one backend and MCP mapping; all adapter config leaves are flat strings with static schema paths. Include immutable resolved configuration, registry tool-capability/lock/inspect/init support and installed capability reporting; do not broaden generic schema kinds.
- A provider contract in `internal/toolbackend` (selected location); MCP implementation below that package or another adapter package. `internal/toolgateway` continues to enforce authority. No provider types owned by `console`; bundled adapters must not import `console`.
- A trusted logical-operation catalog intersected with each Work's effective tools and resource scopes, propagated consistently through schema, parser, prompt, runtime and installed service validation.
- Stable attempt/outcome correlation, bounded untrusted observations, CLI/API/Portal consistency, deterministic tests and one reproducible kind E2E evidence bundle.
- A later E14-coordinated token-required path is part of full Epic acceptance, though the first delivery may be credential-free.

Out of scope:

- General MCP server discovery, stdio or legacy HTTP+SSE transports, arbitrary model-selected URLs/methods, write tools, general plugin installation, memory, multi-agent orchestration, production SSO, Worker caller authentication (#121) and claims of network-enforced isolation.
- Reimplementing #189's GitHub PR/rollback demo, adopting its local identity presets, or routing acceptance through `cmd/agenova-console`.
- Existing runbook annotations: handled separately under [#190](https://github.com/wunderforge/agenova/issues/190).

## Acceptance Criteria

1. `adapters install/inspect/init` and lock round-trip support tool capability without changing scalar schema kinds. `platform validate/plan/apply/status` carries validated backend/profile tool configuration into **`cmd/agenova-control-plane`**. The installed service constructs the provider and reports its capability honestly; readiness is not proof of a successful tool call.
2. A registered template and allowed Work can use a configured logical operation other than the legacy `git.read` literal (reference proposal: `repo.read`). A real kind worker completes model -> tool -> observation -> model and the task's verifiable objective, then cleans up.
3. The six hard-coded areas in [the migration map](design.md#hard-coded-tool-migration-map) use the same configured catalog intersected with effective authority. Provider URL, MCP tool name and credentials never enter worker authority. Gateway checks remain authoritative on every attempt.
4. Every allowed downstream call follows successful decision and ProviderAttempt recording. `ProviderAttempt.Target == ProviderOutcome.Target`; a safe external result reference uses a separate optional field. The same invocation validates and renders through CLI/API/Portal.
5. Each of ordinary Deny, unauthorized resource, cross-claim targeting, terminal claim and pre-call recording failure has a separate test and **zero server-side tool calls**, verified from complete server logs. N3 is a test-only driver probe of the existing service/Gateway-side correlation guard (`service.go:288`), not proof of caller authentication; that belongs to #121. Deny-by-hidden-tool alone is insufficient evidence.
6. Timeout, oversized response, invalid configuration and unavailable server fail explicitly without mock fallback. Bounded successful MCP text is marked untrusted and visibly marked if truncated for the observation budget.
7. `httptest` protocol tests are deterministic; one real kind run captures image/source versions, manifests, Platform revision, Work/claim/invocation IDs, server logs, CLI/API output and rendered Portal evidence. No live result is claimed from the fake server.
8. Full Epic completion additionally requires the E14-coordinated token case: provider-side credential use, no external secret in Worker/request/evidence, and safe failure for invalid credentials.

## Negative Case

The complete case matrix is in [spec.md](spec.md#negative-cases). In particular, journal failure before a call must stop the call even when the decision is Allow. Failure to write a post-call outcome cannot undo an already executed call and must be reported as an evidence failure, never as zero external activity.

## Execution Todo

- [x] Inspect #178, PRD, architecture, installed entry point, hard-coded worker restrictions and #189 overlap.
- [x] Self-review packet coverage against Tom's ten constraints; distinguish planning, implemented behavior and acceptance evidence.
- [x] D1: Tom selected Option B on 2026-09-26. Implement the bounded client with Go 1.22 standard-library APIs inside E16; leave root `go.mod`/`go.sum`, builder baselines and CI toolchain unchanged.
- [ ] D2: before Slice 2, select the independently built SDK file-server module or a vetted existing server plus body-logging reverse proxy; record source/version, effort and log semantics. See design.md. D2 does not block Slice 1.
- [x] Slice 1: implement backend-neutral provider/catalog/result contracts, flat-string Tool Backend/Profile config, registry tool-capability/lock/inspect/init migration, resolved revision metadata and installed wiring. Deterministic provider doubles cover composition and pre-call rejection. The installed build explicitly rejects configured tool-bearing Work until Slice 2 supplies transport plus worker catalog propagation; it never falls back to mock. See [Slice 1 review packet](review-slice1.md) for exact gates and remaining work.
- [ ] Slice 2 (after D2): implement the selected Option B Streamable HTTP client, real pinned reference server fixture, operation/parameter mapping and all six catalog migrations. Preserve Gateway enforcement and wire stable attempt/outcome facts plus separate result references.
- [ ] Slice 2 / S2: prevent incomplete allowed invocations. `internal/console/tool_provider.go` currently checks Catalog.Validate, context and Running after ToolDecision Allow but before ProviderAttempt; an early return leaves stage 1, which `internal/connectedclient/run.go:604` rejects as an invalid completed Work. Validate parameters and available session/Running state in `service.go` before calling Gateway. If a check can still reject after Allow (including cancellation/termination races), record ProviderAttempt plus a Failed ProviderOutcome with the same Target before returning, provided the journal is writable. Preserve explicit evidence-failure handling when recording itself fails. Add an N2 regression with zero provider calls and a completed Work that passes the real connected-client/CLI evidence validator; also cover late context/Running rejection. Do not weaken the CLI validator.
- [ ] Slice 2 / S3: enforce configured `max-concurrent-calls` in `toolbackend.Set`, shared by profiles using the same backend. Acquire capacity before entering the provider, respect context cancellation while waiting, and release capacity on every completion/failure path. Add deterministic concurrent-call tests that assert the observed maximum, cancellation without a provider call and capacity release after errors. Slice 1 only validates the configuration value.
- [ ] Slice 3: complete deterministic negative/failure cases, CLI/API/Portal parity and the installed kind E2E bundle. Fix findings and rerun affected gates before broadening.
- [ ] Slice 4: coordinate the token-required path with E14 and attach independent no-secret evidence before closing #178.
- [ ] Run focused gates and `./scripts/check.ps1 -All` for each remaining slice; review diff and update only evidence-owning documentation for behavior actually proven. Slice 1 results are in the review packet.
- [ ] Prepare reviewable PR(s) with exact evidence and the remaining Epic work. Split child Tickets only when independently reviewable deliveries need their own closure; do not close #178 on a planning-only or credential-free PR.

## Quality Gates

Planning gate (run now): `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` and `git diff --check`.

Implementation gates (planned, not yet executed for E16):

```sh
go test ./api/v1alpha1 ./internal/platform ./internal/adapterregistry ./internal/adapters/bundled ./internal/cli ./cmd/agenova ./cmd/agenova-control-plane
go test ./internal/toolgateway ./internal/console ./internal/workerprotocol ./internal/runtime/agentsandbox ./cmd/agenova-demo-worker ./internal/modelprovider ./internal/facts ./internal/connectedclient
go test ./internal/toolbackend/...
pwsh -NoProfile -File ./scripts/check.ps1 -All
```

`internal/toolbackend` is a planned package, not an existing successful gate. Add an opt-in installed-MCP E2E runner with mandatory explicit context/namespace and exact CLI invocation documented in the implementation PR; it must not default to a current/production cluster. Retain the supported Node 24 gate. D1 retains Go 1.22: run the focused suite and full build with an actual Go 1.22 toolchain, not just the developer's newer Go installation. Slice 1 must prove tool AdapterLock round-trip and idempotent install, inspect static string schemas, init a valid backend+profile fragment, reject duplicate operation/resource routes and preserve model/runtime adapters. Any D2 SDK server is built/tested separately in its own module and image.

## Evidence Required

- Planning: self-review, primary SDK module requirements, selected D1 B and pending D2 alternatives in design.md; no runtime evidence fabricated.
- Implementation: exact commands/exit codes, baseline and tested SHA, Go/Node/client/server versions, pinned image digests, secret-free configuration and canonical revision.
- Positive run: two or more actual model turns, a genuine file read at the MCP server, matching invocation correlation, correct Work result and cleanup, CLI/API/Portal parity.
- Five separate zero-call cases with test-driver receipts and complete server logs covering before/after windows, pod UID/restarts and log-continuity checks. Missing logs are insufficient evidence.
- Failure cases: timeout, hard response limit, soft observation truncation, malformed reply, unavailability and post-call journal failure distinguished explicitly.
- Token path: credential reference only, provider/Pod inspection and sanitized evidence; no token values in artifacts.

## Constraints

- Preserve the architecture contract and claim-scoped authority; MCP transport/provider types remain adapter-internal.
- Keep the root `go 1.22` directive and dependency files unchanged under selected D1 B. Option A would require a maintainer-approved separate chore for go.mod, both `golang:1.22-alpine` Dockerfiles and CI. A fixture server may use the official SDK in an independent module without changing the root baseline.
- `cmd/agenova-control-plane` is the installed acceptance entry point. Shared `internal/console.Service` can remain a consumer/composition layer; `cmd/agenova-console` is not the acceptance route.
- No `console` imports from provider contracts or bundled adapters; no MCP URL/tool/credential accepted from Work or model output.
- Do not weaken evidence validation to accommodate mismatched targets or treat response truncation as a successful oversized-wire response.
- Initial server fixture is internal to the selected kind cluster, read-only and bounded. Environment setup/cleanup uses explicit ownership and context.

## Decisions and Blockers

- Fixed by Tom on 2026-09-26: installed-service route, Streamable HTTP, kind Deployment+Service, Tool Backend config/validation, dependency decision gate, equal attempt/outcome targets, six-area migration and the acceptance/test matrix.
- D1 is resolved as B. F1 adopts toolProfiles with flat string config and the necessary tool-capability lifecycle migration, not nested registry schemas. F3 bounds cross-claim evidence to correlation-only Gateway-side rejection; #121 owns caller authentication. F4 keeps any root Go upgrade outside E16 and subject to the maintainer's independent chore approval.
- D2 is pending before Slice 2: compare an independent-module SDK file server with a vetted existing server plus request-body logging proxy. Slice 1 remains unblocked by D2; no server image has yet been selected.
- Timeline verified on 2026-09-26: `wunderforge` assigned #178 to `TIAN-TOM` at `2026-09-21T09:28:05Z` (`GET /repos/wunderforge/agenova/issues/178/timeline`). This corrects the earlier attribution to Tom's account.
- #189 remains unmerged at `38ee951358e36f32f440fdea851a61c7ce53661d` at inspection. It does not change `cmd/agenova-control-plane`; its console-owned ToolProvider and mismatched Target behavior are not imported as a frozen contract.
- Real E2E and E14 token acceptance remain unexecuted. Image digest/server build selection must be recorded before an integration result is accepted.
- Planning validation on 2026-09-26 (before Slice 1): `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` passed; local links, scaffold-marker removal and whitespace were checked. Application code, `go.mod` and `go.sum` are unchanged. This is packet validation, not implementation evidence.

- Slice 1 delivery: provider-neutral contracts and installed composition are implemented with doubles, not live MCP. `Result.ResultRef` is separate from Target; shared evidence-field projection and all dynamic worker consumers remain Slice 2. The catalog is immutable and intersection-tested. Default Go 1.22.12 linking failed on this host with `missing LC_UUID`; the focused suite passed with the same compiler and external linking. Exact full-gate results are in [review-slice1.md](review-slice1.md).
