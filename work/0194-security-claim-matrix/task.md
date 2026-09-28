# Task: Security claim matrix and threat model

- Ticket: [#194](https://github.com/wunderforge/agenova/issues/194)
- Mission: Map every security-relevant architecture-contract sentence to its configuration, verification method, independent oracle, and owning ticket, so E15 has a checkable definition of done.
- Target: `work/0194-security-claim-matrix/spec.md` (threat model + matrix). `docs/project-status.md` only if merged evidence or known gaps change.
- User value: Reviewers and demo presenters can tell, for each security promise, what is proven, what is unverified, and which ticket will provide evidence, without over-claiming from unit tests.
- PRD outcome: [4. Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [5. Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and the out-of-scope rule on hostile-agent isolation claims.

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md): the threat model and claim matrix (the deliverable)
- [`docs/product/architecture-contract.md`](../../docs/product/architecture-contract.md): Submission and Resolution, Authority and Credentials, Facts and Future Lineage, Evidence Surfaces, Reference Installation and Bootstrap
- [`docs/project-status.md`](../../docs/project-status.md): Limits and Unverified Claims
- [`docs/development/AIDLC.md`](../../docs/development/AIDLC.md): Sources of Truth (the matrix must not duplicate GitHub state)
- Code cited by the matrix: `cmd/agenova-control-plane/main.go`, `internal/console/http.go`, `internal/console/service.go`
- Evidence cited by the matrix: [`docs/evidence/165/`](../../docs/evidence/165/), [`docs/evidence/167/`](../../docs/evidence/167/), [`docs/evidence/51/`](../../docs/evidence/51/)

## Scope

In scope:

- Threat model: attackers, assets, trust boundaries, attack paths, and non-goals.
- One matrix row per security-relevant contract sentence, or an exclusion with a reason.
- Per row: verbatim quote, configuration (K/E), one-time baseline at `c56ba3a`, verification with positive and negative controls and an independent oracle, owning ticket, and demo relevance.
- Observed reference facts recorded as unverified leads.

Out of scope:

- Running attacks or collecting runtime evidence (#52, later E15 tickets).
- Fixes, NetworkPolicy changes, or CNI changes.
- New authoritative security documents or per-row mutable status.
- Parent/child claim inheritance, hostile-agent isolation certification, broad scanning.

## Acceptance Criteria

- Every security-relevant sentence in Authority and Credentials, Submission and Resolution, Evidence Surfaces, and Reference Installation and Bootstrap has a row, or an exclusion with a reason.
- Every row names the configuration it applies to; no kind result is extended to EKS.
- Every demo-relevant row names a positive control, a negative control, and an oracle outside Agenova's own evidence.
- Rows are marked demo-shaping (`D`) or full-delivery-only (`F`), pending maintainer confirmation on #177.
- Every row links to an owning ticket or names a planned E15 ticket.

## Negative Case

- A claim supported only by unit tests, static manifests, or in-memory behaviour is classified as unverified at runtime, never proven.
- The fixed reference identity (reaching the API means Team A) is recorded as a current gap co-owned with E14, not as an identity control.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [x] Write the threat model and matrix in `spec.md`.
- [x] Cross-check every quoted sentence against the contract and every cited test/code line against `c56ba3a`.
- [x] Run `./scripts/check.ps1 -Docs` and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.
- [ ] After merge: ask the maintainer on #177 to confirm the `D` rows; link planned E15 tickets into their rows when created.

## Quality Gates

- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

- The merged `spec.md` with the matrix; every row links to its owning ticket.
- A maintainer comment on #177 confirming which rows the final demo narration depends on.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Quote contract sentences verbatim; do not paraphrase them into new claims.
- Do not maintain mutable per-row status; the baseline column is a one-time classification.
- Test harnesses proposed by the matrix must not weaken `AGENOVA_ALLOWED_WORKER_IMAGE`.

## Decisions and Blockers

- Packet language is English, matching every existing packet; the GitHub Ticket body stays Chinese.
- The matrix lives in the task packet spec, the AIDLC-owned place for accepted feature behaviour, instead of a new `docs/` file.
- Facts rows (FL-1, FL-2) are included beyond the four named sections because evidence integrity is a named asset in the Ticket.
- Planned E15 owners (E15-T2, E15-T3, E15-ID, E15-EKS) are named but not yet created. Creating them is tracked on #177, not by this packet.
- `docs/project-status.md` is unchanged: this packet adds no merged runtime evidence, and its existing limits already cover the gaps recorded here (fixed principal, unproven network isolation, process-local evidence).
- macOS-only baseline failure `ui/smoke/console.spec.ts:74` in `-All` is pre-existing (CI on Ubuntu passes; seen on #193) and is reported in the PR, not fixed here.
