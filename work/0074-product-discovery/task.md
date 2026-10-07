# Task: Validate target users and real agent-governance pain

- Ticket: [#74](https://github.com/wunderforge/agenova/issues/74)
- Mission: Determine which real users and problems Agenova should serve, using current public evidence rather than confirmation of the existing vision.
- Target: A dated, repository-tracked product-discovery report and traceable source ledger.
- User value: The team can prioritize the MVP, positioning, communities, and follow-up validation from evidence.
- PRD outcome: [Target Users](../../docs/product/prd.md#target-users), [Required Outcomes](../../docs/product/prd.md#required-outcomes), and [Out of Scope](../../docs/product/prd.md#out-of-scope-unless-re-prioritized)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md)
- [AIDLC workflow](../../docs/development/AIDLC.md)
- [Issue #74](https://github.com/wunderforge/agenova/issues/74)

## Scope

In scope:

- Current official capabilities and boundaries of adjacent runtimes, sandboxes, gateways, agent platforms, and cloud offerings.
- First-person public evidence from GitHub, Reddit, Hacker News, public technical communities, and practitioner reports.
- Evidence-backed personas, jobs, pain, alternatives, niche selection, positioning, community targets, and MVP recommendations.
- The user's nested-isolation hypothesis: Agenova may govern a VM/container/host assignment whose internal agent runtime starts its own sandbox.
- Explicit counterevidence and cases where Agenova adds no useful value.

Out of scope:

- Private-community scraping, unsolicited outreach, market-size estimates, implementation, or silent PRD/architecture changes.
- Treating vendor marketing, search snippets, or unsupported LLM synthesis as user evidence.

## Acceptance Criteria

- At least 15 credible first-person problem signals from at least three independent communities or ecosystems are traceable to source, date, persona, job, workaround, pain/consequence, and possible fit.
- Official evidence accurately separates what existing platforms already solve from remaining governance gaps.
- The report identifies and ranks target personas/problems, including counterevidence and low-fit audiences.
- The current PRD is assessed as keep, clarify, deprioritize, validate next, or change; proposed changes require separate decision Tickets.
- At least five high-signal public projects/users/maintainers and relevant communities are identified without implying endorsement.
- Recommendations distinguish evidence, inference, hypothesis, confidence, and unanswered questions.

## Negative Case

- The report must be considered inconclusive—not favorable—when public evidence is too weak, indirect, vendor-driven, or already addressed by existing products.

## Execution Todo

- [x] Scout the current product contract, Ticket, risks, and adjacent solution categories.
- [x] Confirm this packet with the Owner before research; independent review remains required on the PR.
- [x] Map official capabilities and positioning of adjacent products and projects.
- [x] Collect and code first-person public pain and counterevidence.
- [x] Build a claim/evidence/gap matrix and run targeted follow-up searches.
- [x] Synthesize personas, niche choices, positioning, scope implications, and outreach targets.
- [x] Create the report and source ledger with descriptive citations.
- [x] Run documentation gates, link checks, and final claim-to-source audit.
- [x] Review the diff for scope, overclaiming, unsupported inference, and source-of-truth changes.

## Quality Gates

- Manual source audit: every material market/product claim has an appropriate public source and confidence label.
- Acceptance audit: at least 15 first-person signals, three ecosystems, five high-signal targets, and counterevidence.
- `.\scripts\check.ps1 -Docs`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Dated research report with direct executive answer, evidence matrix, alternatives boundary, persona/problem ranking, niche analysis, MVP recommendations, community targets, limitations, and source ledger.
- Search coverage and stopping rationale.
- Exact local gate output and passing PR CI.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Do not equate anecdotes with prevalence or infer purchase intent from technical complaints.
- Do not contact or identify private individuals beyond their voluntary public project/community identity.
- Prefer current primary/official evidence; label forum content as anecdotal discovery signals.

## Decisions and Blockers

- Owner authorization: the project lead explicitly requested execution of #74 on 2026-08-25 and asked for an honest, deep assessment including direction changes.
- Decision: deliver the durable artifact as repository Markdown and use the PR as the independent review gate.
- Blockers: none.

## Verification Evidence

- `2026-08-25`: `.\scripts\check.ps1 -Docs` passed, including local Markdown links and task-packet contracts.
- `2026-08-25`: `.\scripts\check.ps1 -All` passed, including formatting, module consistency, static analysis, Go tests, and Agent Sandbox integration compilation.
- Manual acceptance audit: 23 first-person signals across seven ecosystems, 21 counterevidence/capability entries, eight public validation targets, explicit low-fit users, and continue/stop criteria.
- Manual claim audit: material capability claims use primary documentation; community reports are labelled as anecdotal signals; adoption and willingness-to-pay remain explicitly unvalidated.
