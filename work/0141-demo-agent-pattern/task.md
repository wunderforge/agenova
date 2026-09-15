# Task: Document the demo-agent pattern for building future agent demos

- Ticket: [#141](https://github.com/wunderforge/agenova/issues/141)
- Mission: Extract the reusable pattern behind Epic 9 — how a demo agent proves and
  teaches the Agenova governance contract — into one guide so future agent demos are
  planned and built the same way without re-deriving the approach.
- Target: `docs/harness/demo-agent-pattern.md` (new file; docs only)
- User value: A contributor or presenter can read one document to understand what an
  agent demo is, which layers to reuse, how to plan the delivery, and exactly how to
  run the demo for an audience.
- PRD outcome: [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [E9 epic](https://github.com/wunderforge/agenova/issues/13) and the E9 task packets
  under `work/0115`–`work/0123` — source of the layered build and planning pattern
- [Merged engineer artifact](../../examples/engineer/) (#53/#138) — source of the
  verified implementation conventions (injectable `run`, `ToolClient` mock seam,
  fixture-mode label, test matrix)
- [Contract fixture inventory](../../harness/fixtures/contract/v0/inventory_test.go)
  — source of the fixture manifest rules summarized in the guide
- [Local governance setup guide](../../docs/harness/local-governance-setup.md) —
  phased path the demo-day script aligns with

## Scope

In scope:

- `docs/harness/demo-agent-pattern.md`: plain-language explanation (guard/actor
  framing), the six-chapter demo narrative, the layered build table with
  dependencies, the planning and risk pattern, implementation rules verified against
  merged code, the audience-facing demo-day script, and a checklist.

Out of scope:

- Any code, fixture, or packet changes for the open E9 tickets (#115–#119,
  #121–#123); the demo-day commands describe artifacts those tickets will deliver.
- Changes to `examples/`, `internal/`, `api/v1alpha1/`, or existing docs.

## Acceptance Criteria

- The guide exists at `docs/harness/demo-agent-pattern.md` and covers all sections
  named in scope.
- Implementation rules match the merged `examples/engineer` reference and AGENTS.md
  (single access seam, mode labels, non-zero exit on invalid input, evidence
  conventions).
- The layered build table matches the E9 ticket dependency graph recorded in the
  `work/0115`–`work/0123` packets.

## Negative Case

- The guide must not describe fixture-mode paths as live enforced governance
  (correlation ID vs. verified claim credential per the #121 Owner decision) and
  must not contradict the committed MVP scope (#110 — no parent/child lineage in
  demos; multi-claim demos use independent claims).

## Execution Todo

- [x] Scout the E9 packets, the merged engineer artifact, and the fixture inventory
      rules.
- [x] Write the guide with the narrative, layer table, planning pattern,
      implementation rules, and demo-day script.
- [ ] Run `./scripts/check.ps1 -All` (CI; no local PowerShell available).
- [ ] Review the diff for scope and source-of-truth consistency.

## Quality Gates

- `./scripts/check.ps1 -Docs`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing repository baseline (docs checks included) on the PR.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Docs only: no behavior, fixture, or contract changes.

## Decisions and Blockers

- Decision: single-file guide in `docs/harness/` (alongside the other harness
  guides) rather than `examples/README.md`, per Owner direction during drafting —
  one document others follow to build the next agent demo.
- Decision: the demo-day script intentionally documents the *finished* E9 show; it
  doubles as the acceptance target for the open E9 tickets and is updated if those
  tickets change command surfaces.
- Blockers: none.
