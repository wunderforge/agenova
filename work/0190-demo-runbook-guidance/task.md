# Task: Preserve demo clusters and route to the installed runbook

- Ticket: [#190](https://github.com/wunderforge/agenova/issues/190)
- Mission: Prevent accidental removal of a rehearsal cluster and identify the current installed-service guide.
- Target: `docs/reference-cli-kind-ollama.md` and `work/0143-midterm-vertical/demo.md`.
- User value: Contributors keep the cluster available and avoid treating the old standalone demo as the current installed path.
- PRD outcome: [demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path).

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, this packet.

- [Current runbook](../../docs/reference-cli-kind-ollama.md).
- [Historical checkpoint](../0143-midterm-vertical/demo.md).
- [Substrate harness](../../harness/spike/agent-sandbox-substrate/reproduce.sh): command dispatch and ownership validation.
- [Installed service](../../cmd/agenova-control-plane/main.go): fixed reference identity and installed composition.

## Scope

Add the two existing local documentation notes in a separate PR. No script, runtime, E16 feature or demo-acceptance changes.

## Acceptance Criteria

- Explain explicit `status` / `up` use and that `all`, including the default invocation, tears down the owned cluster.
- Preserve ownership safeguards; do not suggest adopting a cluster by copying receipts.
- Preserve the runbook's separate recovery path: inspect a stale or unmatched cluster and manually remove it only after confirming it is disposable, then recreate it with `up`.
- Keep the current runbook's new instructions in English; retain the historical checkpoint's Chinese text.
- Label #143 as historical and route current CLI/Connected Portal usage to the installed guide.
- State that its separate Team B service is not same-installation multi-user proof.

## Negative Case

A reader must not be told to omit the harness subcommand when retaining a cluster, bypass ownership checks, or use the old console as the installed Platform.

## Execution Todo

- [x] Inspect the original annotations, harness dispatch/ownership checks and installed composition.
- [x] Self-review scope, acceptance and negative case against #190 and the PRD.
- [x] Add the two notes without runtime changes.
- [x] Run documentation and repository gates; record the unrelated browser-smoke failure below.
- [x] Review the diff for documentation-only scope.
- [x] Validate the saved PR body and prepare the separate documentation PR requested by Tom.

## Quality Gates

- `git diff --check`
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All` (Node 24 on PATH)
- `pwsh -NoProfile -File ./scripts/check-pr-body.ps1 -BodyPath <saved-pr-body>`

## Evidence Required

Source inspection of `all` dispatch and owned-cluster checks, resolved links, exact gate outcomes and final PR diff. No cluster mutation is needed for this prose correction.

## Constraints

- Preserve the architecture contract, installed-service boundary and existing ownership logic.
- Keep this PR separate from E16 and do not close #107.

## Decisions and Blockers

- Tom explicitly requested the separate docs PR on 2026-09-26.
- #190 exists to satisfy the repository's linked-Ticket/task-packet PR contract for this bounded correction.
- Validation results will be recorded in the PR; no runtime behavior is claimed changed.
- Documentation checks and `git diff --check` passed on 2026-09-26. The full gate passed Go, frontend contracts/types/components/build and 51/52 browser checks; the unchanged keyboard scenario failed at `ui/smoke/console.spec.ts:99` while waiting for the `Loading evidence` heading. The run used supported Node 24. No application or test source is changed by this PR.
- A focused rerun in the freshly built docs worktree reproduced the same line-99 failure; `git diff --exit-code HEAD -- ui` confirmed that its UI sources match main `c56ba3a`. The saved PR body passed `scripts/check-pr-body.ps1`.
- PR #191's first CI run passed on commit `1ca292a`. Its two P2 review findings are addressed by the explicit, linked manual-recovery exception and the English retention block. The follow-up is documentation only; no cluster was removed.
