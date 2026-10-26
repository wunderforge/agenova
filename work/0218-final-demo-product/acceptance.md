# Local product-journey acceptance — 23 October 2026

## Scope and source

Release update: the owner accepted the walkthrough and authorized real-model
adaptation/deployment/testing, plus 010/011 and minimal grants. Historical local
results below remain local evidence. Current release status is tracked separately
in [release.md](release.md); it does not imply cloud acceptance has completed.

Local branch `codex/final-demo-product-local`, base main
`58d951cd8c0bb293fe820263894e490027ce1c2c`, uncommitted changes. #247 and its network
implementation remain on the previous branch, not included in this work.

No push, PR, merge, deployment, AWS mutation, DNS change, real OAuth/key reading,
or paid model call. Existing kind and shared EKS resources were not modified.

## Implemented behavior

### Compact failure banner follow-up

- Failed Work renders actual refusal facts below the title, before Permissions,
  as `<target> blocked` plus `Evidence` in the approved amber banner. No arrows,
  repeated explanation or invented denial. Success keeps the quiet refusal list;
  generic failures without refusal facts do not acquire a blocked banner.
- Build/contracts/typecheck, 161 UI unit tests, 83 browser smoke tests and nine
  actual local API/PostgreSQL/MCP journeys passed. Three new browser cases cover
  failure/success placement, no-refusal failures, evidence navigation and narrow
  widths. Actual negative journeys retain zero downstream ProviderAttempt facts.
- Rendered proof: `.tmp/ui-smoke/connected-recorded-refusal-a32c6-for-failed-Work-failed-true/failed-refusal-banner.png`
  and `.tmp/product-acceptance/missing-permission-web-res-0ca45-er-attempts-no-false-result/failed-web-task.png`.
- First check attempts in the sandbox hit denied cache/loopback/hard-link
  operations; reruns use workspace-local D: caches and the approved local test
  execution outside that sandbox. No cloud or backend implementation changes.
- Final unsandboxed `scripts/check.ps1 -All` passed docs/contracts/format/tidy/vet
  and all Go packages except the already recorded Windows
  `TestOwnerBootstrapDeliveryGuards` invitation-delivery failure. That existing
  sensitive file/test was not modified or skipped. The original full gate is
  therefore not green. Final `-Docs` and `git diff --check` passed separately.

### Permission-first Work details and missing-permission correction

- Actual connected details now follows the focused mock. Requested fields only;
  matching values are identical black/normal weight, only requested-but-ungranted
  values are gray. Pending, unknown and execution refusal are not conflated with
  issuance denial. Historical Policy and caller remain authoritative for display.
- Context is secondary, results unchanged, activity/evidence expandable. Existing
  polling, nested turn keyboard controls, failure links, clipboard references,
  access comparison and interruption/cleanup behavior remain tested.
- The local Researcher attempts the read required by a supplied URL even if its
  effective catalog is empty/unrelated. The real Gateway refuses missing tool or
  scope; the agent fails without a provider/default-model answer. This is scoped
  to local Researcher execution, not a new platform semantic rule. Coding agents
  retain catalog-first behavior even when their objective includes a URL.
- Independent downstream-provider counters prove zero execution on a real
  Gateway denial; actual SQL/API journeys prove one refusal, zero ProviderAttempt,
  Failed and no result for missing tool, mismatched scope and empty requests.
  These are Gateway tests, not native Pod network enforcement or real-LLM quality.
- The owner's historical false-success Work is not repaired in place. It remains
  evidence of the original bug; fresh requests prove the correction.

Current checks: 161 UI unit tests and 80 browser smoke tests passed; all nine
actual API/SQL/MCP journeys passed again against the final local binary (1.3m).
The complete repository `-All` gate passed documentation, architecture,
formatting, module tidy and vet; its Go suite failed only the already documented
Windows `TestOwnerBootstrapDeliveryGuards`. No invitation/secret-file writer was
changed to hide this failure. One earlier run under concurrent load failed the
existing short-deadline network-lifetime fixture; it then passed three isolated
repetitions and the complete `-All` Go suite. Final serial full-Go regression
with only the explicit existing Windows bootstrap exclusion, followed by vet,
also passed. Earlier four browser
failures were old tests interacting with newly collapsed activity; tests now
open it first and retain keyboard/polling/assertions, not remove the checks.

### Approved design follow-up: Researcher and business MCP

- Agents now uses a server-filtered catalog, project/tool checkbox facets,
  query search and a details drawer. Same-name project registrations stay
  project-qualified. Inspection is not an execution grant; viewers retain Run
  but real submission is denied. Revoking research access removes its directory
  entries and facets from a new authenticated response.
- Administration defaults to a user summary table and per-user editor; separate
  project saves remain independent. Owner's organization-wide configuration
  access is shown without implying business Policy bypass.
- Platform uses six balanced cards and safe explicit projections. Ordinary
  members receive no installation connection topology or endpoint addresses.
- research/researcher is registered only to the new project. Existing engineer,
  incident-reader, payments and billing stay intact. New Work presets request
  different tools/scopes under the same caller, template and Policy.
- Existing Go MCP service retains read_file and optionally adds web_fetch and
  crm_read. Real public HTTPS source fetches and real server-side synthetic CRM
  reads pass through the actual Tool Gateway. No new database or credentials.
- Researcher has no public-web ceiling; its Policy explicitly caps network to
  empty. A forged direct-network request fails admission. Native Pod networking
  is a separate gate, not proven by this synthetic local runtime.

### Current evidence boundary

The 5 local browser journeys passed, including old task scopes/Gateway refusal,
Policy Allow → Deny → rollback without restart, anonymous/viewer behavior,
Researcher project binding/access revocation, two real MCP paths, safe platform
projection, search-chip removal, unknown-filter rejection and mobile overflow.
UI unit tests: 154 passed. Canonical checks, typecheck and production UI build
passed. The final 77-case browser smoke suite and all 5 actual product journeys
passed again on the final build, including the restarted binary's embedded
Administration page. Screenshot evidence is in .tmp/product-acceptance.

The independent MCP module passed all tests with public-source verification
explicitly enabled. Negative tests cover unknown IDs, reserved/private IPs,
oversized responses and dataset traversal; existing SDK/file cases remain.
Tool Gateway regression tests separately check deny makes no adapter call.
These are distinct evidence layers, not a single cloud penetration test.

Existing local kind has kindnet/kube-proxy only; it does not enforce
NetworkPolicy. No CNI replacement or shared cluster modification was performed.
Actual raw-Pod HTTPS blocking with positive controls, real model answer quality,
Google sign-in and EKS/Worker compatibility remain release gates.

One full Go regression encountered an unchanged runtime fixture deadline under
high parallel load. Three isolated repetitions passed; the lower-parallel full
run passed runtime/agentsandbox but overlapped a local-preview source edit and
failed importing the newly added os/signal. Focused local-preview/adapter/console
tests passed afterward. These failed runs are retained, not silently called green.
The frozen-source final `go test -p 2 ./... -skip '^TestOwnerBootstrapDeliveryGuards$'
and `go vet ./...` both passed, with only the pre-existing Windows directory-sync
test excluded. No functional test was weakened or removed to obtain this result.
An initial documentation check raced the deletion of a Go build temporary
directory; documentation checks were rerun after all build jobs completed.

The table below describes the earlier accepted local slice; this follow-up
supersedes its 3-journey count and synthetic-tool statement.

- Stable submission intent; uncertain outcomes resolve authenticated member-scoped
  client names to canonical server Work IDs. Identical retries are not new tasks.
- Separate inspection/submission project lists; registered template selection;
  generic tasks; explicit tools/scopes/network and a live requested-access summary.
- Display-only organization metadata; independent per-project grant operations;
  existing operational-role replacement remains explicit and warned.
- Readable Policy subject/caps, preserving absent versus explicit-empty semantics;
  preview differences, immutable versions, atomic expected-active activation,
  structured project audit, history, rollback and exact historical version links.
- Copy editable task intent, never issued authority; compare two authorized Work
  snapshots; retain useful result alongside Gateway refusal; show refusal reason
  and direct evidence, not just an operation name.
- Distinguish Policy/Gateway/native-runtime evidence; never attest packet blocking
  from NetworkActive, timeout, fixture evidence or an absent provider record.
- Read-only OTLP configuration, export-failure count, explicit unknown ingestion,
  safe configured diagnostics navigation and copied Work/Trace IDs.
- Separate example workspace from connected failures. Expanded June landing
  states current open source and planned hosted/catalog/marketplace separately.

## Executed checks

### Keyless public-page and preset follow-up

- Added immutable `researcher-v2` alongside the original Agent. `web.read(url)`
  fetches real public HTTPS through MCP/Gateway; no search provider or API key.
  This is URL retrieval, not keyword search. CRM remains synthetic and local
  runtime/model behavior remains explicitly synthetic.
- Task preset is capability-based and generic. Objectives start empty; examples
  are placeholders. Switching presets preserves typed intent and names. Pointer
  select focus is quiet while keyboard Tab focus remains visible.
- Current UI checks: production build/type/contract passed; 156 Vitest tests and
  79 smoke tests passed. The new actual Go/API/PostgreSQL/MCP journey passed:
  public-page success, cross-scope Gateway refusal, CRM success and distinct
  authority under the same caller/template/Policy. The complete six-journey
  suite passed together after this follow-up (31.4 seconds).
- Full Go regression passed with only the explicitly documented existing Windows
  directory-Sync test excluded; `go vet ./...` passed. MCP tests and real HTTPS
  probes for example.org and jitsi.org passed. Native Pod isolation and real LLM
  answer quality have not been newly verified by these local tests.

| Check | Result / boundary |
|---|---|
| Canonical contract check, TypeScript, production UI build | Passed; final asset `index-vbsm2fKv.js` |
| Vitest | 154 passed, 13 files |
| Playwright smoke | 77 passed; existing auth/landing/June/mobile/access/evidence/polling/history plus new recovery/caps/selection cases |
| Local product Playwright | 3 journeys: actual Go APIs/PostgreSQL; synthetic login/runtime/model/tool data explicitly marked |
| Focused real SQL/API | Policy CAS: one winner/one conflict, rollback and immutable history; independent grants; disabled/foreign access rejection; slash-project history isolation; canonical name recovery and one provider invocation on retry |
| `go test -p 2 ./... -skip=^TestOwnerBootstrapDeliveryGuards$` with disposable PostgreSQL | All packages passed. Existing test excluded explicitly only for the Windows issue below |
| Final control-plane package with same explicit skip; `go vet ./...` | Passed after schema startup probes |
| Documentation / architecture / delivery contracts / Markdown links | Passed |
| Repeatable startup script `-PrepareOnly -SkipBuild` | Passed; verifies exact container ownership and loopback binding, does not download or create cloud resources |
| Desktop/mobile rendered proof | Captured and visually inspected landing, Work refusal, comparison, Administration and Platform |

### Original gate failures, not concealed

`scripts/check.ps1 -All` stopped at the existing Windows
`TestOwnerBootstrapDeliveryGuards`: syncing an opened parent directory returns an
error on Windows. The invitation proof writer and this test were not changed.
The same test binary compiled for Linux passed in an offline, read-only local
container with only its test binary mounted and temporary synthetic files.
This is not an unqualified green Windows `-All` gate or a production deployment
test. The remaining Windows Go tests passed with the explicitly reported skip.

One earlier SQL run transiently failed the pre-existing application-session
capacity test. Its isolated repeated run (3 times) and the later complete SQL/Go
regression passed without altering its checks. Earlier UI-test selector failures
and one stale-build rendering mismatch were corrected/rebuilt and rerun; test
expectations were not removed to hide missing behavior.

## Evidence and owner handoff

[Owner walkthrough](../../docs/development/local-product-acceptance.md) has the
local URL, prepared Work links, Policy publish/rollback instructions and restart
commands. Browser screenshots are in `.tmp/product-acceptance/`; smoke artifacts
in `.tmp/ui-smoke/`. These directories are ignored local output, not published
customer evidence. Re-running browser tests replaces screenshots but retains the
local durable synthetic Work history.

The Policy experiment submitted identical `request.spec` before activation,
after activation and after rollback, asserting Allow → Deny → Allow. Prior Work
policy and authority snapshots remained unchanged. Two tasks under the same
caller/template/Policy produced distinct resource scopes. A deterministic
cross-scope attempt hit the real local Gateway and was denied while the permitted
task completed. It is not an AWS packet-level bypass test.

## Release gates still separate

- Owner local acceptance; normal PR/CI and explicit release authorization.
- Operator migration 010/011, runtime minimal new-table/identity-sequence grants,
  backup and coordinated single writer; schema-011-compatible rollback image.
  The old binary refuses a newer schema. Never delete migration/audit rows to
  force an old-image rollback.
- Real Google sign-in, existing image/model/MCP golden path and Cloud/AWS
  authorization/network lifecycle acceptance. Local synthetic runtime/model
  answers are not replacement evidence.
- No CloudWatch/Grafana deployment in this task. No observability editor, domain
  migration, general organizational hierarchy or marketplace implementation.
