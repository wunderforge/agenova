# Local acceptance: Task Access Advisor

Date: 30 Oct 2026. Branch: `codex/task-access-advisor`. No PR, merge, deployment,
AWS request, paid inference, image download or database migration was performed.

## Current owner acceptance — 1 November 2026

Use [the real local preview](http://127.0.0.1:5224/login), with
[the current walkthrough](local-acceptance.md). The old 5221 process/history is
preserved. No PR, merge, cloud request or deployment was performed.

The same `researcher-advisor-v2` executable ran all cases through real local
`llama3.1:latest` inference and the actual Go MCP adapter. Automatic preparation
used the separately configured `qwen2.5-coder:7b` Advisor (736 input / 55 output
tokens for the internal case). Manual requests did not invoke the Advisor.

| Case | Current Work ref | Execution model calls | Tool calls / matching MCP receipts | Goal |
| --- | --- | --- | --- | --- |
| Public, Manual, no caller URL/type | `work-bc71d7ca-1871-47fd-81a7-965b31cea0d8` | 2 | 1 / 1 | Completed; official Python source cited, 59-word answer |
| Internal customers, Automatic | `work-1fcab6dc-5fc1-426b-9bdd-87c0489f7d16` | 3 | 1 / 1 | Completed; C001/C002/C003, synthetic label and follow-up recommendations |
| Same Public goal, only CRM granted | `work-cc3b3e86-4b5c-4403-a579-531f1fdf14bc` | 1 | 0 / 0 | Incomplete, `missing-capability / web.read`; no unrelated customer read |

All three reported cleanup success. MCP tool receipts independently matched the
two actual tool invocation IDs. The negative is an Agent/catalog-unavailability
case, not a fabricated Gateway refusal. The separate
`TestMissingWebPermissionIsDeniedBeforeProvider` exercises the actual Gateway
with a scoped Work identity: Deny, zero provider attempts. Neither is a fresh
Kubernetes or packet-blocking test.

Local identity, customer records and lifecycle/network state remain explicitly
synthetic; model/Agent/MCP execution is real. Work history is in memory. The
reference Agent's own explicit-source checks are not platform Policy or infra
enforcement; other images and optional legacy API types remain supported.

Verification: affected Go packages and `go vet` passed; 178 frontend unit tests,
93 browser regressions, six Advisor/layout cases, and the final three real
business-task cases passed (2.6 minutes). Final worker executable SHA-256:
`71133708EC2074408F053708A9D4D3F57A3E4BFEE0D12CC0D4B9EB57BDFD275A`.
Read-only rendering review waited
for terminal UI results rather than photographing an earlier Running snapshot.
The pre-existing Windows `TestOwnerBootstrapDeliveryGuards` full-check failure
remains visible and is not counted as green. No database restart or cloud proof
is claimed.

Reproduction logs/artifacts are under `.tmp/repair-final-joint.log`,
`.tmp/repair-final-all.log`, `.tmp/repair-owner-acceptance.log`,
`.tmp/repair-public-concise.log`, `.tmp/repair-render-review.log`,
`.tmp/repair-selected-evidence.json` and `.tmp/live-researcher-render-review`.
Earlier unsuccessful model/formatting runs remain in separate `.tmp/repair-*`
logs and saved histories; they were not rewritten as successful evidence.
Final owner approval is pending. Task preset placement remains a separate open
decision.

## Original Advisor-only preview — 30 October (historical)

From the repository root, in PowerShell:

```powershell
$env:GOCACHE = Join-Path (Get-Location) '.tmp/gocache'
$env:GOTMPDIR = Join-Path (Get-Location) '.tmp'
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:TEMP
npm --prefix ui run build
go build -o .tmp/access-advisor-preview.exe ./tools/local-product-preview
.\.tmp\access-advisor-preview.exe --volatile --listen 127.0.0.1:5221 --advisor-config harness/examples/access-advisor/advisor-model.json
```

Open <http://127.0.0.1:5221/login>. It supplies an explicitly synthetic local
identity, project `research` and template `researcher-advisor-v1`. No Google login
or cloud connection is needed. Existing `qwen2.5-coder:7b` must be served by Ollama
at `127.0.0.1:11434`; nothing downloads a model. This preview does not alter the
existing preview on port 5210 or cloud Researcher registrations.

The Advisor performs real local inference. Task execution, business tool data and
runtime are synthetic, and Work history is memory-only. The header discloses this.
Docker was nonresponsive and the previous disposable PostgreSQL listener absent;
neither Docker nor other services were restarted. The existing PostgreSQL preview
path is retained, but a real database restart was not exercised this round.

### Suggested request

1. Choose **Suggest permissions** and generic **Investigation**.
2. Enter: `Read https://jitsi.org/security/ and summarize the security features from that public page.`
3. Click **Suggest permissions**. Verify `web.read`, `web:public-pages`, the Agent's
   default model profile, no direct network and the actual local planner model.
4. No Work exists yet. Edit if needed, then **Start work** to test the synthetic
   execution flow. Details show requested/granted access and **Permission preparation**.

### Fully automatic request

1. Create a fresh Work. Choose **Automatic**, generic **Investigation**.
2. Enter: `Use the internal customer-retention dataset to identify three customers with the largest usage declines. Draft follow-up recommendations without sending anything.`
3. Click **Plan and start work** once. No second confirmation is needed.
4. Verify `crm.read`, `crm:customer-retention`, no `web.read` or direct Pod egress,
   and the separate planner provenance in **Permission preparation**.

### Manual/upstream regression and negatives

- Choose **Manual**, explicitly select one tool/scope and submit. Planning attempts
  shown in Platform must not increase; no planner provenance is invented.
- Obtain a suggestion, then change the task objective/type/runtime. The old review
  disappears; Suggest mode cannot start until a new suggestion is obtained.
- For deterministic clarification use `advisor-rules.json`, Investigation and an
  unsupported task: it asks to choose a supported type, without allocating Work.
  Model semantics are probabilistic; ambiguous wording is not a security boundary.
- Automated HTTP tests cover cross-user receipt access, changed guide/context,
  disabled automatic mode, revoked session, current Policy removal and retry.
  Policy removal proves zero runtime/tool calls. A stored-evidence test simulates
  fresh-process recovery without inference; it is not a real PostgreSQL test.

## Executed gates

- Guide/config/platform/CLI and existing affected Go package regressions.
- Six browser cases using real local Go APIs and deterministic rules: Manual,
  Suggest/review/confirm, Automatic CRM, clarification/stale-review reset.
  Desktop (1440px) and mobile (390px) verify task-first order, nearby permission
  controls, mode switching without losing task text, and no horizontal overflow.
- Three opt-in browser tests with actual `qwen2.5-coder:7b`: same generic task type,
  public objective selects `web.read`; customer objective selects `crm.read`.
  Both returned real planner metadata/token counts and no direct network request.
  The third verifies one-click model planning plus automatic submission, with no
  intermediate confirmation, and separate planning provenance in Work details.
- Real-model tests initially exposed an unnecessary source-confirmation question.
  The prompt now limits clarification to permission-relevant missing context;
  both unchanged tasks passed after the correction.
- UI contracts/typecheck/production build passed; all 178 component tests in 16
  files and all 90 existing browser regression tests passed (`npm --prefix ui run check`).
- Final affected Go run: all seven packages passed (API, Advisor, console,
  platform, CLI, evidence, registration), including tool/Policy narrowing and
  accepted-Work recovery without a new inference.

Reproduce rules E2E:

```powershell
npx --prefix ui playwright test --config ui/playwright.advisor.config.ts --grep-invert 'real local model'
```

Reproduce real-model E2E (requires existing local Ollama, no paid model):

```powershell
$env:AGENOVA_TEST_LOCAL_ADVISOR = '1'
npx --prefix ui playwright test --config ui/playwright.advisor.config.ts --grep 'real local model'
Remove-Item Env:AGENOVA_TEST_LOCAL_ADVISOR
```

Screenshots and traces are generated under `.tmp/advisor-browser/` (rules) and
`.tmp/advisor-browser-model/` (real model). Inspect the public/CRM real-model
screenshots there. Test artifacts contain synthetic tasks
and local planner metadata only; they are not part of the feature commit.

## Known limits

One closed access profile per task; no union/composition, optional-permission
fallback, durable rejected-plan audit, persistent financial quota, live Platform
config reload or cloud acceptance in this delivery. Limits/cache reset when the
control-plane process restarts. Existing authorization/enforcement is reused.

`scripts/check.ps1 -All` reached Go tests and failed only
`cmd/agenova-control-plane: TestOwnerBootstrapDeliveryGuards`. The unchanged
owner-invitation writer calls directory `Sync`, which fails on this Windows host.
Documentation, contract checks, module tidy and `go vet ./...` passed. This gate is
not reported as green or skipped. The rest of the Go package run passed. The
separate frontend check continues despite this unrelated full-check stop.

Optional controlled/live integration entry points and the `agenovaprobe` command
compiled locally; `go test -tags agenovaprobe ./internal/console/` passed. These
were compile/local tests, not real Kubernetes or cloud runs.

## Task-first UI refinement

30 Oct owner walkthrough: reorganized New Work into task context and objective,
Permissions, Run settings, then actions. Permission setup now lives inside
Permissions, immediately above its dependent controls. Removed the repeated
sidebar and inline instructional prose. Suggested permissions are shown once as
editable fields; the explanation is available in a disclosure. Configuration
metadata is also optional to expand. Existing Policy/Advisor submission contracts
are unchanged. Production build and all 178 frontend unit tests passed; screenshots
are recorded in the layout cases under `.tmp/advisor-browser/`. The existing
90-case frontend browser regression also passed for this refinement. All six
Advisor/layout cases passed; final neutral-focus desktop/mobile screenshots were
inspected. No backend or authorization contract changed. The existing port 5221
process was kept running, so in-memory Work history was not reset for this UI update.
