# Task: Application-side integration candidate for CI/CD validation

- Ticket: [E20 #218](https://github.com/wunderforge/agenova/issues/218); Owner authorizes local integration on 19 October 2026.
- Mission: Produce one tested Control Plane/Worker candidate combining member access, governed MCP tools and existing diagnosis fixes for the Owner's CI/CD trial.
- Target: installed composition, console, Gateway, Worker catalog, evidence decoder/UI and build handoff.
- User value: CI/CD tests one compatible application revision rather than unrelated feature images.
- PRD outcome: [claim-scoped authority and evidence](../../docs/product/prd.md), including member access.

## Context to Read

- AGENTS.md; docs/product/prd.md; docs/product/architecture-contract.md; docs/development/AIDLC.md.
- [Spec](spec.md), [design](design.md).
- work/0178-governed-mcp-tools/task.md, spec.md and design.md; work/0215-invited-team-access/task.md; work/0180-diagnosis-fixes/task.md.
- cmd/agenova-control-plane; internal/console/service.go and durable.go; internal/toolgateway; internal/connectedclient; internal/workstore; ui.

## Scope

Integrate application changes from pinned PR #216, #192 and #217. Preserve authentication, scoped registry, durable facts and role checks. Repair integration defects, test and document the release contract. No external-service deployment, AWS/Kubernetes mutation, image push, GitHub PR merge or public comments. Do not implement teammates' Collector/OTEL or business tools. Task-conditional Policy integration awaits the Owner's scope choice; no new authority semantics by default.

## Acceptance Criteria

1. Local commits identify upstream SHAs; native CLI/CP, Linux CP/Worker and UI build from that source.
2. Member preparation uses project-scoped Policy/template and Tool catalog; no fixed Team A fallback or external credential in Worker configuration.
3. Allowed/rejected calls use the same Gateway/evidence; focused, native Windows and repository gates pass or failures are reported without readiness claims.
4. Handoff specifies runtime configuration, RBAC, image bindings and Owner-managed endpoints. No real credential or AWS state in Git.
5. Public-cloud/provider acceptance remains a later CI/CD gate. Existing private credential finding remains open until independently resolved, not relabelled by passing ordinary tests.

## Negative Cases

Invalid/unavailable configured Tool routes fail explicitly without mock fallback. Unauthorized calls dispatch no provider. Missing tokens fail closed. Evidence failure never becomes success. Member isolation, revocation and durable history must survive the older Tool changes. Synthetic auth tests are not real Google login.

## Execution

- [x] Read contracts, pin sources and self-review scope/negative gates.
- [x] Integrate Tool with member/durable paths, then diagnosis.
- [x] Add combined-path regressions and scoped portability fixes.
- [x] Run focused, disposable TLS SQL, UI/browser and full gates; build native/Linux artifacts and actual local Docker images.
- [x] Record [evidence](evidence.md), [external-service contract](../../deploy/production/cicd-candidate.md) and residual gates; keep commits local without push/merge.

## Quality Gates / Evidence

`go test ./...`; focused/race tests where available; separate MCP fixture-module tests; disposable TLS PostgreSQL member tests; `scripts/check.ps1 -All`; repository Playwright and Linux builds. Exact outcomes belong in this packet. No cloud result inferred from fixtures/builds.

## Decisions and Risks

Base #216 `2ad6daef564f252a909cdc873761952ce426bf00`; Tool #192 `127d793853b79b69d5fd61dda88dd570fb2d8a47`; diagnosis #217 `296d2d0d253cede029f0d1adf71fa9a471ff1eb9`. Do not import abandoned lab IaC/private files. Local Git integration is authorized; upstream PRs/main remain untouched. Windows evidence checkout needs per-command `core.longpaths=true`; a failed initial checkout is recoverably stashed, not deleted.
