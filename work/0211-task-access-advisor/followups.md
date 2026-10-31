# Walkthrough follow-ups — local implementation authorized

Recorded: 31 October 2026 (Australia/Sydney).

Owner initially requested recording only. Subsequent instruction on 31 October
2026 authorizes the repair sequence below, complete local implementation/testing,
owner acceptance and commit/push to `codex/task-access-advisor`. No merge,
deployment, paid inference or cloud changes are authorized.

Confirmed product direction (31 October): remove Task type from the primary New
Work interface, remove ordinary-task dependence on a user-selected category, and
preserve API compatibility. This approves the requirement and planning only;
the subsequent start instruction now permits local implementation.

## 1. Preserve and display actionable execution failure reasons

- [x] Record a bounded, safe failure code, stage and explanation in Work evidence.
- [x] Show that reason prominently on the failed Work, with access to its evidence.
- [x] Keep raw provider errors, credentials and infrastructure internals out of the
  public message. Preserve unknown status when no reason was actually observed.

Observed Work: `work-1cce1a24-b337-4b7a-a6bf-150265a1f9d4`.
Task: “Can Zoom's free plan support our team's weekly one-hour client meetings?”
Real Advisor selected `public-research`; requested and granted `web.read` on
`web:public-pages`; admission was Allow. No tool or Worker-model attempt was
recorded. Work ended with `execution-failed` and only the generic message
“Execution failed; no more specific failure reason was recorded.”

Code-path diagnosis: local `previewExecutor` extracts an HTTPS URL from the
objective and returns `local public-page task requires a public HTTPS URL` when
none exists. That specific reason was not retained in the public Work record.
Do not relabel this failure as a Policy/Gateway/infra denial.

Acceptance criteria: reproduce a bounded known error and retain/show its safe reason;
unknown errors remain explicitly unknown. No fabricated denial or invocation.

## 2. A real Agent must perform the task, not a synthetic execution substitute

- [x] Replace the synthetic local task executor/provider with an explicitly
  configured real Researcher for business-task acceptance.
- [x] Let the Agent choose public sources/tool inputs from the business goal;
  do not require a URL in the task text as a platform input rule.
- [x] Use real governed tool calls and observed results. Keep synthetic customer
  records labeled as such, independently of whether model/tool calls are real.
- [ ] Test Public and Internal customer objectives with the same Agent; permission
  requests/grants, actual attempted calls and answer quality are separate checks.
- [ ] Avoid selecting the first granted tool as a substitute for task reasoning,
  and avoid unrelated canned answers being presented as task completion.

Additional observed Work: `work-5c02a8a9-c8d4-4fac-8251-d8e350356e57`.
The Jitsi workshop-assessment objective had requested/granted `crm.read`, no
Advisor receipt, and an unrelated fixed retry/backoff answer from
`synthetic-local-provider` (zero model tokens). Its execution status was Succeeded;
this is not evidence that webpage research or the user's goal was completed.
This record does not establish an Advisor misclassification: it has no receipt.

Acceptance criteria: a short goal without an explicit URL can be attempted by the
real Agent; observed web/customer calls and a relevant evidence-grounded answer
are inspectable. Missing necessary access must have real recorded evidence.
Agent-reported complete/partial/incomplete remains separate from execution phase;
the platform must not acquire responsibility for task semantics or independently
invent a successful business outcome. Gateway authority and Pod network limits
remain intact. Search capability/provider selection, if needed, is a separate
explicit design decision, with no assumed paid API key or new cloud permission.

## 3. Task type — confirmed requirement, implementation deferred

- [x] Remove the Task type selector from the primary New Work interface.
- [x] Ordinary tasks must work from the selected Agent, objective and requested
  access/preparation mode without requiring the user to choose a semantic category.
- [x] Remove behavioral dependence on a hidden category too: moving the dropdown
  to advanced settings or silently filling an implementation-specific type is
  insufficient if the task still fails or routes incorrectly because of that type.
- [x] Preserve `task.type` compatibility for existing API/CLI callers, registered
  guides and historical records. Existing explicitly typed specialized workflows
  must retain their documented behavior; do not rewrite old evidence.
- [x] Define generic/omitted-type handling at the submission boundary and test its
  canonicalization, request digest and retry behavior. No extra mandatory model
  call or compulsory Advisor may be introduced to populate this metadata.
- [x] Model Advisor may plan from the objective with generic task metadata.
  Explicit upstream permission requests and Manual mode remain independent.
  Keep exact-type rules an explicit configured workflow contract rather than a
  hidden dependency of ordinary natural-language submission. Unmatched rules
  must not guess or widen access; define their clear clarification behavior.
- [x] Retain required specialized inputs (for example repository/base branch)
  when removing the type selector. Investigate their appropriate Agent-specific
  input contract; do not remove existing coding functionality or add a generic
  workflow engine as a prerequisite.
- [x] Task type, preset and inferred classification remain untrusted intent
  metadata. None can issue authority or bypass current Policy/Template checks.
- [x] Reconcile CRM type names (`customer-insights` versus `customer-analysis`)
  at their actual consumers with explicit compatibility, rather than substituting
  one name globally and breaking existing requests or registrations.

Original pre-repair facts (retained for provenance): a preset changes proposed tool/scope, task type and placeholder;
it does not overwrite the objective or grant authority. Task type is serialized
in the request. The rules Advisor matches it exactly; the model Advisor can plan
from the objective even with generic `investigation`. `repository-change` also
controls which task-specific inputs the form exposes.

The current generic classifications (research/investigation/summary) primarily
serve implementation routing. A user-facing choice needs independently justified
business meaning, such as drafting versus actually sending a message. An Agent
owns its execution strategy; the optional Advisor prepares requests before the
Work; the platform authorizes and enforces. Classification is not a new authority.

## 4. Task preset — optional UX decision still open

- [ ] Review whether presets should become a small examples/shortcut entry.
- [ ] Preserve user-authored objectives/names when using a shortcut. Clearly
  distinguish a suggested permission request from an actual grant.

Removal of Task type is confirmed. The final placement or removal of Task preset
is not decided by that approval; do not silently expand the UI change to it.

## Planned repair sequence and acceptance gates

| Order | Work | Required evidence before proceeding |
| --- | --- | --- |
| 1 | Failure observability | Known executor/tool/model failures retain a safe code, stage and actionable reason in Work evidence and at the top of the failed Work. Unknown causes remain unknown; no fabricated denial or leaked raw error. |
| 2 | Decouple ordinary submission from task categories | Audit parser, CLI/API, Advisor, executor and form consumers. Ordinary Manual/upstream and model-Advisor requests accept generic/omitted classification according to a documented compatibility contract; task text drives planning. Existing explicit types, canonical digests/retries, guides and coding inputs pass regression. |
| 3 | Real Researcher execution and usable tool inputs | Replace the synthetic acceptance substitute for business-task verification. A short objective without a URL produces observed real-model decisions and governed tool calls with relevant results. Keep synthetic CRM data labeled and prove missing-access attempts are blocked at the actual boundary. |
| 4 | Simplify the form | Remove Task type after the underlying dependency is removed. Keep Task → Permissions → Run settings. No hidden category prerequisite or new mandatory classification call; specialized required inputs remain available. Preset placement is reviewed separately. |
| 5 | Joint local acceptance and owner walkthrough | Repeat same-Agent Public/Internal tasks, Manual/Suggest/Automatic paths, a missing-permission case and failure-diagnosis case. Inspect requested/granted access, actual calls, planning versus execution model metadata, task completion and cleanup separately. Verify legacy API/CLI, copy/retry, existing coding tasks and mobile UI. |

Minimal task corpus for later acceptance:

- Public, no supplied URL: “Can Zoom's free plan support our team's weekly
  one-hour client meetings?” Verify current evidence or report what could not be
  verified; no hardcoded answer or URL substitution in the platform.
- Internal: “Identify three customers with the largest usage declines and draft
  follow-up recommendations.” Real model/tool execution may use explicitly
  synthetic customer records; no permission to send messages is implied.
- Missing access: preserve the goal, remove its required permission and inspect
  the actual attempted call/denial and Agent-reported incomplete/partial outcome.
  The model electing not to attempt a call is a different observation and cannot
  be reported as Gateway or infrastructure enforcement evidence.
- Compatibility: ordinary requests do not require the UI category selector;
  existing explicit `repository-change` and registered-rule clients retain their
  behavior. Repeated identical submissions do not allocate or infer twice.

A tool failure need not globally fail every Agent task: preserve the existing
execution-phase versus Agent-reported completion distinction. The platform must
not infer business success from a runtime Succeeded marker or become an Agent's
workflow/semantic evaluator. Public search-provider selection and any additional
external cost remain separate decisions; this plan does not authorize them.

Execution started under the subsequent owner instruction. Implement and validate
locally first, then request owner acceptance. No merge/deployment is implied.
