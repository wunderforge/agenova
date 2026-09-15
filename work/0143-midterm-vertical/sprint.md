# Sprint control plane

Entry: midstream; fast bounded integration batch. Owner: wunderforge. Checkpoint: UI submits -> kind worker -> real governed LLM -> actual evidence/result in UI, plus denial.

## Verified baseline

- Main base: 18168a9; #136 kind seam and #33/#34/#35 merged.
- Original main worktree is dirty and excluded. This worktree: `agenova-0143-vertical-wt`, branch `codex/0143-midterm-vertical`.
- Accepted React preview: clean `agenova-react-ui-wt`, branch `codex/react-product-ui`, head 10422b4; integrate only after evidence contract review.
- #36 / PR #142 follow-up 6424f82 is merged locally: immutable launch/gateway input snapshots and exact NPM/GitLab/SSH aliases.
- #37 has original owner wuy10; coordinate before establishing a second writer.

## Execution map

Main-only: fact/evidence contracts, identity/worker transport choice, submission/API composition, PRD exception, integration/release.

Writer lanes finished. Root owns integration/release. Core composition: 0998b9a; provenance/live gates: 648f45d; React integration: d13e4b1.

Independent core review fixes passed re-review. Final integrated release review is active and read-only. Owner approved blind Codex review; no Claude quota spent.

Pending integration limit: two. Stop a lane after two same gate failures; root diagnoses. No writer changes frozen acceptance or shared boundaries from a consumer branch.

## Recovery

Recovery automation: not installed; one existing thread heartbeat prevents creation. Do not replace it without authorization.

Live HTTP/kind gate passed, including Team B zero claim/backend/provider calls. Live browser gate passed against local Ollama: actual text/usage, same claim/worker/invocation, 14 facts and cleanup. Artifacts: `.tmp/ui-kind-checkpoint/`.

Integrated baseline passed with Git access: Go/vet, canonical generation, 72 unit and 37 browser tests. Final review's queued cancellation issue fixed at f34b1b7 with a no-further-polling regression.

Next action: record final release review, publish scoped PR and reconcile prerequisite PR #142. Final binary is running on loopback port 8088; repeat browser gate against final source and keep UI/server available for owner's walkthrough. Initial sandbox VCS failure was resolved with correct permissions, not a product workaround.

Required user input: none for the local checkpoint. Existing local model only, synthetic/public tasks, no paid API calls or downloads. Tools/Memory, SSO and durable history remain outside this integration.
