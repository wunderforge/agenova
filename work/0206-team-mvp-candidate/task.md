# Task: Reproducible team MVP candidate on kind

- Ticket: [#206](https://github.com/wunderforge/agenova/issues/206)
- Mission: Package and prove the E14 trusted-user governance path on a clean kind installation.
- Target: reference install/run guide, OIDC and Policy fixtures, connected CLI/Portal smoke, provider-credential fixture, and release evidence.
- User value: two teammates can independently run the same governed path and inspect consistent claim-scoped evidence.
- PRD outcome: [authorization](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [claim authority](../../docs/product/prd.md#4-claim-scoped-authority), [evidence](../../docs/product/prd.md#5-facts-and-accountability), and [contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path).

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, and this packet.

Additional task-specific context:

- [#206](https://github.com/wunderforge/agenova/issues/206), [E14 #176](https://github.com/wunderforge/agenova/issues/176), and predecessors [#197](https://github.com/wunderforge/agenova/issues/197), [#203](https://github.com/wunderforge/agenova/issues/203), [#204](https://github.com/wunderforge/agenova/issues/204), [#205](https://github.com/wunderforge/agenova/issues/205).
- [Feature specification](spec.md), [technical design](design.md), [authority and credentials](../../docs/product/architecture-contract.md#authority-and-credentials), [evidence surfaces](../../docs/product/architecture-contract.md#evidence-surfaces), and [demo playbook](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice).
- [kind/Ollama guide](../../docs/reference-cli-kind-ollama.md), `harness/local/macos-bootstrap.sh`, `deploy/reference`, `internal/identity`, `internal/console`, `internal/modelgateway`, and `internal/toolgateway`.

## Scope

In scope:

- Reproducible clean-kind OIDC/Policy/template installation, equivalent two-user Work, real Agent Sandbox/Ollama execution, connected CLI/API/Portal evidence, diagnostics, and bounded cleanup.
- Controlled token-requiring provider fixture proving host-side credential use and zero external calls on denial.
- Versioned `v0.x-team-mvp` checklist, redacted evidence, and independent teammate reproduction.

Out of scope:

- EKS/Bedrock proof, production HA/history, general Secret Manager, OPA/Cedar, Kubernetes Operator, delegated administration, and new public claim fields.

## Acceptance Criteria

- Two real company OIDC identities use one installed service and template with byte-equivalent requested access, producing distinct Policy-bounded authority without process identity swaps.
- Allowed real Agent Sandbox/Ollama Work succeeds; over-scope, cross-claim, and post-terminal attempts stop before external side effects.
- CLI/API/Portal agree on public principal, Policy version, requested/effective authority, reasons, invocations, and outcome.
- Worker and evidence expose no user/workload/provider secrets; a controlled provider fixture requires a host-held token and shows zero calls on denial.
- Two teammates reproduce from clean environments; reapply/rerun and cleanup are deterministic; the full gate passes.

## Negative Case

- Missing OIDC, Policy, worker identity, Gateway, runtime, or provider configuration fails at its own boundary without fixed-identity, memory-backend, broader-authority, or mock-success fallback.

## Execution Todo

- [x] Scout prepared M1–M4 branches, kind substrate, dependencies, and missing installation path.
- [x] Assignee self-review: #206, E14, PRD, architecture, and this packet agree on the local/kind boundary; the user's earlier approval waiver and current E14 request authorize preparation. Independent review remains a PR gate.
- [x] Package the kind/OIDC/Policy/template/CLI/Portal run and bounded cleanup; a fresh-cluster reproduction remains outstanding.
- [x] Add controlled host-token provider fixture and positive/negative call-count evidence.
- [ ] Capture real two-user identity and Ollama execution, CLI/API/Portal parity, and rendered Portal proof.
- [ ] Obtain a second teammate's independent clean-environment transcript.
- [x] Run focused gates, synthetic installed E2E, and `./scripts/check.ps1 -All` on the final candidate; company-identity E2E remains open above.
- [x] Review diff, evidence secret scan, release checklist, and residual risk.

## Quality Gates

- Focused Policy, OIDC, authority, worker identity, Gateway, installer, and provider-fixture tests.
- Clean kind/Ollama/OIDC installed smoke plus connected Portal browser test.
- `./scripts/check.ps1 -All`.

## Evidence Required

- Reproducible commands and reviewable outcomes for two clean-environment runs: versions and revisions, request/claim/invocation checks, rendered Portal behavior, adapter call counts, secret scan, scoped cleanup, and focused/full gate results. Generated output is not committed to Git.
- Unavailable company identities or second teammate are explicit blockers, never replaced by synthetic identities.

## Constraints

- Preserve `docs/product/architecture-contract.md` and backend-neutral public contracts.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Never commit company JWTs, signing keys, provider credentials, or raw identity claims.
- Controlled provider evidence is not Bedrock or EKS evidence.

## Decisions and Blockers

- 2026-10-19: M1 PR #207 remains open; M2–M4 are prepared stacked branches, so M5 cannot be accepted or released before predecessor review/merge.
- 2026-10-19: Local `agenova-k8s-lab` kind is available. Real company OIDC users and second-teammate reproduction are unverified and need external participants.
- 2026-10-19: Synthetic A/B installed smoke succeeded on one existing kind cluster, using byte-equivalent Work specs and distinct 5m/2m effective timeouts. This proves the integration path but does not satisfy the real-company-identity or clean-environment acceptance lines. The controlled provider test proves one authorized host-token call and zero denied calls; it is not an installed external-provider run.
- 2026-10-19: The M5 candidate passed `pwsh ./scripts/check.ps1 -All`, including Go tests, static checks, typecheck/build, and 52 browser smoke tests. The installed Portal test passed once per synthetic user. Generated screenshots, JSON records, transcripts, and gate logs were removed from the branch at the user's request; the temporary issuer, tokens, namespace, and image-tag changes were cleaned up.
