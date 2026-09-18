# #167 installed API / CLI / local Portal verification

Date: 2026-09-17. Dedicated target: `kind-agenova-k8s-lab/agenova-system`, Agent Sandbox v0.4.6, local Ollama `llama3.1:latest`.

After rebasing onto the first #165 review fixes, the branch was rebuilt and loaded into the same dedicated kind cluster, then `platform apply -f deploy/reference/platform.kind.yaml --yes --json` reported `ready: true` at revision `sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f`. A fresh `agenova run -f deploy/reference/demo/work.yaml --json` completed `Succeeded` via Ollama `llama3.1:latest` (362 input / 71 output tokens). A separate `agenova work show investigate-payment-retries --json` returned the same claim ID, `Succeeded` outcome and 48 facts. The denied request returned `Deny`, no Claim, zero model facts and two request facts. With `agenova api connect` and the local Vite server running, the opt-in installed Playwright suite again passed 2/2 tests against those actual records. This rerun did not rely on the pre-rebase screenshots or fixture responses. The later #165 review fixes were rebased without conflict; a further real rerun after the final rebase is recorded below.

## Observed CLI and API flow

1. Built the updated `agenova-control-plane:0.1.0` image, loaded it into kind, and reconciled the installed reference Platform. `platform status --json` reported the same revision with zero changes, `installationReady: true`, provider health `not-checked`, and the `agenova api connect` instruction.
2. `agenova run -f deploy/reference/demo/work.yaml` returned `Allow`, a real Agent Sandbox claim, `Succeeded`, a task-specific Ollama answer, and model tokens. After the CLI process exited, `agenova work list` and `agenova work show investigate-payment-retries` returned the same request reference, claim ID, outcome and answer through the installed API. `work show missing-work` returned nonzero HTTP 404.
3. `agenova run -f deploy/reference/demo/denied-work.yaml` returned `Deny` before a Claim/worker/provider call. The same denied reference appeared in `work list/show` without a Claim.
4. `agenova api connect` printed `Forwarding from 127.0.0.1:8088 -> 8081`; a separate `npm --prefix ui run dev -- --port 5177 --strictPort` served the Portal through a server-side `/api` proxy. The opt-in `npm --prefix ui run test:installed` checked `/api/setup` installed identity/revision, allowed/denied HTTP evidence, list/detail parity, and visible React details; **2 real installed Playwright tests passed**. Screenshots: [allowed Work](installed-allowed-work.png), [denied Work](installed-denied-work.png).

The first real browser run found two integration defects rather than passing on fixtures: the in-cluster Control Plane ServiceAccount lacked `list` on namespaced ConfigMaps for registered-template discovery, and the UI expected uppercase Policy rule fields while the actual Go JSON contract emits camel-case rule fields. Both were corrected, then the same real tests passed. A later failure was only the test expecting the request ID in a collapsed details panel; the test now opens that panel and verifies the ID and claim, matching the intended UI behavior.

## Gates and boundaries

The complete repository gate (`scripts/check.ps1 -All`) passed, including Go, TypeScript, build and 49 fixture browser smoke tests. The real installed Playwright checks are intentionally opt-in because they require a prepared kind/Ollama installation and live request references.

The strengthened installed check also requires `AGENOVA_CLI_PATH` pointing to
the built CLI. It executes separate `work show --json` and `work list --json`
processes and compares their records to the HTTP API, then verifies the Portal
rendering. It asserts real Agent Sandbox/Ollama and cleanup facts for the
allowed run and absence of Claim/model facts for the denied run; see the exact
commands in `ui/README.md`.

Final post-rebase rerun (18 September): the rebuilt #167 image was loaded into the dedicated kind cluster and its Deployment restarted. `platform status` reported the installed reference revision ready with zero changes. The CLI ran `investigate-payment-retries` through Agent Sandbox and `llama3.1:latest` to `Succeeded` with 362 input and 75 output tokens and 48 facts. A separate CLI process listed two requests and `work show` returned the same Claim ID and 48 facts; the other request was `Deny` with no Claim and zero model facts. The local `agenova api connect` tunnel and Vite Portal then passed both opt-in installed Playwright checks against those live records. The tunnel and dev server were stopped after verification.

After rebasing onto the later #166 integrity/timeout fixes (`aa373f9`), we repeated the installed check again on 18 September: rebuilt and loaded the #167 image, restarted only the dedicated demo Deployment, and confirmed `platform status --json` returned revision `sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f`, `installationReady: true`, zero changes, and the local API address. The synthetic Work again returned `Allow`/`Succeeded` with a real Agent Sandbox worker, Ollama `llama3.1:latest`, 362 input / 90 output tokens, and 48 facts. A separate CLI process listed both requests and showed the same Claim ID and 48 facts; the denied request had no Claim or model facts. With the loopback tunnel and Vite up, `npm --prefix ui run test:installed` passed **2/2** actual installed-API browser checks. The repository gate passed with **50** browser smoke tests. The worker SandboxClaim was cleaned up, and both local UI/tunnel sessions were stopped. Ollama token counts are observed outputs, not a deterministic assertion.

Post-review parity rerun (18 September, on #167 rebased onto #166 `1641cb8`): rebuilt and loaded the service image, restarted the dedicated Deployment, and observed original Platform revision `sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f` with `installationReady: true` and `changes: []`. Fresh allowed Work returned `Succeeded`, model `llama3.1:latest`, and 48 facts; fresh denied Work returned `Deny`, no Claim, and two request facts. The strengthened opt-in Playwright suite passed **2/2** with actual `AGENOVA_CLI_PATH` child processes comparing CLI JSON against API JSON, asserting backend/model/cleanup evidence and visible Portal details. The full repository gate also passed (Go, frontend contracts/types/build, **50** browser smoke tests); `kubectl get sandboxclaims.extensions.agents.x-k8s.io` returned `No resources found in agenova-system namespace`. Tunnel and Vite were stopped after the test.

The following review pass tightened those checks: the CLI now invokes canonical
ClaimRequest validation on queried evidence, reports `Finishing` while a
Succeeded Claim lacks its final Work outcome, and the real installed denial
asserts zero Runtime, Model, or Tool activity. Focused Go/type checks, the full
repository gate (50 browser smoke tests), and installed Playwright **2/2**
passed again against the same retained live service records. No fixture or
prose-only substitute was used for the installed suite.

The next review pass also required the CLI to validate non-nil issued state
and its request correlation, map Bound to the Portal's Starting label, print
the executable `npm --prefix ui run dev` command, and require `Deny` as the
installed negative's terminal outcome. Focused Go/type checks and the real
installed Playwright **2/2** passed again after those changes.

The final queried-evidence validation pass rejects malformed or foreign
Fact records and empty terminal Outcome objects before a CLI result is
accepted. Unit negatives cover those shapes, and the real installed
CLI/API/React Playwright suite again passed **2/2** against the allowed and
denied kind/Ollama records with this validation enabled.

After #166 entered main, #167 was rebased onto that squash merge. The latest
review fixes reject facts attributed to a different Claim and outcomes that
contradict the issued phase, align CLI `ApprovalRequired` wording with the
Portal, and make the installed Playwright gate assert the registered Policy
rule and AgentTemplate ceiling. Focused Go tests passed. The complete
repository gate passed with **50** browser smoke tests. Against the retained
real kind/Ollama records, the rebuilt CLI `work list` returned the allowed
Succeeded Claim and pre-claim Deny; the separate local tunnel, Vite server,
and strengthened installed Playwright suite passed **2/2**. The tunnel and
Vite sessions were stopped, and `kubectl get sandboxclaims` found no resources.

The next current-head review prompted three additional correlations at the
CLI evidence boundary: Fact authority must be the issued Claim authority;
top-level and nested decision result/policy must agree; and the terminal
model result must have a matching allowed ModelDecision, ProviderAttempt, and
successful ProviderOutcome for its invocation. Negative unit cases were
added. The rebuilt CLI still queried the actual allowed Work (`Succeeded`,
48 facts, `llama3.1:latest`) and denied Work (`Deny`, no Claim, two facts),
and the installed Playwright suite passed **2/2** with an added assertion
that its final model result matches those three recorded invocation facts.

The following review found six more version-skew integrity cases. Queried
views now reject issued action/request mismatches, invocation facts without
the issued Claim, decision facts contradicting the issued decision, duplicate
or unordered Fact IDs/sequences, unknown JSON fields/trailing data, and the
operator guide now starts Vite on the installed test's port 5177. The journal
uses a global sequence, so gaps between a single Work's facts remain valid.
Negative unit cases cover these responses; the rebuilt CLI continued to
accept the retained actual kind/Ollama Allow (48 facts) and Deny (two facts)
records. The full repository gate passed (50 browser smoke tests), and the
real installed CLI/API/React Playwright suite passed **2/2** using the now
documented 5177 port. The temporary tunnel and Vite server were stopped.

The reference API and UI connection are loopback-only transport, not production user authentication. The service uses a fixed Team A identity; Work/evidence exist in one Pod's memory and disappear on restart. The tool artifact reads are marked mock; Agent Sandbox execution and Ollama inference are real. The UI is a separately started local client, not a `platform apply` installation result.
