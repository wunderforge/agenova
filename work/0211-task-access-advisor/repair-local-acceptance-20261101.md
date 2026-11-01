# Researcher repair — local acceptance, 1 November 2026

Status: developer verification complete; owner acceptance pending. Feature branch
`codex/task-access-advisor`. This batch is local-only; no PR, merge, cloud changes or deployment.
5225 and its original records are retained. The new volatile preview is at
[5226](http://127.0.0.1:5226/login), Template `researcher-advisor-v4`, project `research`.

## What changed

- `web.search` performs a real query against a replaceable keyless Bing RSS provider.
  `web.read` reads public HTML, text and JSON, validates public destinations at dial
  time and on redirects, tries public IPv4 before unavailable IPv6, and returns
  bounded useful excerpts with URL/time/truncation metadata. Search and read remain
  independent permissions; older Work grants do not gain either automatically.
- A new opt-in reference-Agent protocol uses a compact action grammar, readable
  observations and explicit accepted/rejected feedback. Current facts and explicit
  website research require observations. Ordinary knowledge/calculations can
  finish without an unrelated tool. The reference image carries source guidance;
  task wording never chooses authority in the platform.
- Successful answers are not rejected merely for omitting a verbatim URL. The
  exact source/arguments/result remain inspectable in the tool record. A failed
  source can produce a useful partial/incomplete answer without crashing the run.
- Known MCP failures retain safe code, stage, HTTP status and size bounds through
  the adapter, Agent and UI. Unknown remote error text is not copied as a diagnosis.
- Activity shows real model output, model-reported decision summary, actual
  provider thinking when returned, tool arguments/results, Gateway decisions,
  explicit Agent feedback, tokens and available provider timing. The Work's
  requested/granted authority layout is unchanged.
- Optional host-only local model endpoints keep Advisor and execution independently
  configurable. No mandatory image/provider change for existing clients.

## Actual local composition and limits

The existing `Qwen3-4B-Instruct-2507-Q4_K_M.gguf` and llama.cpp runtime on D: are
reused, alias `local-qwen3-4b`, loopback `8080/v1`, one inference slot, context 4096.
No model was downloaded. Old Ollama inference models remain installed; only this
run's completed inference cache was unloaded before starting the smaller model.
Do not run competing image generation or an old Ollama Work concurrently on the
4 GB GPU while accepting this preview. Old 5225 records can still be read.

The API/Worker/MCP paths and model calls are real. Identity and Work network
lifecycle are explicitly local synthetic composition; CRM is synthetic sample data.
This is not a fresh Kubernetes packet-enforcement or production deployment test.

Raw content is explicitly enabled only for this preview: bounded volatile store,
16 MiB/4096 entries, 24-hour retention, 12 KiB per text field, credential-pattern
redaction. References are random, Work-scoped and checked against current read
access. Lists carry references, not bodies. Restarts/expiry can make content
unavailable; historical bodies are never reconstructed. Known credential shapes
are masked; this is not a general PII anonymizer. Production capture stays off
unless an operator supplies an ActivityStore.

The selected Qwen model is non-thinking; it has no separate hidden-thinking field
to display. Decision summary is its actual emitted explanation, not a recovered
internal chain of thought. Fixtures test provider-returned thinking separately.

Bing RSS and public websites are best-effort external services, not an SLA. A
challenge, empty results, rate limit, JS shell or source failure is reported as
such. The HTML parser is pinned to x/net v0.57.0 in the separate MCP module,
which fixes the relevant parsing issues and supports this repo's Go 1.25; no HTML
Render output is injected into the browser.

## Owner walkthrough

1. Open [New Work](http://127.0.0.1:5226/?mode=connected&project=research&template=researcher-advisor-v4#/work/new).
   This preview uses its labelled synthetic local owner, no Google sign-in needed.
2. Automatic permissions; enter `When is Chinese New Year in 2027?` and submit.
   Inspect requested/granted `web.search` and `web.read`, then Agent activity.
3. For current weather enter `What is the weather in Melbourne today?`.
   Check the actual tool URL, returned measurement/time and final answer together.
4. Internal task: `From our internal customer data, find three at-risk customers
   and suggest next steps.` Suggest can show the draft before submission. Only
   `crm.read` / `crm:customer-retention` should be selected. Findings must be
   labelled synthetic.
5. Keep a public goal, choose Manual, request only `crm.read` /
   `crm:customer-retention`, and submit. The supported public tool remains visible
   to the Agent. Inspect its actual Gateway denial and incomplete answer; MCP
   must have no matching tool receipt. Do not treat a model voluntarily refusing
   without an attempted call as this proof.
6. In Activity open a turn and choose **Inspect** on **Model request** for the
   actual response/summary; choose **Tool call** for parameters/result; choose
   **Tool access** on the denied row for the Gateway fact. Earlier failed calls
   stay visible even if a later call succeeds. No separate inference is used to
   invent an explanation.

The source reader does not require search if the Agent already knows a useful
URL. Removing only `web.search` while keeping `web.read` can legitimately permit
public reading; remove both to test absence of all public-tool authority.

## Reproduce

From the repository root, use the installed Go/Node 24 tools and D: Go caches.
Build UI with `npm --prefix ui run build`; build the reference executable with
`go build -ldflags="-X main.agentRole=researcher" -o .tmp/researcher-repair-v4.exe ./cmd/agenova-demo-worker`.
Build `tools/local-product-preview` to `.tmp/researcher-preview-v4.exe`, and build
`harness/integration/mcpfixture` (its separate Go module) to `.tmp/researcher-mcp-v4.exe`.

Start the already-installed model runtime on loopback with its existing GGUF,
`--alias local-qwen3-4b --ctx-size 4096 --parallel 1 --n-gpu-layers 99 --jinja`.
Then start the preview with these explicit local options (do not replace 5225):

```text
--listen 127.0.0.1:5226 --volatile --researcher-repair
--advisor-config harness/examples/access-advisor/advisor-model.json
--advisor-model local-qwen3-4b --advisor-endpoint http://127.0.0.1:8080/v1
--execution-model local-qwen3-4b --execution-endpoint http://127.0.0.1:8080/v1
--live-worker .tmp/researcher-repair-v4.exe
--live-mcp .tmp/researcher-mcp-v4.exe --live-mcp-listen 127.0.0.1:5230
```

Only restart processes belonging to this preview. Ports already owned by another
process must cause startup to fail rather than silently reuse its health response.
Advisor planning remains capped at 24 calls per preview process; existing Work
records and raw content here are deliberately volatile.

Explicit real inference gate:

```text
py -3 tools/local-product-preview/repair_acceptance.py --execute --case benchmark
py -3 tools/local-product-preview/repair_acceptance.py --execute --case failure
```

The 10-case mixed benchmark includes automatic public tasks, explicit/manual
Python, suggest/confirm CRM and withheld public access twice. It stops on failure,
checks answers independently of execution status, and verifies the MCP's own
receipt log for denied invocation IDs. Raw evidence stays under ignored
`.tmp/repair-acceptance/`; no tokens/cookies are printed or persisted by the gate.

Read-only UI gate after creating the records:

```text
node ui/node_modules/@playwright/test/cli.js test --config ui/playwright.researcher.config.ts
```

See [the script](../../tools/local-product-preview/repair_acceptance.py) and
[UI cases](../../ui/local-acceptance/researcher-repair.spec.ts).

## Verification results

Final local candidate, 1 November, using the real installed Qwen model and real
public sources. Ten consecutive mixed cases passed, including manual,
suggest/confirm and automatic submission. Every case used two **execution** model
calls; Advisor calls are separate. There is no fixed-answer cache.

| Case | API end-to-end seconds | Execution model calls | Work |
| --- | ---: | ---: | --- |
| Sydney weather 1 | 15.97 | 2 | work-8f996c5f-2e74-4b1e-bf75-135f990cc9e9 |
| Calendar 1 | 14.25 | 2 | work-f4261852-71c0-4c97-ac8b-f765c2a4562d |
| Python 1 | 11.75 | 2 | work-31bd959b-8428-40fa-8cdc-deda453692fd |
| Customers 1 | 13.95 | 2 | work-fc73f819-27e6-4a6b-9224-c6ec8efbaaf6 |
| Public tools denied 1 | 7.12 | 2 | work-188c8998-24b7-49fb-ad75-77939ed19f63 |
| Sydney weather 2 | 17.31 | 2 | work-0b85a562-10d3-47a4-be92-d886e57345e7 |
| Calendar 2 | 14.35 | 2 | work-c95c19a7-a60e-4265-ac5f-5098b2605619 |
| Python 2 | 12.56 | 2 | work-b68f9689-4334-45f5-95f8-1f58d58509b8 |
| Customers 2 | 14.48 | 2 | work-29ecbe96-20d1-4280-937e-73b6328c1245 |
| Public tools denied 2 | 7.11 | 2 | work-8d825bce-c7ef-4c4f-9c81-1be5b0dfc25d |

P50 **13.95 s**, P95 **17.31 s** (nearest rank, warm model). These measurements
include permission planning/confirmation when applicable, submission, polling and
retrieval of the raw activity content; they exclude browser rendering. Model cold
startup was about 4.3 s separately. Normal llama.cpp prefix/KV reuse was enabled;
unavailable queue/cache-hit metrics are not invented.

Two additional actual browser submissions measured click-to-rendered-result:
calendar **15.23 s**, customers **14.33 s**, both Automatic and completed with
real tool evidence. These are two samples, not a browser P95 claim.

Additional real checks:

- Melbourne weather: **16.02 s**, real city JSON read, 19 C and conditions from
  that response; two execution calls. Work `work-6c75d6dc-799a-4bff-beaa-f963c52f848a`.
- Missing public page: **7.50 s**, actual source HTTP 404 preserved as
  `source-unavailable`, stage `response`, including through the provider Set's
  sanitizing boundary. Work `work-a9330857-ef9b-4126-87b0-c41725fcbd7f`.
  The Agent reports the page inaccessible and finishes normally; the failed call
  remains visible. A candidate guessing page status before any read is rejected.
- Both denied cases have a genuine Gateway Deny, zero tool-provider attempts and
  **zero matching tool receipts in the MCP's own log**. They finish with an
  honest `incomplete / tool-denied` answer, never an invented successful result.

Regression and UI evidence:

- Relevant Go packages and separate MCP module tests pass; documentation,
  architecture boundaries, formatting, root module metadata and `go vet ./...`
  pass. Full repository Go testing still fails only the unchanged Windows
  `cmd/agenova-control-plane/TestOwnerBootstrapDeliveryGuards`
  (`member_auth_test.go:345`, owner invitation-file delivery). That existing
  failure is not waived; the full check is **not green**.
- **185** frontend unit tests pass; contracts/typecheck/build pass.
- **95** standard browser smoke tests pass. Four live local acceptance browser
  tests also pass against the current build, covering the form, raw response/tool
  inspector, real denial and handled HTTP 404. Desktop and 900 px layouts were
  visually inspected; no horizontal overflow. A final content-loading race test
  also covers switching records before a prior response arrives.
- Local ignored evidence: `.tmp/repair-benchmark-final.log`,
  `.tmp/repair-acceptance/`, `.tmp/repair-source-failure.log`,
  `.tmp/repair-melbourne.log`, `.tmp/repair-click.log`,
  `.tmp/repair-ui-unit-final.log`, `.tmp/repair-ui-smoke-final.log`,
  `.tmp/repair-ui-live.log`, `.tmp/research-repair-full-check-final.log`.
  Inspected screenshots: `.tmp/researcher-ui/model-response-panel.png`,
  `source-failure-desktop.png`, `tool-observation-narrow.png` and
  `gateway-denial-header.png` in that same directory.

Source availability and unrestricted question quality remain dependent on the
chosen model/provider. These scoped results do not establish universal research
correctness, a paid search SLA, or Kubernetes network isolation. Owner acceptance
is the next gate. No PR, merge or deployment was performed.

## Practices used

- [Ollama structured output](https://docs.ollama.com/capabilities/structured-outputs):
  ground the action shape and parameter meaning in the prompt, in addition to
  constrained syntax. Validate semantics after decoding.
- [Official Qwen model card](https://huggingface.co/Qwen/Qwen3-4B-Instruct-2507):
  this version is non-thinking. Model selection here is based on local runs,
  not on published benchmark scores.
- [OpenTelemetry GenAI content](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md#capturing-instructions-inputs-and-outputs):
  explicit collection and separately authorized content references.
- [Go parsing advisory](https://pkg.go.dev/vuln/GO-2026-5028): pin a compatible
  parser fix rather than exposing an old indirect dependency to arbitrary HTML.
