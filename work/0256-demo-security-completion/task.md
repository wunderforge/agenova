# Task: Truthful Researcher completion, native HTTPS evidence and walkthrough UI

- Ticket: [#256](https://github.com/wunderforge/agenova/issues/256), parent #218; related #242/#243.
- Mission: make incomplete research understandable and demonstrate actual native HTTPS attempts without granting Pod egress.
- Target: reference Researcher, controlled execution edge, canonical outcome, connected Work UI, user editor and Platform summaries.
- User value: useful incomplete answers, visible reasons, truthful enforcement evidence and coherent actions.
- PRD outcome: [facts/accountability and final-demo journey](../../docs/product/prd.md).

## Context to Read

AGENTS.md, PRD, [architecture](../../docs/product/architecture-contract.md), [spec](spec.md), [design](design.md), [playbooks](../../docs/harness/playbooks.md), parent-workspace improvement plan dated 2026-10-23.

## Scope

Owner authorized implementation, normal merge and deployment after gates on 23 October. No branch-protection bypass, secrets, new IAM/RBAC or paid infrastructure approval is inferred. Preserve existing permissions and immutable history. Include grouped page actions, Admin inherited-project presentation and MCP adapter/instance distinction. No marketplace/search engine, arbitrary command tool, role redesign or DNS change.

## Acceptance Criteria

- Recoverable tool failures reach the agent; incomplete/partial replies survive and are not green business successes.
- Outcome separates process execution from agent-reported completion, with absent legacy metadata unknown.
- First-screen notices distinguish unavailable capability, real Gateway denial, observed timeout and independently verified infrastructure blocking.
- A model-selected native attempt really executes in the original Work Pod without changing egress. Local and AWS proof are separate; no attempt means security-demo failure.
- Public/CRM/legacy agents, role scope, CLI, image decoupling and cleanup regressions pass.
- Actual connected UI and user editor have browser assertions and before/after evidence.

## Negative Case

No tool/no observation must not become invented success; model-reported network blocking cannot become trusted Deny. Reject forged/mismatched reports, private targets, redirects, stale claim identities and evidence-write failures. A timeout alone cannot prove CNI enforcement.

## Execution Todo

- [x] Scout source and tests; main d3b101e, branch codex/demo-security-completion, unrelated debug.log untouched.
- [x] Assignee self-review against owner scope and architecture: lifecycle/authority unchanged; optional composition-edge/report metadata only.
- [x] Implement completion/report contracts and focused tests.
- [x] Implement bounded native HTTPS in the reference image and persist attributed, explicitly agent-reported observations.
- [x] Implement truthful notice/actions/Admin/Platform presentation; 176 UI unit tests and all 90 browser tests passed. Independent failure/cleanup warnings survive alongside incomplete/partial/native/denial notices; equivalent messages are deduplicated.
- [x] Enforcing local-network joint acceptance; full Windows check.ps1 -All attempted with the known bootstrap test failure retained below. Linux CI remains the release gate.
- [ ] Independent PR review, CI, normal merge/release and exact-image AWS acceptance.
- [ ] Update parent plan, reproducible evidence and user walkthrough.

### Cloud follow-up: reference-image call formatting

#257 merged at `92fdb61` and automatic release `39949039840` succeeded. The first
real v5 public Work (`work-244c91003175eac9a2d5f2344b33bf5ee236e9378f884c080d83817afdab8b10`)
retained an incomplete reply: Nova Micro incorrectly filled final-answer metadata
on tool actions. No MCP call occurred; this is a failed positive acceptance, not
a permissions denial. One isolated exact-image/model format diagnostic reproduced
`action=tool` with `completion=incomplete`; a complete empty-metadata example
passed the bounded diagnostic. These are not governed network evidence.

Follow-up changes only the reference Researcher prompt: complete catalog-derived
tool/native examples, final-answer fields explicitly empty on calls. No parser,
grant, Gateway, model-provider, lifecycle or network-rule relaxation. Regressions
require all eight example fields, valid web/CRM/native shapes, no unrelated web
example for CRM and no advertised tool for an empty catalog. Fresh image and
real-model campaign remain required; retain the failed v5-r1 attempt.

### Cloud follow-up: evidence consumption and CLI native facts

#258 merged at `33193da`; v6 / Policy 7 preserved earlier versions. First public
Work `work-638338e523adec70d1a222b105a6db2ac19403f190de83e72df10f89aadb8f44`
made a real MCP call with correlated successful receipt (2776 bytes), but the
Agent did not finish from the successful observation and unnecessarily attempted
native HTTPS. Its incomplete result is retained, not accepted as a positive.
The normal CLI also rejected the new native WorkerActivity vocabulary; the API
still retained the complete canonical Work. Do not resubmit an unknown result.

Small follow-up: make the reference prompt's decision order explicit after the
transcript (observed evidence before tools before requested native fallback),
include a complete finish example, and teach the CLI the two bounded native
facts with model-selection/order/original-worker correlation. Never accept a
worker report as a CNI verdict or grant. Focused tests must reject unbound,
replaced, unordered, private-IP, mismatched-code and claimed-firewall reports.
No Gateway/provider/runtime/network rules or authority semantics change.

### Short-demo closeout (24 October)

Owner explicitly authorized administrator merge only for #259 after all gates,
then prioritized the short Demo: stable public/CRM positive runs, same-task
missing-Web/native isolation proof and readable UI evidence. Reuse #257/#258 UI
and exact reference image; no new features, services or wider security project.

Deduplication checked live: main remains #258 and #259 remains open at bc4f3ae;
the prompt and native interleaving fix already exist. Only remaining review
invariants are added: succeeded native runs require the observation and a later
correlated model finish/FinalAnswer/Outcome.Model; reference report phase/IP/status
relationships must be possible. Failed interrupted attempts remain readable.
The stricter report projection is CLI-local; no reference-image input changes.

CI's repeated 20ms bind-setup timeout is in the fake adapter, not production.
Use a 1s positive bind setup budget; retain the explicit 20ms timeout case and
existing cleanup/cancellation deadlines. Focused suites and 15 repeats of the
affected cold-recovery/cancellation/lifetime cases pass. Full Linux CI/review and
real cloud campaign remain required; no unknown submission retries.

AWS and personal CLI logins expired during the discussion; normal user login is
requested. No cloud configuration changes before access is restored. The single
approved AWS success-control Pod has not yet been used. Local kind evidence is
retained and its dedicated cluster was already removed.

## Quality Gates

Focused Go packages; npm --prefix ui run check; ./scripts/check.ps1 -All; actual Pod network checks in an enforcing CNI; positive web/CRM and missing-web real-model runs; image/Policy/Work/Pod attribution. Failures stop broad expansion. Existing platform-specific gate failures remain explicit.

## Evidence Required

Canonical Work JSON; successful MCP receipts; native attempt phase/result and original Pod UID; CNI verdict or controlled same-path allow/deny; screenshots; exact tests and release versions. No raw credentials or task/provider bodies in telemetry.

## Constraints and Decisions

Controlled-v1 and old images remain supported; optional execution-edge capabilities are not grants. Native diagnostics are local image code, never a Gateway proxy or control-plane fetch. Application roles never bypass Work Policy. AWS login currently valid; personal CLI login needs refreshing before production Work acceptance. New administrative bypass requires explicit approval if normal merge is blocked.

Local evidence: isolated kind 1.36.1 + Calico 3.33.0 (manifest SHA256
2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5).
Same image/node/public IP produced HTTP 200 in the control Pod and a 5s TCP
timeout in the denied Pod; actual image fixture-driven native attempt and
incomplete final reply passed. This is not real-model or AWS proof.
Focused Go packages, full actual PostgreSQL console/role suites, and durable
staging/restart/history completion regression tests passed. Both recovery
reconstruction windows preserve incomplete/partial classifications.
Windows unskipped -All fails the known TestOwnerBootstrapDeliveryGuards
directory-sync/delivery test; retain the failure and require exact-source Linux
CI, not a silently skipped green claim. Personal CLI login subsequently restored.
Owner approved administrator merge only for PR #257 after CI and independent review pass.
Owner separately approved one temporary same-node/image credential-free AWS
control Pod with no node/IAM/log service additions and deletion after testing.

Sanitized exact-image local record: [evidence](evidence/local-network.json).
Reproduction prerequisites: [local network commands](network-reproduction.md).

