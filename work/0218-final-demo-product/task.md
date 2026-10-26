# Task: Locally acceptable final-demo product journey

- Ticket: [release #252](https://github.com/wunderforge/agenova/issues/252), parent [E20 #218](https://github.com/wunderforge/agenova/issues/218)
- Owner: wunderforge; approved implementation scope: owner's 23 October 2026 request and `../../../agenova-demo-deliverables/Agenova-Demo-Product-Improvement-Plan-20261023.md` in the parent workspace.
- Mission: implement the approved local product journey using canonical requests and evidence, without publishing, merging, deploying, cloud changes or fabricated enforcement claims.
- Targets: connected Portal, administration BFF/PostgreSQL management, public landing and repeatable local acceptance.
- PRD outcomes: declarative requests, scoped authority, durable evidence, invited-team production extension and anonymous landing.

## Context to read

- `AGENTS.md`, `docs/product/prd.md`, `docs/product/architecture-contract.md`.
- Approved improvement plan; `spec.md`; relevant existing member/governance APIs, Portal and smoke tests.
- `docs/harness/playbooks.md`: user-facing demo slice and quality gates.

## Scope

Release authorization update: after the successful local walkthrough, the owner
requested real-model adaptation, shared-environment deployment and testing.
Follow [release prerequisites and acceptance](release.md). This supersedes prior
local-only instructions for this release but does not authorize secret access,
new IAM/model grants, additional infrastructure or branch-protection bypass.

Owner-approved follow-up (23 October 2026): implement the reviewed June design
in `../../../agenova-demo-deliverables/product-design-review-20261023/DESIGN.md`:
permission-filtered Agent directory, per-user administration, balanced Platform
configuration summaries, project-scoped Researcher and research project, and
web.fetch/crm.read extensions to the existing MCP service. Retain old templates,
projects and tools. External data access for this new scenario is gateway-only;
direct Pod public egress must not be obtained from a Work request. Test locally
with real APIs/SQL/MCP, distinguish synthetic model/runtime and real network
evidence. No merge, deployment, cloud changes or real credentials authorized.

S0/S0a submission recovery; S1 member/project management; S1a templates and generic tasks; S2 policy history/activation concurrency and two-Work comparisons; S3 truthful telemetry/boundary/status presentation and stable navigation; S4 June landing narrative. Tenant display metadata uses a separate additive table, never changes trusted identity. No general organization hierarchy, LLM access planner, workflow engine, cloud resources, runtime/CNI replacement or public marketplace.

## Acceptance

- Same request retries retain identity and reconcile uncertain outcomes; a new logical request gets new identity and normal authorization.
- Project grants are edited individually; configuration visibility does not confer submission permission.
- Project/template changes cannot submit stale data. Generic tasks and old repository-change requests both work.
- Policy constraints and original versions are readable. Immutable policy activation is atomic and optionally compare-and-set. Historical versions do not change historical authority.
- Two real API Work views compare without synthesizing evidence; copy creates an editable new request, not authority.
- Logs/metrics/traces show configured versus unverified state; safe configured diagnostic links, no credential-bearing URLs.
- Runtime configuration is not reported as verified packet enforcement. Synthetic local acceptance never claims AWS or real Google/provider validation.
- Landing and anonymous asset boundaries preserve authentication and membership requirements.
- Local UI and administration are available for owner acceptance with repeatable backend/API/browser tests and screenshots.

## Negative cases

Lost response, same key changed body, unauthorized management, revoked project access, stale policy writer, missing historical version, empty scope, stale template response, invalid diagnostic URL, unavailable service, mixed fixture/live views.

## Execution

- [x] Read accepted plan, E20, routing, PRD and architecture; clean branch based on main 58d951c, preserving unmerged network branch.
- [x] Self-review: owner-approved presentation/management extension; preserve all authority and credential boundaries.
- [x] Implement and test submission recovery and selection model.
- [x] Implement administration, policy management and additive metadata/history support.
- [x] Implement Work comparison, boundary/result view and observability metadata.
- [x] Complete landing, local integrated acceptance and visual proof.
- [x] Execute focused/full Go/UI checks, build and browser suites; preserve the known Windows directory-sync failure instead of claiming a green original gate.
- [x] Record results, limits and owner acceptance instructions; no merge/deploy.
- [ ] Owner accepts the local product. Separate production migration/rollback and real environment verification approval before any release.

### Authorized design follow-up

- [x] Agents discovery table, server-filtered project/tool facets, detail drawer;
  discovery and submission permissions remain distinct.
- [x] Administration defaults to a user table and single-user editor; independent
  project saves preserve other grants. Platform has six balanced configuration cards.
- [x] research project / researcher registration, preserved old agents/projects,
  and web.fetch / crm.read in the existing real Go MCP service.
- [x] Local real API/SQL/MCP joint journeys: both tasks, cross-tool refusal,
  wrong project / forged network intent / Viewer denial, visibility revocation,
  immutable Policy activation/rollback and legacy-path regression.
- [ ] Native Pod packet isolation in an enforcing backend; existing local kindnet
  is not an enforcing CNI. No kind/EKS mutation was authorized or performed.
- [ ] Real model business-answer quality and production Google/Worker golden path;
  synthetic model/runtime are explicitly labelled, not passed off as these checks.

## Gates

### Owner-authorized walkthrough fixes — local implementation

Owner approved implementing these TODOs locally on 23 October 2026. No commit,
merge, deployment, cloud mutation or reading real credential files is authorized.
Use empty objective plus contextual example placeholder; presets suggest type and
access but preserve a user's objective/work name. Custom clears suggested access,
not user text. Add explicit bounded text parameter mode; old allowlist profiles
retain their canonical form and refusal behavior. Owner subsequently clarified
the goal is public-network demonstration without any key-dependent service:
implement web.read(url), not a search API. No Tavily account/key is needed.

- [x] Dropdown focus styling: the strong purple outline remains after mouse
  selection until focus moves elsewhere. Make pointer interaction visually quiet
  while retaining an accessible keyboard focus-visible indicator; verify both
  mouse and keyboard selection rather than removing all focus outlines.
- [x] Research task preset / objective prefill: review the current automatic
  "What should it do?" content and the Custom task transition with the owner.
  Avoid misleading preset/description combinations or silently overwriting
  user-authored objectives. Implemented as empty input plus contextual example
  placeholder; selecting any preset preserves user-authored task text/name.
- [x] Keyless public web access (supersedes the initial keyword-search proposal
  following the owner's clarification): web.read(url) accepts a supplied public
  HTTPS page through MCP/Gateway. No search provider, API key or implicit credential
  access. SSRF/IP/DNS/redirect/size/time boundaries are retained; direct Agent Pod
  public egress stays absent. This is page retrieval, not keyword search. Publish
  researcher-v2 rather than overwrite the immutable original template; old tools,
  registered researcher, projects and historical authority remain unchanged.
- [x] Rename the visible "Research task preset" label to "Task preset".
  Presets are reusable across Agents and task categories, not a Researcher-only
  concept. Align accessible labels, help text and tests; populate choices from
  the relevant Agent/context without making a preset an authorization grant.

No search credentials are required. Real-model quality and native Pod enforcement
remain separate acceptance gates, not implied by local product tests.

Owner walkthrough copy refinement: preset titles describe business goals:
Market research, Customer insights, Product comparison, Incident investigation.
Keep implementation limits in contextual help, not in dropdown titles. No
capability, objective-prefill, request or authorization behavior changes.

### Locally resolved — Researcher missing-tool false success (owner acceptance pending)

- [x] Fix local Researcher execution/result handling when the requested task
  needs an unavailable tool. Owner authorized local implementation together
  with the approved permission-first Work details design; no merge/deployment.
- Reproduce: research / researcher-v2, Market research objective containing
  https://jitsi.org/jitsi-meet/; uncheck web.read, check crm.read, retain scope
  web:public-pages, then submit.
- Verified record: work-4185e2ddb40fef18c92221eba01ef127df4858a05519e348e76542c16a0f15a5.
  Effective tools contain only crm.read; its scope does not match, so the
  executable catalog is empty. No tool call occurred. The synthetic model
  returned an unrelated default answer and execution was marked Succeeded.
  This is neither successful web access nor evidence of a Gateway refusal.
- Acceptance: retain an allowed web.read positive control; in an explicit
  denied-call probe, actually attempt the unauthorized web.read through the
  Gateway and record its refusal with zero MCP web_read invocations. The local
  Researcher must explain it cannot complete the requested web research without
  the tool, not report an unrelated successful answer. Never fabricate a blocked
  event when no attempt occurred. Ordinary agents may legitimately finish
  without tools; do not impose a platform-wide tool-required success rule.
- Add regression coverage for missing tool, mismatched tool/scope and neither
  tool requested; preserve CRM/legacy success and immutable historical evidence.
  Work admission, execution status and business completion remain distinct.

### Approved Work details refinement

- [x] Follow-up: failed Work places actual recorded refusals below the title,
  before Permissions, in the approved compact amber banner: `<target> blocked`
  and `Evidence`, without arrows or explanatory subtext. Successful Work keeps
  the quiet refusal list; ordinary failures without refusal facts have no banner.
  Only presentation and local regression coverage change; owner acceptance pending.

- [x] Apply the accepted focused mock: requested versus granted first, all
  successful values identical black/normal weight, only requested-but-ungranted
  values gray. Unrequested capabilities do not appear. Pending/unknown authority
  is distinct from denial. Never infer a denial from an execution failure.
- [x] Group Agent/caller/team/project/Policy/Gateway/backend/evidence strength as
  secondary metadata, keep actual result intact and activity/evidence accessible.
  Remove repeated explainers without hiding synthetic/unverified evidence facts.
- [x] Run local positive/negative/legacy, UI/browser and actual API/MCP tests;
  preserve the owner's historical false-success record unchanged for comparison.
  161 UI unit, 80 browser smoke and nine final API/SQL/MCP journeys passed.
  Full Go/vet passed with the explicit existing Windows bootstrap test skip;
  the original unskipped `-All` gate still reports that known platform failure.

`go test` focused packages; `npm --prefix ui run check`; `./scripts/check.ps1 -All`; disposable PostgreSQL integration when available. Real backend/Google/model checks require their actual environments and cannot be substituted by fixture successes.

## Baseline and decisions

- Baseline main: 58d951cd8c0bb293fe820263894e490027ce1c2c. Existing #247 network changes intentionally excluded.
- Work on `codex/final-demo-product-local`, existing clean checkout; no remote changes.
- Docker daemon initially unavailable; use only local disposable resources if restored. Default build/test outputs remain on D:.
- Local demonstration uses an explicitly identified acceptance service; production auth is never bypassed by a mode flag.
- Local-only PostgreSQL container `agenova-final-local-pg-20261023`, labelled `agenova.owner=final-local-20261023`, bound to 127.0.0.1:55439; no real credentials/data. Own preview listens only on 127.0.0.1:5210.
- Refresh recovery preserves navigation but clears business records when current access cannot be revalidated. No sensitive stale cache is retained merely for presenter convenience.
- [Acceptance results](acceptance.md) and [owner walkthrough](../../docs/development/local-product-acceptance.md) separate real local APIs/SQL/MCP/public fetch from synthetic identity/runtime/model/customer data and unverified AWS boundaries.
