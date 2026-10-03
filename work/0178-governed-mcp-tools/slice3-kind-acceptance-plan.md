# E16 Slice 3: installed kind acceptance plan

Status: revision 3, accepted for implementation after independent review rounds 1–3 (findings in [section 10](#10-review-record)). Nothing in this plan has been executed on kind, and it does not change the Epic's completion state.

This plan refines the Slice 3 Todo in [task.md](task.md#execution-todo). The packet's [acceptance criteria](task.md#acceptance-criteria), [evidence requirements](task.md#evidence-required), [negative cases](spec.md#negative-cases) and [verification strategy](design.md#verification-strategy) remain authoritative. Where this plan and the packet disagree, the packet wins and this plan is corrected.

Goal: move Slice 2 from local interoperability to installed acceptance. The unmodified production control plane, a real Agent Sandbox worker, Ollama and the independent MCP fixture complete one verifiable task, and the five governance rejections are proven against complete server logs with zero tool calls.

## 1. Baseline (checked 2026-10-03)

- Branch `codex/e16-slice1`, HEAD `94a8481949be40ec0ec35cbb903414ee51c2c55c`, synced with origin. PR #192 is Draft with green CI. `main` is still `c56ba3a`.
- Slice 2 delivered the Streamable HTTP client, the D2 fixture (`harness/integration/mcpfixture/`), the per-Work catalog, `resultRef`, S2 attempt/outcome completion and S3 concurrency limits. Focused and local interop logs exist in `docs/evidence/178/`. None of that is installed evidence.
- Fixed endpoint: `http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp` (`fixtureMCPHost`, `internal/adapters/bundled/mcp.go:23`). Fixture namespace is `agenova-e16`. The exact-host exception stays as is.
- Fixture dataset: `README.md`, `logs/timeout.log`, `src/retry.txt`, `logs/slow.log` (35s server delay, above the 30s timeout ceiling), `logs/full-trace.log` (73,744 bytes, above the 65,536-byte response cap), `notes/incident-timeline.md` (11,239 bytes, above the 4,096-byte observation cap).
- Relevant code paths at this HEAD:
  - Per-Work operation handler in `Service` (`internal/console/service.go:297-330`): claim-ID and call-context check, then `app.RequireRunningClaim`, then `toolParameter`, then the Tool Gateway.
  - `toolParameter` (`service.go:411`) checks the installed route and its parameter allowlist before the Gateway. It does not decide authority.
  - `Service.executeMu` (`service.go:45`) and `app` `runMu` (`internal/app/run_service.go:267`) each serialise Work within one service instance.
  - The run lifecycle cancels the work context on every terminal transition (`run_service.go:377` onwards).
  - `buildInstalledTools` is unexported in `package main` (`cmd/agenova-control-plane/tools.go:23`). `Service.journal` is an unexported concrete `*facts.Journal` (`service.go:53`).
  - Fixture `/healthz` is served outside the receipt middleware (`harness/integration/mcpfixture/main.go:132`).
- Reference demo inputs (`deploy/reference/demo/*.yaml`) still grant `git.read` and cannot be reused unchanged. Work narrows access with `requestedAccess.tools` and `requestedAccess.resourceScopes` inside the AgentTemplate `capabilityCeiling`.

### Environment state

| Item | State | Action |
| --- | --- | --- |
| Docker | daemon not running; `~/.docker/run/docker.sock` missing | Start Docker Desktop (Tom) |
| kind | context `kind-agenova-k8s-lab` configured; liveness, ownership and cluster-to-Ollama reachability unverified | Confirm in Phase 0 before any deploy |
| Go | default 1.27.1; Go 1.22.12 toolchain cached and executes | Root-module gates with `GOTOOLCHAIN=go1.22.12`; the fixture module keeps its own toolchain |
| Node | default 26.10.0; Node 24.21.0 installed via Homebrew `node@24` | Prefix `$(brew --prefix node@24)/bin` on `PATH` for UI gates |
| Ollama | `llama3.1:latest` present, matching `platform.kind.yaml` | Check reachability from inside the cluster in Phase 0 |

The macOS external-linking workaround used for Go 1.22 focused tests on this host must not be carried into Linux container builds.

## 2. Scope and boundaries

In scope: the installed positive run, N1–N5 against the real server, the failure matrix, CLI/API/Portal parity for MCP invocations, and the code gaps in section 4.

Unchanged boundaries:

- Backend-neutral contracts and Gateway ownership of authority. The service still checks only the installed route and allowlisted argument before the Gateway.
- No production API, flag or route for fault injection or probing. The production binary contains one inactive append function value (G2) that always points at `journal.Append`; the API that replaces it compiles only under the `agenovaprobe` build tag.
- No root `go.mod` or CI toolchain change. No CI job for the fixture.
- E14 tokens and worker authentication (#197, formerly #121), hostile-worker network bypass (E15), EKS and durable storage (E18) keep their current owners.
- PR #192 stays Draft. Slice 3 passing does not close #178; Slice 4 is still required.

## 3. Pass tiers

The packet requires real-server proof for the five zero-call cases and an explicit, distinguishable result for every failure case. *Additional* items may slip to a later campaign without blocking Slice 3, provided they are recorded as not executed.

| Case | Layer | Tier |
| --- | --- | --- |
| Positive run (production binary) | kind | Required |
| N1–N5 zero-call | kind, real server, probe composition | Required |
| N6 timeout (`logs/slow.log`) | kind + deterministic | Required |
| N7 hard response cap (`logs/full-trace.log`), plus SSE/chunked boundaries | kind + deterministic | Required |
| N8 soft truncation (`notes/incident-timeline.md`), visible in evidence | kind + deterministic | Required (criterion 6) |
| N9 invalid config / unsupported tool | deterministic and `platform validate` | Required |
| N10 connection failure (`TestMCPClientUnreachableServerIsUnavailable`, `mcp_client_test.go:239`), malformed reply, session, version, `isError` | deterministic | Required |
| N10 unreachable endpoint on kind (temporarily break the Service selector, then restore) | kind | Additional |
| N11 composition (`tool_provider_test.go:165`, provider double plus injected outcome failure) | deterministic | Required |
| N11 server-backed: outcome append fails after a successful call; exactly one `tools/call` receipt with a successful handler result, no replay | kind, probe composition | Required |
| N12 injected instruction text | deterministic forged-action probe; a real run only shows the text is labelled untrusted | Required (deterministic) |
| Admission Deny (no claim, no worker, no calls) | kind, CLI/UI regression | Required, never a substitute for N1 |

## 4. Code and configuration gaps

All of these land on `codex/e16-slice1` as separate commits.

### G1 Truncation is not visible in evidence (blocks criterion 6)

`toolbackend.Result` and `workerprotocol.Reply` carry `Untrusted`/`Truncated`, and `internal/console/tool_provider.go:95-99` prefixes `[TRUNCATED]` for the worker, but `facts.Fact` (`internal/facts/journal.go`) has no such field. Add one optional `truncated` boolean, set only on successful `ProviderOutcome` facts, and propagate it through fact projection, the connected-client validator, API decoding, Portal rendering and their tests. Legacy facts without the field still decode; an absent field is never presented as proof that an observation was complete. Raw observation text is not persisted. Regenerate shared contract output and run the import-boundary check.

### G2 Journal fault seam, gated by build tag

- Add one unexported per-`Service` append function value, defaulting to `s.journal.Append`, used by every tool-path append: the ToolDecision observer, ProviderAttempt and ProviderOutcome.
- Add `internal/console/probe_hooks.go` with `//go:build agenovaprobe`, exporting a narrowly named function that replaces that value on a given `Service`. Console `_test.go` files are not visible to another package's test binary, which is why the bridge is a tagged non-test file.
- Production images are built without the tag. Separation is proven from build inputs and build information, not from symbols: `go list -f '{{.GoFiles}}' ./internal/console` without the tag must exclude `probe_hooks.go`; the control-plane binary is copied out of the deployed image and `go version -m` must show no `-tags` setting containing `agenovaprobe`; the probe binary must show it. The reference Dockerfile strips symbols (`-s -w`) and dead-code elimination can drop an unused bridge, so `go tool nm` is supplementary only and "no symbols found" is never a pass.
- Probe hooks are installed before any Work starts on that service instance and never changed while Work is executing.
- This does not reshape the E18 storage contract.

### G3 Probe composition

- Probe tests live in `cmd/agenova-control-plane` as `_test.go` files with `//go:build agenovaprobe`, so they can call `buildInstalledTools` and the G2 bridge. They are compiled with `go test -c -tags agenovaprobe` into a separate probe image and run as an in-cluster Job, which reaches the fixed cluster DNS endpoint. Manifests and orchestration live under `harness/integration/e16/`.
- A test Executor captures the real per-Work operation handler and injects operations through it, bypassing only the model's hidden catalog. No authorisation logic is copied or substituted.
- Every probe has a driver case ID independent of any invocation ID, because some rejections (N3, N4) happen before the Gateway issues one. Receipts go to `probes.jsonl`: case and sub-case ID, claim IDs, invocation ID if any, start/end time, expected rejection point, observed error and observed facts.
- **N3.** `executeMu` and `runMu` both serialise Work inside one service instance, so one runner cannot hold two concurrently Running claims. The probe composition builds two independent service instances (each with its own runner and journal) on the same installed tool set. Claim A runs in instance A and claim B in instance B, both reaching Running through the normal lifecycle; their authoritative snapshots are recorded as provenance. B's authority grants the injected operation, which removes B's own grant as an explanation for the rejection. Each runner owns a separate state map (`run_service.go:205`), so A's reader never recognises B: this proves only that A's handler rejects a non-matching claim ID. It does not prove shared-service cross-claim authorisation or authenticated isolation. The operation nominating B is injected into A's captured handler and must be rejected at the claim-ID check, with no fact attributed to B in either journal. Matched-claim controls (A's operation through A's handler, B's through B's) run outside the zero-call window and must succeed.
- **N4.** The lifecycle cancels the work context on termination, and the handler checks the call context before `RequireRunningClaim`. A probe that reuses the original context would pass on cancellation, not on terminal state. Terminal probes therefore call the captured handler with a fresh, live call context and assert the error comes from `app.RequireRunningClaim` for each of Succeeded, Failed and Expired.
- **N5.** The G2 bridge fails the ToolDecision append (N5a) or the ProviderAttempt append (N5b). The receipt records which append failed.
- **N11 server-backed.** Runs in the in-cluster probe Job against the deployed fixture, because the MCP adapter accepts plain HTTP only at the fixed cluster DNS endpoint (`mcp.go:75`) and production endpoint validation is not relaxed. The call succeeds and the G2 bridge fails only the ProviderOutcome append. Pass requires an explicit evidence failure, exactly one `tools/call` receipt for the invocation with a successful handler result, and no replay. The `initialize`, `notifications/initialized` and session `DELETE` receipts of that one session (`mcp_client.go:113` onwards) are expected and recorded, not counted as calls.
- The evidence manifest records the probe binary SHA, build tag, replaced seam and exact command, and keeps probe results separate from the production positive run.

### G4 One Platform revision with two routes

`spec.md` R1 allows one profile per (operation, resource scope) pair and forbids duplicates, and `toolParameter` rejects any file outside the profile's allowlist before MCP. One profile therefore cannot serve both the positive task and the fault files. Use one backend and two `repo.read` profiles in a single Platform revision for the whole campaign:

| Profile | Operation | Resource scope | Allowed values |
| --- | --- | --- | --- |
| `e16-fixture` | `repo.read` | `repo:agenova/e16-fixture` | `README.md`, `logs/timeout.log`, `src/retry.txt` |
| `e16-faults` | `repo.read` | `repo:agenova/e16-faults` | `logs/slow.log`, `logs/full-trace.log`, `notes/incident-timeline.md` |

Backend config: `timeout` 5s, `max-response-bytes` 65536, `max-observation-bytes` 4096, `max-concurrent-calls` recorded as applied.

- The AgentTemplate (new ID/version) has a ceiling of `repo.read` on both scopes. The Policy (new ID/version) admits both.
- The positive Work requests only `repo:agenova/e16-fixture`, so its catalog never shows a fault file. Fault Works (N6, N7, N8) each request only `repo:agenova/e16-faults`. Each Work has a unique `requestRef`.
- Fault Works advertise all three fault files. Each fault case asserts the intended file in its tool call and the expected failure or truncation result; a run where the model reads a different file is rerun, not counted.
- N2a uses a claim granted only `e16-fixture` and nominates the configured `e16-faults` route with an argument valid for that route (`notes/incident-timeline.md`), so `toolParameter` passes and the rejection comes from the Gateway scope check (`internal/toolgateway/gateway.go:242`). N2b nominates a file outside the granted route's allowlist.
- No configuration transition is needed between phases, so no Work history is lost to a Platform change. If execution shows a transition is unavoidable, Phase 6 parity for every earlier case is completed first (see section 5).

### G5 Opt-in runner

A script under `harness/integration/e16/` that requires `--context`, `--install-namespace`, `--output` and the model profile, refuses to run without them, passes `--context` on every `kubectl` call, checks the `kind load` cluster name against the context, and stops if ownership is unclear. It never deletes or recreates an existing cluster and never runs `platform apply` while a Work is active.

### G6 Installed MCP Portal spec

Add an MCP-specific installed Playwright spec, or parameterise `ui/installed/installed.spec.ts` without silently changing its existing assertions. It reuses `AGENOVA_CLI_PATH`, `AGENOVA_LIVE_REQUEST_REF` and `AGENOVA_LIVE_DENIED_REF`. Any new variables (truncation and tool-failure refs) are documented with the runner.

## 5. Execution phases

Each phase passes before the next starts. A failure is fixed and the phase rerun; nothing is bypassed by widening authority, relaxing parsers or hiding errors.

**Per-run parity rule.** Control-plane Work and evidence live in process memory ([runbook](../../docs/reference-cli-kind-ollama.md#seven-commands)). For every kind Work run (positive, admission Deny, N6, N7, N8), the CLI/API comparison and the rendered Portal check are done and archived immediately after that run, before any restart, fixture change or further configuration step. Phase 6 then reviews the archived parity results rather than re-querying old Work.

### Phase 0 Environment and ownership

Start Docker, confirm the kind cluster is live and Tom's, check for active Work or other installs in the target namespaces, select Go 1.22.12 and Node 24, and confirm Ollama is reachable from inside the cluster. If ownership cannot be confirmed, stop and record why.

Exit: environment table fully green, ownership recorded.

### Phase 1 Offline work (G1–G6)

Implement G1–G6 with focused deterministic tests, including any N6–N12 deterministic coverage not already in Slice 2. Validate all acceptance inputs offline with `platform validate`. Build the CLI on the host with Go 1.22.12. Build the control-plane and worker images from their Dockerfiles without the probe tag; the control-plane builder `golang:1.22-alpine` is a floating tag, so record its resolved digest and the Go version reported by `go version -m`. Build the fixture image separately and the probe image with the tag. Run the G2 separation checks.

Record for every image: source SHA, build command, build tags, builder image digest, toolchain, architecture and the image content digest (the local image ID, which is the config digest). Record a registry manifest digest only if one exists, and record the `docker save` archive SHA-256 separately as a transfer checksum, never as the image digest. Each image gets a unique source-derived tag; tags are mutable, so every later stage checks the content behind the tag against the recorded digest. `latest` and placeholders are never used.

Exit: focused tests green on Go 1.22.12, inputs validate, build identity recorded, tag separation proven.

### Phase 2 Deploy and capture

Load images into kind and apply the fixture manifests (one replica, ClusterIP, read-only ConfigMap, non-root, no service account token). Each runtime identity chain is completed when that component is actually running, mapping the recorded content digest to the node image and the Pod's `status.containerStatuses[].imageID`:

- Fixture: in this phase, once its Pod is Running.
- Control plane: in Phase 3, right after `platform apply` reports it installed.
- Worker: in Phase 3, while the positive Work is Running and before cleanup.

Local loading satisfies `spec.md` R5 when each chain maps to the recorded content digest. If a chain cannot be established, stop and fall back to a registry (for example a local registry attached to kind), which changes the reference environment and needs Tom's approval first.

Start log capture before any request: full MCP JSONL, Pod UID, container ID, restartCount, deployment events, control-plane logs and campaign start time. Health checks on `/healthz` bypass the receipt log, so they are not counted; `initialize` and `tools/call` receipts are counted separately.

Exit: one controlled read is captured end to end; readiness alone is not accepted as proof.

### Phase 3 Installed positive run

Follow the [seven-command runbook](../../docs/reference-cli-kind-ollama.md#seven-commands) with the E16 inputs: `adapters install/inspect/init` and lock check, `platform validate/plan/apply/status`, Policy and AgentTemplate registration, `run`, `work show`. Save every command and exit code, then an identical reapply to prove idempotence.

Task: investigate the payment retry incident using `logs/timeout.log` and `src/retry.txt`, explain why the 5s total deadline is exceeded and propose a fix. A checker reads the fixture files independently and asserts facts in the answer: each attempt starts a fresh deadline, the 4s first attempt plus 2s backoff exceeds the total budget, and retries need a stable idempotency key. Natural wording may vary. These facts are never placed in the model input.

Pass when all hold:

- Work goes Running to Succeeded with at least two real model turns and a real MCP file observation in between; no synthetic or mock path.
- MCP logs show `read_file` receipts matching the invocation IDs, with no replayed `tools/call`.
- Per invocation: ToolDecision Allow, then ProviderAttempt, then ProviderOutcome Succeeded, equal Targets, valid `resultRef` matching the real file.
- The answer satisfies the checker. `Allow`, HTTP 200 or `Succeeded` alone do not pass.
- CleanupSucceeded matches the runtime. The claim's resources and worker binding are released; a reference warm-pool worker retained by design is not a leak.
- Control-plane and worker image identity chains completed (Phase 2), the worker chain captured before cleanup.
- Per-run parity archived.

Run admission Deny next, with its parity archived.

### Phase 4 Failure cases on kind

Run N6, N7 and N8 as Works on `e16-faults`, archiving parity after each. The additional N10 unreachable case, if run, comes last in this phase, followed by fixture restoration and a positive control.

- N6: explicit timeout, the received request is kept, bounded cancellation, no replay, no fallback.
- N7: `tool-response-too-large`, no successful observation, judged on full wire bytes.
- N8: valid wire reply, observation capped at 4,096 bytes with intact UTF-8, untrusted and truncated markers reach the worker and, through G1, the CLI/API/Portal evidence; the answer admits incomplete data.
- N11 (server-backed): run through the probe Job as specified in G3. It makes one real call, so it belongs here rather than in the zero-call campaign.

### Phase 5 Zero-call campaign (N1–N5)

Start only after Phase 4 restoration and a successful positive control. Run each case serially in its own time window, with a positive control against the same server before and after. A window passes only if there is no `tools/call` receipt, no handler execution and no `initialize` session for that case, and the probe receipt shows the rejection happened at the intended point. A UI Deny, a zero client-side spy count or an empty log file do not pass.

| Case | Trigger | Intended rejection point | Required result |
| --- | --- | --- | --- |
| N1 | `repo.read` injected for a Running claim without the tool grant | Tool Gateway authority | Explicit Deny, no ProviderAttempt, zero calls |
| N2a | Claim granted `e16-fixture` nominates configured `e16-faults` | Tool Gateway authority | Deny, zero calls |
| N2b | Granted route, argument outside its allowlist | `toolParameter` allowlist | Rejection, zero calls |
| N3 | Handler of claim A, operation nominating Running claim B (section 4 G3) | Claim-ID check in the handler | Rejection, no fact attributed to B, zero calls |
| N4a/b/c | Captured handler with a live call context after Succeeded, Failed, Expired | `app.RequireRunningClaim` | Each rejected independently, zero calls after the boundary |
| N5a/b | ToolDecision append fails; ProviderAttempt append fails | Journal append before the provider | Explicit evidence failure, zero downstream calls |

N3 and the probe lifecycle prove only the service-side correlation guard. They do not claim authenticated worker isolation, which belongs to #197.

Any lost log, unknown request, Pod UID change, restartCount increase or collector exit invalidates the affected window. Keep the failed evidence, fix, and start a new campaign; never stitch partial runs into one pass.

### Phase 6 Parity review

Review the archived per-run parity for success, admission Deny, truncation and tool failure: `work show` (strict CLI parsing) against the API JSON for the same `requestRef`, and Portal assertions on claim and invocation IDs, `repo.read`, decision, ProviderOutcome, Target, `resultRef`, error reason, cleanup and the truncation marker. Inspect the saved screenshots. Every result must come from this campaign's installed revision and real Work; no mocked routes or old screenshots.

### Phase 7 Gates and evidence review

Run the focused Go suite from [task.md](task.md#quality-gates) on Go 1.22.12, the fixture module tests separately, `npm --prefix ui test`, then `pwsh -NoProfile -File ./scripts/check.ps1 -All` on Go 1.22.12 and Node 24. The known `console.spec.ts:99` keyboard failure is reported as-is, not skipped. Docs changes also pass `-Docs` and `git diff --check`. Review the full diff for scope, secrets and debug leftovers, then update the Slice 3 Todo and evidence summary in task.md.

## 6. Evidence bundle

Raw local output goes to `.tmp/e16-slice3/<campaign>/` (gitignored). Sanitised review evidence goes to `docs/evidence/178/slice3/<campaign>/` with a manifest and a SHA-256 file list:

- `run.json`: tested SHA, toolchains, build tags, context and namespaces, image identity chain, Platform revision, dataset hashes, start/end times, and pass/fail/not-executed per case with its tier and layer (production, probe composition, deterministic).
- Inputs, rendered manifests, Pod identity and restart state, build and deploy transcripts, the three runtime image identity chains and the G2 build-separation checks.
- CLI/API JSON, exit codes, Work/claim/invocation mapping, model turn evidence, cleanup checks and archived Portal parity for each kind Work run.
- `probes.jsonl`, complete MCP server logs, positive controls, per-case counts and a log-continuity report. Uncorrelated malformed or oversized receipts are kept, not filtered out.
- Focused, fixture, UI and full-gate output, and a final summary.

No token, credential or raw trusted metadata appears in any artifact.

## 7. Coordination with E14 (#197)

#197 adds a claim-bound worker JWT that Tool and Model Gateways verify before authority lookup and adapter invocation, on the same entry path as G2 and G3.

- The initial Slice 3 campaign runs on this reviewed baseline, without #197.
- After integration with #197, any rerun of N1–N5 must present a valid claim-bound identity and prove each probe still reached its intended rejection point. A missing-token rejection cannot stand in for N1–N5's distinct mechanisms.
- N3's correlation claim stays separate from authenticated cross-claim rejection, which #197 owns.
- A note on #197 about overlapping paths is useful but is a separate action that needs Tom's approval. It is not a prerequisite for implementation.

## 8. Order, commits and estimate

Commits on the same branch, in order: G1 truncation evidence; G2 append seam and tagged bridge; G3 probe composition with server-backed N11; G4 + G5 inputs and runner; G6 Portal spec; campaign evidence and packet update. Estimate: 4–6 working days including one fix-and-rerun cycle. This is an estimate, not a commitment. Model instability, environment issues and the N3 two-instance composition are the main risks. If time runs short before the 2026-11-05 final demo, protect the positive run and N1–N5 first, and record every other required item as a blocker rather than claiming Slice 3 passed.

## 9. Decisions

Resolved at review round 1:

1. Tiers: N10 unreachable on kind is additional. Deterministic connection failure and server-backed N11 are required; server-backed N11 runs on kind (round 2).
2. G1: a single optional `truncated` boolean on successful ProviderOutcome facts, legacy-compatible.
3. G2: a per-`Service` unexported append function covering all tool-path appends, replaced only through the `agenovaprobe`-tagged bridge.
4. G3: probes in `cmd/agenova-control-plane` under the tag; manifests and orchestration under `harness/integration/e16/`.
5. Publication: commit this English plan to the packet once round 2 has no P1/P2 findings. Machine-specific raw output stays local.

Resolved at review round 2:

6. Image pinning: local loading is acceptable when each runtime chain maps to a recorded content digest; a registry is the fallback and needs Tom's approval.
7. N3: two independent service instances are accepted for the narrow handler-correlation claim, with matched-claim controls and the exact rejection-point assertion.

No decisions are open.

## 10. Review record

Round 1 (2026-10-03) raised nine findings. Dispositions:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| 1 | P1 | One profile cannot serve the positive task, fault files and N2a | G4 rewritten: one revision, two scopes, per-Work grants |
| 2 | P1 | G2/G3 package boundary not implementable with `_test.go` alone | G2 tagged bridge in console; G3 tagged probes in the control-plane package; symbol check on the production binary (replaced in round 2, R2-1) |
| 3 | P2 | N3 must handle `runMu` as well; N4 could pass through context cancellation | G3 N3 two-instance composition with snapshots; N4 uses a live call context and asserts the `RequireRunningClaim` error; driver case IDs |
| 4 | P2 | Configuration transitions could erase evidence before Portal checks | Single revision; per-run parity rule archives CLI/API/Portal after each run |
| 5 | P2 | Phase 1 exit required Pod image identity before deployment; digest claim unconditional | Build identity in Phase 1, node/Pod chain in Phase 2; registry digest conditional; R5 question opened as decision 6 |
| 6 | P2 | N10 connection failure and server-backed N11 coverage unclear | Section 3 lists both as required |
| 7 | P2 | E14 integration could make probes reject at the wrong boundary | Section 7 rewritten; posting on #197 is a separate approved action |
| 8 | P3 | `/healthz` bypasses the receipt log | Health counting removed |
| 9 | P3 | Stale `service.go:288` references | spec.md R3, design.md and task.md criterion 5 now cite the handler symbol and current line |

Round 2 (2026-10-03) found no P1 and confirmed findings 1, 3, 4, 7, 8 and 9 resolved. Dispositions:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| R2-1 | P2 | `go tool nm` cannot prove build separation (stripped binary, dead-code elimination) | G2 now proves separation with `go list` file selection and `go version -m` build settings on the deployed binary; `nm` is supplementary; hooks installed before Work starts |
| R2-2 | P2 | Local N11 cannot reach a localhost fixture through the fixed endpoint; receipt count wording wrong | Server-backed N11 runs in the in-cluster probe Job; pass counts one `tools/call` receipt with a successful handler result |
| R2-3 | P2 | Runtime identities checked before control plane and worker run; archive hash conflated with digest; "immutable tag" | Per-component chains in Phases 2 and 3; content digest vs archive checksum separated; tags treated as mutable and checked; builder digest recorded |
| R2-4 | P3 | G4 title and N2a argument | "two routes"; N2a uses a valid `e16-faults` argument; fault cases assert the intended file |
| R2-5 | P3 | N3 scope of proof | Qualified to handler correlation only; matched-claim controls added |

Round 3 (2026-10-03) confirmed R2-1 to R2-5 resolved at plan level with no new P1/P2. Implementation and runtime evidence remain unverified.
