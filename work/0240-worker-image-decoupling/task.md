# Task: Independent Agent images and automatic platform delivery

- Ticket: [#240](https://github.com/wunderforge/agenova/issues/240)
- Mission: Separate template-selected Agent artifacts from platform delivery while retaining the controlled-v1 execution boundary.
- Target: registration, control-plane composition, Agent Sandbox adapter, evidence diagnostics, release scripts/workflow and operator documentation.
- User value: Publish a compatible Agent image without restarting or republishing the platform; incompatible images fail explicitly.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution), [reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Behavior specification](spec.md), [architecture contract](../../docs/product/architecture-contract.md).
- [Registration](../../internal/app/registration.go), [member governance](../../cmd/agenova-control-plane/governance.go), [composition](../../cmd/agenova-control-plane/main.go).
- [Runtime control](../../internal/runtime/agentsandbox/controlled.go), [stdio execution](../../internal/runtime/agentsandbox/execution.go), [runtime configuration](../../internal/adapters/bundled/registry.go).
- [Release transport](../../scripts/release/release.mjs), [release workflow](../../.github/workflows/shared-eks-release.yml), [current guide](../../docs/development/shared-eks-release.md).
- [AIDLC](../../docs/development/AIDLC.md), [playbooks](../../docs/harness/playbooks.md).

## Scope

In scope:

- Structurally valid independently selected template images and commands, legacy configuration, bounded failure diagnostics, independent reference Worker publication and automatic default main delivery.

Out of scope:

- Generic arbitrary-program execution, new worker protocol, image provenance verification, live release toggles, cloud resources, merging and deployment.

## Acceptance Criteria

- Two compatible images register and execute under one unchanged control plane; preserve immutable registration and timeout/authority checks.
- Incompatible images register but fail with safe bounded protocol/start diagnostics; cleanup is confirmed or explicitly unknown.
- Unavailable images produce actionable pull/readiness evidence without provider dispatch.
- Legacy image settings remain parseable but never restrict templates; omitted image settings work.
- main publishes/deploys by default with emergency off switches; Worker publication never rewrites templates or restarts the platform solely for a Worker artifact.
- PR/docs-only events never access cloud resources; rollout refuses active Work and retains guarded rollback.

## Negative Case

- Malformed templates, unknown protocol, excessive timeout, invalid worker acknowledgements, cross-claim workload tokens and terminal authority remain rejected. No mock success or failed-auth fallback.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Self-review the accepted scope, negative cases and PRD; record the Owner's image-decoupling and CI/CD decision.
- [x] Remove image checks while preserving legacy configuration and execution contracts.
- [x] Add safe runtime diagnostics and regression tests.
- [x] Decouple release artifacts and enable default automatic main delivery.
- [x] Document compatibility, authoring and migration.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/app ./internal/adapters/bundled ./internal/runtime/agentsandbox ./internal/evidence ./internal/console ./cmd/agenova-control-plane`
- `node --test scripts/release/*.test.mjs`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Distinct image selection/isolation, negative protocol/readiness/cleanup, unchanged authority and release behavior. Real local backend execution when available; otherwise state the blocker rather than claiming backend proof.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- No AWS resources, production changes, live release switches, merge or deployment. Do not read secret-bearing files.
- Never infer successful execution from readiness or an image's self-declared protocol.

## Decisions and Blockers

- Owner explicitly approved independent images under the current execution contract, then added automatic CI/CD in this PR on 20 October 2026. Generic arbitrary-application adaptation remains excluded.
- Compatibility is an execution contract, not an image allowlist. An incompatible application may fail; authorization must not fail open.
- Existing repository `SHARED_EKS_RELEASE_ENABLED=false` would prevent a default-only change from activating. The Owner's automatic-delivery request is implemented with new opt-out `*_DISABLED=true` switches; obsolete opt-in variables no longer gate delivery. No live variables are changed in this PR.
- Windows full gate reaches one pre-existing `TestOwnerBootstrapDeliveryGuards` failure (directory fsync); do not weaken this test. Linux CI remains required. The changed adapter-init expectation was updated for the now-optional image hint.
- [Local acceptance and reproduction](acceptance.md). No AWS changes, live switch updates, merge or deployment were performed.
