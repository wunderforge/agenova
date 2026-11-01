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
- [x] Test Public and Internal customer objectives with the same Agent; permission
  requests/grants, actual attempted calls and answer quality are separate checks.
- [x] Avoid selecting the first granted tool as a substitute for task reasoning,
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

## 3. Task type — implemented locally, owner acceptance pending

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

## 4. Task preset — removal confirmed, implementation not started

Owner decision: 1 November 2026. Record TODO only; do not implement yet.
Selecting a preset currently changes placeholder text rather than filling a usable
task, so the user still has to type. The owner explicitly chose to remove the
feature, superseding the earlier open examples/shortcut discussion.

- [ ] Remove the Task preset feature from New Work, including its selector,
  preset-specific placeholders and implicit preset-driven form mutations.
- [ ] Keep direct objective entry and Manual / Suggest / Automatic permission
  preparation. Do not replace the selector with another examples/shortcut control.
- [ ] Preserve user-entered text, Copy as new Work, registered Advisor guide
  profiles and explicit legacy API/CLI task contracts. Review shared consumers
  before removing preset code; removing a UI shortcut must not remove the
  independently configured rules Advisor or its profiles.
- [ ] Verify no Task preset control remains; Public/Internal tasks can still be
  submitted from an objective without a category or preset prerequisite. Confirm
  permission setup, copy/retry and existing coding-task regressions pass.

## 5. Model-call budget — design confirmed, implementation not started

Owner decision: 1 November 2026. Record together with Task preset removal;
no development, service restart, merge or deployment is authorized by this TODO.

Current finding: `workerprotocol.MaxTurns` is a shared hardcoded value of six.
The reference Agent, host protocol exchange and Work execution handler enforce
it. It is not a configurable, general Model Gateway quota. Changing the image
alone cannot increase the current effective limit.

- [ ] Give the reference Researcher image its own internal safety limit of 20
  model calls, independently of the platform-approved Work budget. Preserve
  arbitrary-image support; other Agent implementations own their internal loops.
- [ ] Add **Max model calls** next to **Time limit** under Run settings. Keep
  the platform default at six; allow an explicitly configured Demo request of
  20 when permitted by the server's Template/Policy budget ceiling.
- [ ] Define and validate an optional budget in the Work/API contract, including
  the approved effective value, server-owned ceiling and legacy omission default.
  Reject invalid or above-ceiling requests with a clear reason. A browser value,
  Agent image or Advisor suggestion cannot raise the approved budget by itself.
- [ ] Replace the shared six-turn coupling in the execution handler and protocol
  exchange. Their transport/action limits must accommodate the approved budget
  without accidentally retaining a lower fixed ceiling. Preserve existing
  protocol identity checks, timeouts and tool/model authorization.
- [ ] Enforce the approved budget on the trusted side before provider dispatch.
  Define one budget unit as one execution-model request attempt through the
  governed boundary, not a tool call or an Agent's internal reasoning step.
  Reserve atomically so parallel requests cannot exceed the cap; retries after
  malformed model replies consume additional units. Document provider-internal
  retry behavior separately. Advisor preparation remains separately recorded.
- [ ] Retain consumed budget across supported Work resume/recovery paths; a new
  process, repeated submission or retry must not reset an existing Work's quota.
  Use existing persistence where available; explicitly label volatile-preview
  limits rather than claiming durable quota enforcement there.
- [ ] In Work details show requested/granted model-call budget and actual usage,
  for example `Model calls · 3 / 8`. Record a bounded budget-exhausted reason and
  evidence; distinguish exhausted quota from denied permission/provider failure.
- [ ] Let the Agent return an existing result and report partial/incomplete when
  the budget is exhausted, without forcing an unexplained task failure. Any
  model-based finalization must fit within the same approved cap; no hidden
  extra model call. Never invent an Agent answer or business completion.

Acceptance: an image able to run 20 calls under a platform budget of two can make
at most two execution-model dispatches; a third attempt records exhaustion and
causes zero additional provider calls. With an approved budget of 20, requests
beyond the previous six-call limit must work. Verify exact-cap completion,
lower image limits, parallel attempts, invalid values, retry/recovery, timeout,
manual/upstream operation, separate Advisor usage and unchanged access denial.
Use deterministic provider counters for boundary tests; a 20-call paid/cloud
inference run is not needed or authorized. UI tests cover adjacent controls,
copy behavior, requested/granted display and mobile layout.

## 6. Prominent Work status and activity — design preview, approval pending

Owner request: 1 November 2026. Redesign the Work details status/activity hierarchy
without weakening the requested-versus-granted presentation. A design mock has
been produced; no actual UI or service changes were made.

- [ ] Put a compact lifecycle strip directly below the Work title: Requested,
  Authorized, Agent, Result, Cleanup. Show the current recorded action, elapsed
  time and most recent event. Keep Permissions immediately below it, with the
  existing requested/granted columns and restrained color rules unchanged.
- [ ] Make recorded stages navigate to and highlight their corresponding Activity
  event, opening that event's details. A running Agent stage targets its latest
  model/tool operation. Unobserved stages must not link to invented events.
- [ ] Display Activity as an open section, not a bottom collapsed disclosure.
  Keep model/tool iterations there rather than pretending they occur once as
  sequential top-level phases. Raw evidence and secondary metadata can collapse.
- [ ] Distinguish execution lifecycle from task completion: a recorded Result and
  successful Cleanup can coexist with an incomplete task. Missing capability is
  not automatically a Gateway refusal or infrastructure-blocking event.
- [ ] Derive state from existing Work facts. Do not invent percentage completion,
  duration, successful preceding stages, or a Live indicator without refresh
  evidence. Preserve failure/unavailable banners and distinguish a failed stage
  from a later successful cleanup. Keep the user's scroll position during updates.
- [ ] Test pending/running/completed/incomplete/denied/failed/cleanup-failure and
  missing/stale evidence, click-to-record behavior, keyboard navigation, mobile,
  stable permission rows and existing actual activity links before implementation
  is accepted. The current mock covers running/completed/missing-access/waiting
  designs only, with seven interaction/layout checks and no JavaScript errors.

Design rationale: reuse GitHub Actions' status-to-step-log navigation and
New Relic's timeline-to-details navigation, adapted to a compact lifecycle strip
and repeatable model/tool event list. These sources support the interaction
pattern; they do not establish Agenova runtime behavior.

- [GitHub Actions visualization graph](https://docs.github.com/en/actions/how-tos/monitor-workflows/use-the-visualization-graph)
- [New Relic trace details](https://docs.newrelic.com/docs/distributed-tracing/ui-data/trace-details/)

## 7. Platform Advisor planner model — TODO only

Owner request: 1 November 2026. The Task Access Advisor card currently shows
`Planner: model`, which identifies the engine but omits the concrete model.

- [ ] Show a **Planner model** field on the Platform Task Access Advisor card,
  populated from the active server configuration (for example
  `qwen2.5-coder:7b`), not inferred from the execution Agent's model or hardcoded.
- [ ] Keep planner engine and model identity distinct. For a rules-only engine,
  show Not applicable; for unavailable model metadata, show Not reported rather
  than guessing. Configuration does not establish verified provider availability.
- [ ] Expose only safe model identification, not API keys, credential values or
  sensitive provider connection parameters. Existing per-Work Advisor provenance
  continues to represent the model actually used for that historical preparation.
- [ ] Verify the configured identifier is visible on the card without opening
  details, configuration changes render correctly, long names fit on mobile,
  and rules/disabled/missing-metadata states are handled truthfully.

No development or deployment is authorized by this recording request.

## 8. General-purpose model profile naming — TODO only

Owner request: 1 November 2026. The Researcher uses the profile identifier
`approved-coding-model`, which incorrectly suggests a coding-specific purpose.

- [ ] Adopt a general-purpose profile name for new Researcher configurations;
  proposed identifier: `approved-general-model`. Keep the profile concept distinct
  from the actual provider/model identifier shown elsewhere.
- [ ] Update the relevant provider mapping, Template defaults/limits, Policy,
  Advisor guide references, form values, examples and tests consistently. Do not
  perform an unrestricted repository-wide string replacement or change unrelated
  coding-specific profiles, models, credentials or authority boundaries.
- [ ] Preserve explicit old API/CLI profile references and historical Work
  evidence. Use an explicit compatible mapping where needed, never a wildcard
  fallback. Respect immutable registrations by using new versions/identities
  rather than silently rewriting existing Templates or evidence.
- [ ] Verify new Public/Internal tasks display and request the generic profile,
  resolve to the intended configured model, and still pass access checks. Old
  requests/copies remain usable; unknown/ungranted profiles remain rejected.

Separately, the owner deferred the broader editable Platform configuration /
Advisor-toggle proposal. Do not add it to this implementation batch. Section 7's
read-only Planner model display remains a separate recorded TODO.

## 9. Whole-UI visual consistency — TODO only

Owner feedback: 1 November 2026. Work details still contains browser-default
buttons, including **Copy request ID** in the expanded execution details. The
owner requires visual inspection of every UI change, including typography and
layout, rather than relying on functional checks alone.

- [ ] Audit all reachable pages, dialogs and expanded/empty/error/loading states
  for unstyled buttons and inconsistent controls, including deep Work activity
  and evidence views, comparison, New Work, Agents, Administration, Policy,
  Platform and authentication pages. Reuse the June component styles; avoid
  page-specific cosmetic patches and browser-default controls.
- [ ] Fix action grouping/placement, button height/padding/weight, keyboard focus,
  icon alignment, font families/sizes/line heights, spacing and long-ID wrapping.
  Keep secondary metadata visually secondary. Do not introduce decorative colors
  or disturb the requested/granted semantics and existing warning-banner rules.
- [ ] Distinguish lifecycle completion from goal completion in visible labels:
  this Work's process ended successfully but its task was incomplete; avoid a
  standalone **Work completed** label implying the customer's goal succeeded.
- [ ] Every UI implementation slice requires rendered desktop/mobile review of
  its affected states, in addition to interaction/accessibility checks. Acceptance
  must explicitly include visual quality, not only screenshot generation or a
  passing click assertion. Preserve original local Work records during fixes.

Confirmed code finding: `CopyRequestID` in `ui/src/TaskInstructions.tsx` renders a
native button without the shared button class. The surrounding copy-reference
layout rule does not supply the missing button style. Other pages still need the
planned complete audit; this finding is not evidence that all have been inspected.

## 10. Internal task diagnosis and per-turn evidence — design follow-up, no implementation

Observed local Work: `work-7413c4ba-c78e-46eb-8fee-6fb48bfc8d01`, named
`1_nov_test_4`, on 1 November 2026. Objective: identify three customers with the
largest usage declines and draft follow-up recommendations.

Verified from the local evidence API (no new task was submitted):

- No tools were requested or granted. `crm:customer-retention` was requested and
  granted as a resource scope; a scope alone does not grant `crm.read`.
- One real execution-model request, zero ToolDecision facts and zero tool
  ProviderAttempt facts. No Tool Gateway refusal or tool backend execution was
  recorded. No Advisor preparation receipt exists for this Work.
- The model selected a final-answer action in Turn 1 and reported
  `incomplete / missing-capability / web.read`. Its answer incorrectly claims
  public web access is required for this internal customer query. This is an
  Agent-reported misdiagnosis, not evidence of a platform block.

Confirmed implementation gaps:

- `workCatalog` is derived from effective authority. This Work's callable catalog
  is empty; the local constrained decoder offers no tool action when that catalog
  is empty. The current path cannot be presented as an attempted forbidden call.
- `ActionValidated` records generic action-format text, but not enough structured
  selected-action details to show each public Agent decision and its target.
- `groupWorkerTurns` filters Model access / Tool access records when turns or
  provider calls exist. A ToolDecision Deny with no provider call can therefore
  disappear from the per-turn view. This is a confirmed rendering gap, separate
  from this particular Work having no such denial at all.
- The banner uses Agent-reported completion target plus the grant list to label
  a capability unavailable; it does not establish that this is the capability
  actually required by the business goal. Preserve source attribution.

Proposed repair order for discussion:

1. Fix reference-Agent missing-capability reasoning for internal/public tasks,
   including empty-catalog cases. Use Agent-owned tool/guide context and truthful
   observations, not platform keyword-based business routing. Do not silently add
   permissions or rewrite the stored answer/evidence for this historical Work.
2. Record safe public action fields per turn: model invocation/turn ID, selected
   action kind, tool and resource target, validation result, linked platform
   decision, actual provider attempt/outcome, and bounded observation/final status.
   Show selected actions, not private chain-of-thought, raw prompts or credentials.
3. Render each turn as model action -> platform response -> actual execution or
   explicit not-executed -> observation/next action. Correlate by identifiers;
   retain denied access decisions even when no provider execution exists. Keep
   repetitive successful access checks subordinate rather than duplicating rows.
4. Implement the confirmed discovery-versus-authorization design in section 11
   when development is separately authorized. Provide a deliberate, real
   forbidden-call acceptance scenario through the actual Gateway. Changing the
   catalog/protocol boundary requires compatibility tests; do not simply remove
   its checks or manufacture a refusal. Unknown/malformed calls and
   ungranted-but-installed calls remain distinct.
5. Integrate the per-turn view with the proposed top lifecycle strip; make warning
   links land on the actual selected-action/denial record. Preserve Permissions as
   the primary authority comparison. For images without decision instrumentation,
   show Not reported; do not require every image to adopt this reference Agent's
   internal loop or invent its intentions from Gateway activity.

Acceptance: an authorized internal task actually invokes CRM and returns grounded
customer findings; the same internal objective with no tool grant reports a
relevant capability limitation and no fabricated web dependency. A separately
observed forbidden CRM call must retain the real Gateway Deny, zero backend calls
and matching IDs in the visible turn flow. A model choosing not to call anything
must instead show its actual finish action with zero Gateway refusal. Cover tool
failures, unavailable backends, invalid actions, multiple turns, missing evidence
and lifecycle-versus-task status. The platform must not certify business meaning
or infrastructure enforcement based solely on an Agent's narrative.

Reference: [OpenTelemetry GenAI agent spans](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md)
distinguishes Agent operations and tool execution and supports tool-call
correlation. These conventions are under development; reuse the separation and
correlation pattern without making it a dependency on a particular Agent SDK.

## 11. Tool discovery separate from Work authorization — confirmed direction, design only

Owner confirmation: 1 November 2026. Develop this design and record TODOs only;
do not implement, restart services, run new Work, merge, deploy or access remote
infrastructure. This supersedes using Work grants to hide all ungranted tools
from the reference Researcher's discovery catalog.

### Responsibilities and contracts

- **Discovery catalog:** safe descriptors of the selected immutable Template's
  supported tools intersected with registered platform routes and the caller's
  organization/project discovery visibility. Discoverability is not permission
  to execute. Known backend health can be shown separately; a transient failure
  must not silently become a change in authorization or a claim that a tool never
  existed. Do not expose other tenants' tools, credentials, private endpoints or
  actual customer records. Input schemas and scope descriptions must themselves
  be safe to disclose; discovering a tool does not authorize enumerating data.
- **Work authority:** requestedAccess still resolves through the existing
  membership/Policy/Template intersection into server-issued EffectiveAuthority.
  Discovery never changes that grant. The permission table continues to show
  only this Work's requested items and their actual grants; do not insert every
  discoverable tool as an unrequested permission row.
- **Gateway enforcement:** authenticate and bind the actual caller to the active
  Work, check expiry/revocation and the effective tool operation/resource scope,
  then dispatch an allowed, valid request to the backend. A known, ungranted tool
  request must reach the Gateway's authority decision and record Deny with zero
  backend executions. Identity errors, malformed/unknown requests, unavailable
  backends and tool execution failures remain distinguishable recorded stages.
  Neither tool descriptions nor a model's selected action can issue authority.
- **Agent behavior:** choose actions and handle tool responses independently.
  The discovery catalog may contain both web.read and crm.read even when the
  Work grants only one or neither. A permission summary may help normal behavior,
  but no model instruction, SDK filter or hidden-tool list substitutes for the
  Gateway check. Agent-local failures/observations remain explicitly attributed.
- **Network boundary:** preserve credential isolation and existing egress rules
  so images cannot bypass governed interfaces by directly accessing MCP backends
  or public networks. Local synthetic lifecycle records are not packet-blocking
  proof; real infrastructure bypass validation remains a separate backend gate.

Example: the same Researcher Template discovers web.read and crm.read. A public
Work grants only web.read/public-page scope; a CRM request is denied. An internal
Work grants only crm.read/customer-retention scope; a web request is denied.
A Work with no tool grant may discover both but execute neither. Discovery
visibility alone never causes Work admission or tool execution to be allowed.

### Minimal changes, in dependency order

1. [ ] Define distinct descriptor/discovery and issued-authority inputs at the
   console/Worker boundary. Audit workCatalog, ActionSchema/ResearchSchema,
   GatewayResearchSchema and Exchange validation. Construct action grammar from
   the approved discovery descriptors rather than the narrower Work grant.
   Preserve structural, registered-input and identity checks; do not replace
   EffectiveAuthority with the discovery list or broaden actual resource access.
2. [ ] Introduce explicit protocol version/capability negotiation for the changed
   catalog meaning. Preserve existing controlled-v1/legacy callers and arbitrary
   compatible Agent images; do not require a new base image or one Agent framework.
   Reuse the installed Tool Gateway/MCP backend; no extra MCP service is required
   by this split, and backend-specific endpoints remain in their adapters.
3. [ ] Exercise the real Gateway for known discoverable-but-ungranted requests.
   Keep server-side authentication/authorization authoritative even for a client
   that ignores SDK hints, changes its prompt or constructs a request directly.
   Keep grant checks ahead of backend execution; treat Deny as an observable
   tool response the Agent can handle, rather than automatically failing the
   entire Work or falsely reporting tool absence.
4. [ ] Record the safe public Agent action and actual platform response per turn
   as in section 10. Link Work/turn/model-call/action/request/decision IDs. A
   denied request with no provider invocation still needs a correlation ID;
   never fabricate a provider call/span to connect it. Missing Agent telemetry
   remains Not reported; Gateway facts still stand independently.
5. [ ] Update reference-Agent context and examples to distinguish supported,
   authorized, denied and unavailable tools. Fix the internal CRM misdiagnosis
   using Agent-owned context/observed responses. Do not add task-keyword routing
   to the platform, widen permissions or reinterpret the old 1_nov_test_4 record.
6. [ ] Show each turn's selected action, Gateway decision/reason, backend status
   and subsequent observation in the open Activity view. A genuine denial should
   be reachable from the top warning/progress strip even without a provider call.
   Preserve the simplified permission table and desktop/mobile June styling.

### Acceptance gates

| Case | Required result |
| --- | --- |
| Same Template, different Work grants | Discovery descriptors remain the same for a caller with the same visibility; effective grants and permitted executions differ. |
| Authorized internal/public task | Real model selects a relevant tool; actual Gateway Allow, matching MCP/backend receipt and grounded result. |
| Known ungranted tool | Deterministic client/probe sends an actual request under the narrow Work identity; real Gateway Deny, linked decision and zero backend calls. Repeat for web.read and crm.read. |
| Tool granted, resource scope not granted | Actual request is denied for scope; no backend execution. |
| No tool grants | Both supported tools can be discovered; neither request executes. A model-only final answer is not called a Gateway denial. |
| Bypass attempts | Direct Gateway requests still require valid Work identity/authority. Forged/wrong-Work/expired identity, unknown tools and invalid arguments fail at their actual recorded stage. |
| Agent handles denial | Denial is available as a tool observation; the Agent may continue permitted work or finish partial/incomplete with a relevant reason. Runtime success does not certify the business goal. |
| Visibility/privacy | Other projects/tenants' hidden tools, secrets, endpoints and data do not appear in discovery. Descriptors never grant or enumerate protected data. |
| Compatibility and evidence | Legacy/manual/upstream/Advisor/copy/retry paths remain valid. No mandatory Advisor or invented turns; old records unchanged and uninstrumented Agent actions labeled Not reported. |
| Visual quality | Visible per-turn decisions include refusals with no provider call; links land on actual evidence. Requested/granted layout, typography, buttons, focus, desktop/mobile layouts are reviewed. |

A normal model may abstain from an ungranted call. Therefore include a controlled
probe that guarantees a real request for the boundary test, plus a real-model
scenario whose actual actions are reported faithfully. The probe is not a fake
Gateway and does not expand authority. Local joint acceptance comes first;
rebuilding remote infrastructure and cloud verification require later discussion.

Reference: [MCP Tools discovery and invocation](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
defines separate tools/list and tools/call operations. The Template visibility
rules and per-Work Gateway authorization above are Agenova design decisions,
not guarantees supplied by MCP itself.

## Next batch — ordered TODO, waiting for implementation authorization

1. Remove Task preset and its UI-only mutations; preserve independent Advisor
   profiles and legacy task contracts. Run focused form/copy regressions.
2. Define the model-call budget contract/default/ceiling and trusted accounting;
   test rejection, exact caps, concurrency and supported persistence/recovery.
3. Decouple the reference image limit (20) and host execution limits; prove that
   the lower approved platform cap wins and that approved calls above six work.
4. Add the Run settings control and Work details usage/exhaustion presentation.
5. Implement the discovery/authority split and independent Gateway checks from
   section 11, then section 10's internal-task diagnosis and correlated per-turn
   action/decision evidence. Direction is confirmed; development is not started.
6. Display the configured Planner model on the Platform Advisor card (section 7).
7. Apply the generic model profile naming and explicit compatibility plan
   (section 8), without changing the actual execution model or permissions.
8. After visual design approval, implement the prominent lifecycle/activity and
   correlated turn UI (sections 6, 10, 11), preserving Permissions as the primary
   requested/granted comparison.
9. Perform the whole-UI consistency audit in section 9 and visually review every
   affected page/state while preserving the requested/granted layout.
10. Run joint Public/Internal and missing-permission regressions across Manual,
   Suggest and Automatic modes, plus timeout/legacy API/CLI/copy/mobile checks;
   prepare local owner acceptance. No automatic merge/deployment is implied.

## Planned repair sequence and acceptance gates

| Order | Work | Required evidence before proceeding |
| --- | --- | --- |
| 1 | Failure observability | Known executor/tool/model failures retain a safe code, stage and actionable reason in Work evidence and at the top of the failed Work. Unknown causes remain unknown; no fabricated denial or leaked raw error. |
| 2 | Decouple ordinary submission from task categories | Audit parser, CLI/API, Advisor, executor and form consumers. Ordinary Manual/upstream and model-Advisor requests accept generic/omitted classification according to a documented compatibility contract; task text drives planning. Existing explicit types, canonical digests/retries, guides and coding inputs pass regression. |
| 3 | Real Researcher execution and usable tool inputs | Replace the synthetic acceptance substitute for business-task verification. A short objective without a URL produces observed real-model decisions and governed tool calls with relevant results. Keep synthetic CRM data labeled and prove missing-access attempts are blocked at the actual boundary. |
| 4 | Simplify the form | Remove Task type after the underlying dependency is removed. Keep Task → Permissions → Run settings. No hidden category prerequisite or new mandatory classification call; specialized required inputs remain available. Task preset removal was separately confirmed on 1 November; implementation is authorized in the 1 November local follow-up batch. |
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

The 1 November owner instruction now authorizes implementation of sections 4–11,
local joint tests and commit/push to the current feature branch. This supersedes
the earlier recording/design-only status. Track development and evidence in
[the implementation plan](implementation-plan-20261101.md). Owner acceptance is
still pending. Platform configuration editing/toggles, a search provider, remote
recovery and deployment remain outside this batch. Do not access remote infrastructure.
