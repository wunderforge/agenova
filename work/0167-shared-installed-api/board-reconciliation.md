# Ticket reconciliation after #165/#167 merge

This is an execution checklist, not a claim that GitHub issues were changed.
Recheck the merged main revision and each issue's current owner/activity before
editing the public board. Preserve unmet acceptance criteria rather than
closing by thematic overlap.

| Issue | Action after merge | Reason |
| --- | --- | --- |
| #165, #167 | Link merged PRs and exact kind/Ollama/Playwright evidence; close only after main contains both. | These are the narrow delivered integration slices. |
| #68 | Check claim-ID and request-ID lookup, denial, malformed/not-found and redaction tests, then close if still satisfied. | The installed handler exposes both `/api/claims/.../evidence` and `/api/requests/.../evidence`; #167 adds bounded current-session listing but not durability. |
| #60 | Check the actual installed browser and CLI equality evidence, running/terminal/denial states, then close if all acceptance is met. | The existing fixture console and the connected Portal should not be conflated; #167 adds installed-source parity. |
| #146 | Rewrite as the remaining generic shared-service composition/authorization work; keep open. | The narrow kind/Ollama path is delivered, but arbitrary configured instances, credential-reference failure, production identity and running-claim revision snapshots remain broader. Remove any statement that CLI and Portal still use separate services. |
| #147 | Rewrite as general multi-template registration, read-only template query/selection and artifact compatibility; keep open. | #165 provides one create-only compatible reference template, not two selectable templates or arbitrary worker images. Make #146 a related contract, not an all-or-nothing delivery blocker. |
| #47 | Narrow dependencies to the installed reference path (#165/#167) and independently verified policy boundary (#46) only where its own scenario truly needs it; keep open. | A repeatable fresh-cluster, two-independent-claim/cross-claim harness is not delivered by manual E2E evidence. #43 local-memory golden path and broad #147 are not prerequisites for this test. |
| #46 | Keep open for genuine independent Team B identity proof. | The current installed Team A policy can deny an unmatched project, but cannot switch to trusted Team B. |
| #43 | Keep open for deterministic local golden command/Team B/failure matrix; distinguish from installed reference path. | #165/#167 do not automatically satisfy the separate reference/memory acceptance. |
| #56 | Keep open for a teammate's clean-environment rehearsal; allow guide drafting before #47. | README and reference guide exist, but independent reproducibility is not yet verified. |
| #58 | Remove #52 as a hard prerequisite to drafting the current tutorial; explain bypass boundary as a known gap. | A tutorial can accurately state the limitation before bypass proof is complete. |
| #130 | Correct the stale claim that the API only retrieves one item; retain persistence, authorized visibility and pagination as future work. | #167 adds a bounded in-memory list, not durable or multi-user history. |
| #107 | Refresh the current checkpoint and remaining presenter/rehearsal work after merge. | The installed CLI/API/UI parity will be delivered, but teammate rehearsal and production gaps remain. |
| #37, #38 | Check owner progress and precise append-only/evidence acceptance before closing; avoid taking over active work by assumption. | Existing reference facts/evidence do not necessarily finish every original contract. |

Create a focused follow-up for safe Platform rollout while claims are active:
the current single-Pod, process-local Work service can interrupt Work and lose
history on a new apply. Do not duplicate #130 durability or #91 identity scope;
the new ticket should own drain/revision-stable routing and cleanup behavior.
Do not make #155/#161/#152 (provider credential integration,
environment-neutral path, guided bootstrap) blockers for the already verified
local kind/Ollama reference run; keep their separate product scope.
