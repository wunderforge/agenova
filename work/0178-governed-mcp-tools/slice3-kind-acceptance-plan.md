# E16 Slice 3: installed kind acceptance plan

Status: revision 3, accepted for implementation after independent review rounds 1–3 (findings in [section 10](#10-review-record)). Phase 2 began on kind on 2026-10-04. Campaign c3 stopped at the fixture (L6). Campaign c4 passed Phase 2 and then stopped before `install` on a runner defect (L7). Campaign c5, a rehearsal on the L7 fix rather than evidence, reached every step through the probe and found L9–L14. L9–L11 are fixed in a new commit, so the next campaign (c6) starts again from preflight. Slice 4, E16's token-required path, is built offline (section 4, G7) and c6 proves it together with Slice 3. L12 set the rules for model answers on 2026-10-05 (decisions 14–19). Campaign c6, the formal campaign, passed every step on its first run at `fd08ac0` on 2026-10-05 (section 10; evidence in [docs/evidence/178/slice3/c6](../../docs/evidence/178/slice3/c6/summary.md)). On 2026-10-06 E14's split moved worker authentication from #197 to #205 (section 7). The plan does not change the Epic's completion state.

This plan refines the Slice 3 Todo in [task.md](task.md#execution-todo). The packet's [acceptance criteria](task.md#acceptance-criteria), [evidence requirements](task.md#evidence-required), [negative cases](spec.md#negative-cases) and [verification strategy](design.md#verification-strategy) remain authoritative. Where this plan and the packet disagree, the packet wins and this plan is corrected.

Goal: move Slice 2 from local interoperability to installed acceptance. The unmodified production control plane, a real Agent Sandbox worker, Ollama and the independent MCP fixture complete one verifiable task, and the five governance rejections are proven against complete server logs with zero tool calls.

## 1. Baseline (checked 2026-10-03)

- Branch `codex/e16-slice1`, HEAD `94a8481949be40ec0ec35cbb903414ee51c2c55c`, synced with origin. PR #192 is Draft with green CI. `main` is still `c56ba3a`.
- Slice 2 delivered the Streamable HTTP client, the D2 fixture (`harness/integration/mcpfixture/`), the per-Work catalog, `resultRef`, S2 attempt/outcome completion and S3 concurrency limits. Focused and local interop logs exist in `docs/evidence/178/`. None of that is installed evidence.
- Fixed endpoint: `http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp` (`fixtureMCPHost`, `internal/adapters/bundled/mcp.go:23`). Fixture namespace is `agenova-e16`. Slice 4 adds exactly one more cleartext URL on the same host and port, `.../mcp-token` (G7); nothing else changes in the exception.
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
| Ollama | `llama3.1:latest` present (the reference install's model). Since L12, E16 uses `qwen2.5:7b` (Tom approved pulling it on 2026-10-05; Q4_K_M, digest `845dbda0ea48`) | `preflight` requires the E16 model profile's model in the host Ollama and records its digest; check reachability from inside the cluster in Phase 0 |

The macOS external-linking workaround used for Go 1.22 focused tests on this host must not be carried into Linux container builds.

## 2. Scope and boundaries

In scope: the installed positive run, N1–N5 against the real server, the failure matrix, CLI/API/Portal parity for MCP invocations, and the code gaps in section 4.

Unchanged boundaries:

- Backend-neutral contracts and Gateway ownership of authority. The service still checks only the installed route and allowlisted argument before the Gateway.
- No production API, flag or route for fault injection or probing. The production binary contains one inactive append function value (G2) that always points at `journal.Append`; the API that replaces it compiles only under the `agenovaprobe` build tag.
- No root `go.mod` or CI toolchain change. No CI job for the fixture.
- Worker identity tokens and worker authentication (#205 since 2026-10-06, before that #197; source requirement #121), hostile-worker network bypass (E15), EKS (E13) and durable storage (E18) keep their current owners.
- PR #192 stays Draft. Slice 3 passing does not close #178; Slice 4, E16's own token-required MCP path, is still required, and c6 proves both (section 7).

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
| Token valid (`token-valid`): read through `/mcp-token`, every request `auth=ok` | kind | Required (criterion 8) |
| N13 token missing (`token-missing`): reference resolves to nothing, zero server entries | kind + deterministic | Required (criterion 8) |
| N14 token wrong (`token-wrong`): one `initialize` with `auth=invalid` answered 401, no retry | kind + deterministic | Required (criterion 8) |
| N14 header-less request on `/mcp-token` answered 401 with `auth=missing` (controlled read) | kind + deterministic | Required |
| Worker Pod env, manifest and mounts hold no token; no token in any artifact (scan) | kind | Required (criterion 8) |

### Deterministic coverage map

| Case | Deterministic test |
| --- | --- |
| N6 | `TestMCPClientFailuresAreClassifiedWithoutReplay/slow_server` |
| N7 | `TestMCPClientEnforcesResponseByteLimit`, `TestMCPClientResponseLimitCoversEventStreamsAndChunkedBodies` (event stream, large notification before the result, chunked body without Content-Length), `TestMCPClientRejectsOversizedEventStreamTailAfterResult` (the cap covers the stream after the result) |
| N8 | `TestSetBoundsUntrustedResultsWithoutChangingCallerParameters` (UTF-8 cut), `TestProviderBoundaryRejectsBeforeExternalCallAndRecordsBeforeAllow/allow` (markers and `truncated` evidence) |
| N9 | `TestToolConfigAndRoutesFailClosed`, `TestInstalledToolBuilderFailsClosedAndNeverSubstitutesMock`, `TestInstalledToolBuilderUsesResolvedRoutesAndProviderFactory` (`tool_unsupported`, and `tool_route_unavailable` for a grant whose scopes have no route), `TestToolTokenReferenceNeverTargetsTheCredentialFreeRoute` |
| N10 | `TestMCPClientUnreachableServerIsUnavailable`, `TestMCPClientFailuresAreClassifiedWithoutReplay`, `TestMCPClientHandshakeFailuresStopBeforeToolCall` |
| N11 | `TestProviderBoundaryRejectsBeforeExternalCallAndRecordsBeforeAllow/outcome-record` (returns the tool evidence error), `TestProbeAppendWrapperFailsBeforeProviderCall` (Work outcome `tool-evidence-failed`); server-backed in the probe Job, which asserts the refused outcome was `Succeeded` |
| N12 | `TestInjectedToolTextCannotWidenAuthority` |
| N13 | `TestMCPClientUnresolvableTokenSendsNothing`, `TestKubectlSecretReaderFailsWithOneConstantError`, `TestInstalledToolBuilderResolvesTokenReferencesPerCall`, `TestConfiguredServiceRecordsCredentialFailures` |
| N14 | `TestMCPClientCredentialRejectionIsExplicitAndNotRetried`, fixture `TestTokenPathRejectsWithoutTheTokenBeforeAnySession`, `TestConfiguredServiceRecordsCredentialFailures` |

Recording failures are explicit. A ToolDecision, ProviderAttempt or ProviderOutcome that cannot be recorded returns `tool evidence recording failed` to the worker and ends the Work with reason `tool-evidence-failed`, never an ordinary tool failure. The Tool Gateway now wraps its decision-observer error with `%w` (`internal/toolgateway/gateway.go`), the only change in that package. E18's plan on [#180](https://github.com/wunderforge/agenova/issues/180#issuecomment-5862744273) names that file, and E14's #205 will add worker-credential checks on the Gateway entry path.

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

### G4 One Platform revision with five routes

`spec.md` R1 allows one profile per (operation, resource scope) pair and forbids duplicates, and `toolParameter` rejects any file outside the profile's allowlist before MCP. One profile therefore cannot serve both the positive task and the fault files. Use one backend and two `repo.read` profiles in a single Platform revision for the whole campaign:

| Profile | Operation | Resource scope | Allowed values |
| --- | --- | --- | --- |
| `e16-fixture` | `repo.read` | `repo:agenova/e16-fixture` | `README.md`, `logs/timeout.log`, `src/retry.txt` |
| `e16-faults` | `repo.read` | `repo:agenova/e16-faults` | `logs/slow.log`, `logs/full-trace.log`, `notes/incident-timeline.md` |

Backend config: `timeout` 5s, `max-response-bytes` 65536, `max-observation-bytes` 4096, `max-concurrent-calls` recorded as applied.

- The AgentTemplate (new ID/version) has a ceiling of `repo.read` on both scopes. The Policy (new ID/version) admits both.
- The positive Work requests only `repo:agenova/e16-fixture`, so its catalog never shows a fault file. Fault Works (N6, N7, N8) each request only `repo:agenova/e16-faults`. Each Work has a unique `requestRef`.
- Fault Works advertise all three fault files. Each fault case asserts the intended file in its tool call and the expected failure or truncation result; a run where the model reads a different file fails its check and is rerun as a new recorded attempt (section 5). L12 kept this layout rather than giving N8 its own route (decision 15).
- N2a uses a claim granted only `e16-fixture` and nominates the configured `e16-faults` route with an argument valid for that route (`notes/incident-timeline.md`), so `toolParameter` passes and the rejection comes from the Gateway scope check (`internal/toolgateway/gateway.go:242`). N2b nominates a file outside the granted route's allowlist.
- Slice 4 adds three backends on `/mcp-token`, each with one profile and its own scope allowing only `README.md` (G7). `e16-mcp` and its two profiles are unchanged, so the Slice 3 Works keep their catalogs. The template ceiling gains the three token scopes.
- No configuration transition is needed between phases, so no Work history is lost to a Platform change. If execution shows a transition is unavoidable, Phase 6 parity for every earlier case is completed first (see section 5).

### G5 Opt-in runner

A script under `harness/integration/e16/` that requires `--context`, `--install-namespace` (default proposal `agenova-e16-system`, kept separate from any existing `agenova-system` install), a dedicated CLI state directory, `--output` and the model profile. The E16 Platform sets the control-plane and runtime namespaces to the same value, which the installer enforces (`validateReferenceRuntime`, `kubernetes.go:76`). Because the control-plane and worker image tags are shared on the node (Phase 1), the runner protects any existing install before loading images: it records the node image IDs currently behind those two tags, exports them from the node's containerd store to an archive under the local output directory, and after the campaign re-imports that archive and verifies the restored image IDs match the record. `kind load image-archive` imports with `--all-platforms`, so the export includes every platform and the attestation manifests, and protect, load and restore refuse any archive that lacks a blob its `index.json` reaches or whose structure the walk cannot follow by role. Marking the old install out of scope is not enough, because a restart would otherwise pick up the E16 images. The runner also refuses a probe Job whose `go test` output shows the probe test skipped, or whose `E16_PROBE` receipts are missing, duplicated or failing for any expected case and step, even when the Job exits 0. The runner refuses to run without its required arguments, passes `--context` on every `kubectl` call, checks the `kind load` cluster name against the context, and stops if ownership is unclear. It never deletes or recreates an existing cluster and never runs `platform apply` while a Work is active.

G5 also enforces, rather than only records, the campaign's integrity:

- Every Kubernetes inventory query fails closed: a failed or unreachable query never reads as "nothing installed" or "nothing running", and fixed-tag Pods without a known controller block `protect`.
- Image identity at every stage: the node tag must still be the recorded config, and each running Pod's `imageID` must resolve to it. kind-loaded images appear in Pod status as `docker.io/library/import-<date>@sha256:…`, which only the node image list (`crictl images`) maps back to the image ID. Each check resolves against one saved node image list and keeps it, with every line's resolution, next to the identity file (`<file>.node-images.json`, `<file>.mapping`), so the fixture, control-plane and worker chains can be checked again from the archive. The claimed worker named in the Work evidence must have been captured on the recorded worker image. A worker capture taken before its container started (no container ID and no image ID) says nothing about the image and is skipped; the claimed worker still needs at least one started capture, and every started capture must resolve to the recorded config.
- `install` keeps each attempt in `install/<UTC time>/`: every command's combined output in `<step>.txt`, and its command line and exit status in `commands.txt`. It stops at the first failing command, and a rerun never overwrites an earlier attempt.
- `parity` refuses to start while anything listens on 8088 (`api connect`) or 5177 (the UI dev server), counts only the tunnel its own `api connect` reported, and on every exit (success, failure, Ctrl-C or SIGTERM) stops both servers and the test run with all their descendants, including the `kubectl port-forward` below `api connect`. It fails if either port still has a listener afterwards, so a later run can never reach an earlier run's tunnel.
- Log continuity: fixture and control-plane collectors must start cleanly and stay alive; the collector output is frozen (complete records only) before each complete snapshot and every frozen line must appear in it (missing or unreadable files fail); the fixture Pod identity and restart count must match its start record.
- The control-plane Pod, its image and restart count are rechecked against the install record before and after every production Work.
- `work <name> [--attempt N]` runs one attempt of a Work (section 5, recorded attempts), checks it with `evidence work` (below) and archives its parity immediately, also when a check fails. Each attempt and each probe run is recorded once per campaign directory.
- `--model-profile` is required and must exist in both `platform.yaml` and the template ceiling. `preflight` also requires the profile's model in the host Ollama that kind reaches through `host.docker.internal`, and records its name and digest, so a missing model stops the campaign before any change rather than failing a Work (L12).
- `controlled-read` (the Phase 2 exit) runs once per campaign and only while no E16 control plane exists. A failed query refuses. It also refuses if the fixture log already holds its correlation.

The evidence checker (`harness/integration/e16/evidence`) decides calls from the server log alone:

- Probe mode fixes the expected `tools/call` count per case and step; the receipt's own count is ignored. Invocation IDs must be unique across steps, step windows must not overlap, every correlated server entry must fall inside its own step's window, and any server entry inside the probe campaign that belongs to no step fails the run, including malformed or unparseable requests.
- Work mode (`evidence work -case <name> -ref <request name>`) checks one attempt of a production Work. Its evidence must be that attempt's own (`requestRef` equal to `-ref`). Each attempted invocation made exactly one `tools/call` for an allowed file, denied invocations never reached the server, nothing unattributed happened during the Work, N6/N7 failed with exactly `tool-timeout` on `logs/slow.log` or `tool-response-too-large` on `logs/full-trace.log`, N8 has a truncated successful read of `notes/incident-timeline.md` and an answer admitting incomplete data, and the answer facts match. Pattern matching on prose was tried and could not prove meaning (negations, unrelated mentions, reversed claims all passed), so the positive and N8 objectives ask the agent to end its answer with a fixed facts block: the answer's final lines, each exactly `key: value` in plain text, with nothing after them. The positive block states the diagnosis (fresh per-attempt deadline, first attempt 4s, backoff 2s, deadline 5s, budget exceeded, stable idempotency key needed) and the requested fix in structured form (share one deadline across attempts, reuse one idempotency key across retries); N8 states `timeline_complete`. The checker derives every expected value from the fixture dataset (the retry summary must use one of two supported statements of the deadline behaviour, otherwise the dataset is rejected; N8 compares the timeline with the 4,096-byte observation cap) and requires each key exactly once in the final block and nowhere earlier in the answer, in any case or quoting. A fenced block, a block inside an unclosed fence, or text after it is not a facts block. Prose before the block is not judged. An agent that ignores the format fails; the attempt is kept and inspected, and a rerun is a new recorded attempt (section 5), never a pass by loosening the check. Request entries (POST `initialize`, notifications, `tools/call`) must fall inside their invocation's attempt-to-outcome window; completion entries (responses, handler records, the session DELETE) may follow within 2s, or within a minute for a timed-out call. Denied invocations must cause no server entry at all, and a successful read needs one session, one `tools/call` and one successful handler entry for the same file, plus a `resultRef` equal to the Work's resource scope, `/` and the file its server `tools/call` named. In the positive case `logs/timeout.log` and `src/retry.txt` each need such a successful correlated read, so correct answers alone never pass. The invocations of every earlier attempt, failed attempts included, are passed with `-prior` as `<id> <end> <state>` records (end is the latest fact by instant; state is `denied`, `no-outcome` or the outcome reason). Entries up to an invocation's end are its history, already judged with its own Work. Afterwards a denied invocation may log nothing; any other may log each completion kind at most once: its session close (DELETE receipt or response) within 2s, and, for a timed-out call only, its handler, `tools/call` response and session close within a minute while the slow handler finishes. Anything else fails. After a Work with a timed-out call, the runner waits until the server logs that handler finishing. The probe checker applies the same 2s completion allowance and `-prior` rule.

### G6 Installed MCP Portal spec

Add `ui/installed/mcp.spec.ts`, run on its own with a file filter, so the reference `installed.spec.ts` keeps its assertions unchanged. It checks one Work attempt per run: `AGENOVA_E16_CASE` selects the case and `AGENOVA_E16_REF` the attempt's request name, and the runner passes `--grep '@setup$|@<case>$'` and requires the JSON report to show exactly two passed tests with nothing skipped, failed or flaky. It also reads `AGENOVA_CLI_PATH` and `AGENOVA_CLI_STATE_DIR`. The positive test requires a successful outcome for both `logs/timeout.log` and `src/retry.txt`, each `resultRef` exactly the scope plus a route file, and checks both records in the Portal with their exact `resultRef`; joining each result to the server's file is the evidence checker's job. N6 and N7 each have their own test with an exact reason code. The Portal also labelled every `tool.invoke` attempt and outcome as "Mock tool call", including configured provider calls; only facts with a `mock-*` reason code (the synthetic adapter) keep that label, and the spec asserts a real MCP record never shows it.

### G7 Token-required path (Slice 4)

Design: [design.md Slice 4](design.md#slice-4-token-required-mcp-path). Product side: the `provisional-token-secret` reference, the per-call Secret read through a name-scoped Role rule, `Authorization: Bearer` on every request of the session, `tool-credential-unavailable` and `tool-credential-rejected`. Built offline on this branch:

| Backend | Reference | Scope | Work | Pass on kind |
| --- | --- | --- | --- | --- |
| `e16-mcp-token` | `e16-mcp-token/token` | `repo:agenova/e16-token` | `token-valid` | Succeeded; one session on `/mcp-token`, every receipt `auth=ok`, one ok handler, `resultRef` `repo:agenova/e16-token/README.md` |
| `e16-mcp-token-missing` | `e16-mcp-token-absent/token` | `repo:agenova/e16-token-missing` | `token-missing` | Failed, `tool-credential-unavailable`; no server entry carries the invocation |
| `e16-mcp-token-wrong` | `e16-mcp-token-wrong/token` | `repo:agenova/e16-token-wrong` | `token-wrong` | Failed, `tool-credential-rejected`; exactly one `initialize` receipt with `auth=invalid` and its 401 response, nothing else |

- **Fixture.** `/mcp-token` has its own SDK handler and requires the token from `FIXTURE_TOKEN_FILE` (Secret `e16-mcp-token` in `agenova-e16`). Every receipt on either path carries `path` and an `auth` class (`ok`, `missing`, `malformed`, `invalid`); a non-`ok` request on `/mcp-token` is answered 401 before the SDK. The log never holds the header, the token or any hash of it.
- **Secrets.** Tokens are 64 lowercase hex characters from `openssl rand -hex 32`, validated in the pipeline and created with `kubectl create secret generic --from-file=token=/dev/stdin`: never argv, never `apply`, never a file, never regenerated. `fixture` creates the namespace and the fixture Secret before applying and then restarts the fixture onto it; `install` copies the value into the install namespace (or compares an existing copy), creates `e16-mcp-token-wrong` and asserts `e16-mcp-token-absent` is absent. `secrets.txt` keeps each Secret's uid and resourceVersion, nothing derived from the value. A traced run (`-x` or `SHELLOPTS=xtrace`) is refused, because tracing would print the values.
- **Who can read it.** After install the runner archives the control-plane Pod and Role JSON and three access reviews: the control-plane service account may `get secret/e16-mcp-token`, may not get an unlisted name, and the namespace `default` service account, which the worker runs as, may not get it.
- **Worker.** While each Work runs, the watcher keeps the worker Pod list and, once per started Pod UID, the Pod JSON, `/proc/1/environ` and `/proc/1/mountinfo` read inside the agent container. The claimed worker needs all three, from the same UID as its running identity line, and must show automount off, no volumes, no volume mounts, no env `valueFrom` or `envFrom` in any container kind, and no mount under `/var/run/secrets` or `/run/secrets`.
- **Controlled read.** One extra header-less `initialize` to `/mcp-token` (correlation `inv-m2`) must be answered 401 and logged as one receipt with `auth=missing` and one 401 response. The `inv-42` calls must all be on `/mcp` with `auth=missing`.
- **Checker.** Slice 3 Works and the probe must show `path=/mcp` and `auth=missing` on every entry, so a token sent to the credential-free backend fails. After a token-failed invocation nothing may follow except, for a rejected token, its one 401 response within 2s (the fixture logs it after answering).
- **Scan.** `scan [extra-root...]` is the last step before restore. It checks the three Secrets are the recorded objects, takes final complete control-plane and fixture logs, then pipes the three values (`valid-fixture`, `valid`, `wrong`) to `evidence scan`, which requires valid to equal valid-fixture and searches `$OUTPUT`, `$STATE_DIR` and each extra root by real path for the raw value, each half and the base64 core at all three alignments, inside zip entries too, and prints only file names and counts. A run stopped before the search can be repeated; a search cannot. The sanitised `docs/evidence` export is an extra root, scanned while the Secrets still exist. Images and binaries are built before any Secret exists; PNG screenshots cannot be searched, so the Portal spec saves each token page as HTML beside its screenshot.
- **Portal.** `@token-valid`, `@token-missing` and `@token-wrong` assert the exact facts and reason codes, an exact "Tool call finished" heading and no "Mock tool call". The Work page now labels a call "(mock)" only for `mock-` reason codes, and the token tests assert it shows no "(mock)".

## 5. Execution phases

Each phase passes before the next starts. A failure is fixed and the phase rerun; nothing is bypassed by widening authority, relaxing parsers or hiding errors.

**Per-run parity rule.** Control-plane Work and evidence live in process memory ([runbook](../../docs/reference-cli-kind-ollama.md#seven-commands)). For every kind Work attempt (positive, admission Deny, N6, N7, N8) whose `work show` succeeded, passing or failing, the CLI/API comparison and the rendered Portal check are done and archived immediately after that attempt, before any restart, fixture change or further configuration step. Phase 6 then reviews the archived parity results rather than re-querying old Work.

**Recorded attempts.** The control plane refuses a reused request name, so a Work is never resubmitted; a rerun is a new attempt. `work <case> --attempt N` (default 1) submits the case's request as `e16-<case>-a<N>` and keeps everything in `work/<case>/attempt-<N>/`, which nothing overwrites. `work show`, the checker (`-ref`), parity and the Portal spec use that attempt's request name; the checker case stays the case name. The Work that `work show` returns must be the one this invocation submitted: `run --json` prints the evidence of its own submission (nothing when the submission is refused, as it is for a reused name), and those facts must be the first facts of the queried Work, with the same request, decision and outcome. Otherwise the attempt stops before any check, record or parity. Once `work show` has succeeded and matched the submission, the attempt's invocations go to the prior records and its parity is archived even when a check fails, and the step then fails with the attempt's first failure. Every attempt is kept and reported. Since L12 (decision 18), a case gets at most five attempts and its first passing attempt counts: `work` refuses an attempt number above 5, and refuses any attempt of a case whose earlier attempt passed, so a pass is never chosen from several. A case with no passing attempt in five fails the campaign. Answers from a local model vary between runs, so the rule applies to every case alike; for the cases whose result does not depend on the answer, a second attempt only repeats a deterministic failure.

### Phase 0 Environment and ownership

Start Docker, confirm the kind cluster is live and Tom's, check for active Work or other installs in the target namespaces, select Go 1.22.12 and Node 24, and confirm Ollama is reachable from inside the cluster. If ownership cannot be confirmed, stop and record why. Since Slice 4, `preflight` refuses unless both the install namespace and `agenova-e16` are absent: the registration store refuses a changed template body under an existing name, and the token Secrets must be generated for this campaign.

Exit: environment table fully green, ownership recorded.

### Phase 1 Offline work (G1–G6)

Implement G1–G6 with focused deterministic tests, including any N6–N12 deterministic coverage not already in Slice 2. Validate all acceptance inputs offline with `platform validate`. Build the CLI on the host with Go 1.22.12. Build the control-plane and worker images from their Dockerfiles without the probe tag; the control-plane builder `golang:1.22-alpine` is a floating tag, so record its resolved digest and the Go version reported by `go version -m`. Build the fixture image separately and the probe image with the tag. Run the G2 separation checks.

Record for every image: source SHA, build command, build tags, the base image digests BuildKit resolved, the Go toolchain of the binary inside, platform, and the config digest read from the saved archive, which is what containerd on the node reports as the image ID. With the containerd image store the local image ID is an index digest, so it is recorded separately and never compared with the node. Record a registry manifest digest only if one exists, and record the `docker save` archive SHA-256 separately as a transfer checksum, never as the image digest. The fixture and probe images get unique source-derived tags. The control-plane and worker tags are fixed by the reference installer (`controlPlaneImage` in `internal/adapters/bundled/kubernetes.go:35`, `referenceControlledWorkerImage` in `registry.go:24`), so loading the E16 builds replaces those tags in the shared kind node cache for every install on the node. Tags are mutable either way, so every later stage checks the content behind the tag against the recorded digest. `latest` and placeholders are never used.

Exit: focused tests green on Go 1.22.12, inputs validate, build identity recorded, tag separation proven.

### Phase 2 Deploy and capture

Load images into kind and apply the fixture manifests (one replica, ClusterIP, read-only ConfigMap, non-root, no service account token). Each runtime identity chain is completed when that component is actually running, mapping the recorded content digest to the node image and the Pod's `status.containerStatuses[].imageID`:

- Fixture: in this phase, once its Pod is Running.
- Control plane: in Phase 3, right after `platform apply` reports it installed.
- Worker: in Phase 3, while the positive Work is Running and before cleanup.

Local loading satisfies `spec.md` R5 when each chain maps to the recorded content digest. If a chain cannot be established, stop and fall back to a registry (for example a local registry attached to kind), which changes the reference environment and needs Tom's approval first.

Start log capture before any request: full MCP JSONL, Pod UID, container ID, restartCount, deployment events, control-plane logs and campaign start time. Health checks on `/healthz` bypass the receipt log, so they are not counted; `initialize` and `tools/call` receipts are counted separately.

Exit: one controlled read is captured end to end; readiness alone is not accepted as proof. Since Slice 4 the `fixture` step first creates the fixture namespace and token Secret, and the controlled read includes the header-less `/mcp-token` request (G7).

Controlled read (decided 2026-10-03): after `fixture` and before `install`, the runner's `controlled-read` port-forwards to `service/e16-mcp` and runs the Slice 2 interop test (`TestMCPClientInteroperatesWithFixtureServer`) through the real Agenova MCP client. It makes one successful `README.md` read plus the oversized and missing-file rejections, all with correlation `inv-42`. It passes only when the test passes and the port-forward stayed alive. The fixture log collector must also have captured the three calls in order, each in its own distinct session. Each call runs `initialize`, then the initialized notification, `tools/call` for its file, the handler and the closing DELETE in that session. The handler outcomes must be exactly `ok`, `ok` and `error` (`not-found`), with no other request or handler entries. Every request must have exactly one 2xx response. A response answers the earliest open request logged before it with the same HTTP method, RPC method and RPC id, so a missing, duplicated or mismatched response fails. The log snapshot must also pass the continuity and Pod identity checks. The port-forward bypasses cluster DNS; the in-cluster path is proven by the Phase 3 positive Work. The read's entries come before every Work and probe window and never match a real invocation ID, so later checks do not count them.

### Phase 3 Installed positive run

Follow the [seven-command runbook](../../docs/reference-cli-kind-ollama.md#seven-commands) with the E16 inputs: `adapters install/inspect/init` and lock check, `platform validate/plan/apply/status`, Policy and AgentTemplate registration, `run`, `work show`. Save every command and exit code, then an identical reapply to prove idempotence.

Task: investigate the payment retry incident using `logs/timeout.log` and `src/retry.txt`, explain why the 5s total deadline is exceeded and propose a fix, ending with the facts block (G5). The checker derives the expected values from the fixture files: each attempt starts a fresh deadline, the 4s first attempt plus 2s backoff exceeds the 5s budget, and retries need a stable idempotency key. The objective names the keys only, never their values.

Pass when all hold:

- Work goes Running to Succeeded with at least two real model turns and a real MCP file observation in between; no synthetic or mock path.
- MCP logs show `read_file` receipts matching the invocation IDs, with no replayed `tools/call`.
- Per invocation: ToolDecision Allow, then ProviderAttempt, then ProviderOutcome Succeeded, equal Targets, valid `resultRef` matching the real file.
- The answer satisfies the checker. `Allow`, HTTP 200 or `Succeeded` alone do not pass.
- CleanupSucceeded matches the runtime. The claim's resources and worker binding are released; a reference warm-pool worker retained by design is not a leak.
- Control-plane and worker image identity chains completed (Phase 2), the worker chain captured before cleanup.
- Per-run parity archived.

Run admission Deny next, with its parity archived. `install` also copies the token into the install namespace, creates the wrong token and archives the access reviews (G7).

### Phase 4 Failure cases on kind

Run N6, N7 and N8 as Works on `e16-faults`, archiving parity after each. The additional N10 unreachable case, if run, comes last in this phase, followed by fixture restoration and a positive control.

- N6: explicit timeout, the received request is kept, bounded cancellation, no replay, no fallback.
- N7: `tool-response-too-large`, no successful observation, judged on full wire bytes.
- N8: valid wire reply, observation capped at 4,096 bytes with intact UTF-8, untrusted and truncated markers reach the worker and, through G1, the CLI/API/Portal evidence; the answer admits incomplete data.
- N11 (server-backed): run through the probe Job as specified in G3. It makes one real call, so it belongs here rather than in the zero-call campaign.
- Token Works (G7), after N8 and before the zero-call campaign: `token-valid`, `token-missing` (its absent Secret re-asserted just before submission) and `token-wrong`, each with its worker captures checked and parity archived.

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

N3 and the probe lifecycle prove only the service-side correlation guard. They do not claim authenticated worker isolation, which belongs to #205.

Any lost log, unknown request, Pod UID change, restartCount increase or collector exit invalidates the affected window. Keep the failed evidence, fix, and start a new campaign; never stitch partial runs into one pass.

### Phase 6 Parity review

Review the archived per-run parity for success, admission Deny, truncation and tool failure: `work show` (strict CLI parsing) against the API JSON for the same `requestRef`, and Portal assertions on claim and invocation IDs, `repo.read`, decision, ProviderOutcome, Target, `resultRef`, error reason, cleanup and the truncation marker. Inspect the saved screenshots. Every result must come from this campaign's installed revision and real Work; no mocked routes or old screenshots.

### Phase 7 Gates and evidence review

Before `restore`, run `scan` (G7) over the output, the CLI state and the sanitised export. Run the focused Go suite from [task.md](task.md#quality-gates) on Go 1.22.12, the fixture module tests separately, `npm --prefix ui test`, then `pwsh -NoProfile -File ./scripts/check.ps1 -All` on Go 1.22.12 and Node 24. The known `console.spec.ts:99` keyboard failure is reported as-is, not skipped. Docs changes also pass `-Docs` and `git diff --check`. Review the full diff for scope, secrets and debug leftovers, then update the Slice 3 Todo and evidence summary in task.md.

## 6. Evidence bundle

Raw local output goes to `.tmp/e16-slice3/<campaign>/` (gitignored). Sanitised review evidence goes to `docs/evidence/178/slice3/<campaign>/` with a manifest and a SHA-256 file list:

- `run.json`: tested SHA, toolchains, build tags, context and namespaces, image identity chain, Platform revision, dataset hashes, start/end times, and pass/fail/not-executed per case with its tier and layer (production, probe composition, deterministic).
- Inputs, rendered manifests, Pod identity and restart state, build and deploy transcripts, the three runtime image identity chains and the G2 build-separation checks.
- CLI/API JSON, exit codes, Work/claim/invocation mapping, model turn evidence, cleanup checks and archived Portal parity for each kind Work run.
- `probes.jsonl`, complete MCP server logs, positive controls, per-case counts and a log-continuity report. Uncorrelated malformed or oversized receipts are kept, not filtered out.
- Focused, fixture, UI and full-gate output, and a final summary.

No token, credential or raw trusted metadata appears in any artifact. Since Slice 4 this is enforced: `scan` searches every artifact, the CLI state and the export for the campaign's token values before restore (G7), and the export is committed only after that scan passes.

## 7. Coordination with E14 (#197)

Until 2026-10-06 the worker JWT belonged to #197. E14 then split into sequential milestones ([task.md](task.md#decisions-and-blockers), E14 split): #197 is now M1, the Policy kernel, and the worker JWT moved to [#205](https://github.com/wunderforge/agenova/issues/205) (M4). This section names the current owner.

#205 adds a claim-bound worker JWT that Tool and Model Gateways verify before claim and authority lookup, on the same entry path as G2 and G3.

Slice 4 no longer depends on E14 (Tom, 2026-10-04, recorded on [Epic #178](https://github.com/wunderforge/agenova/issues/178#issuecomment-5977720567)): E16 delivers its own token-required path for the `mcp-http` backend ([task.md](task.md#execution-todo) Slice 4), and the formal campaign c6 proves it on kind together with Slice 3. What remains with E14 is merge order.

- c6 ran on this branch, without any worker JWT.
- After #205 merges, E16 reruns N1–N5 under the worker JWT, with nothing needed from E14. Each probe must present a valid claim-bound identity and prove it still reached its intended rejection point. A missing-token rejection cannot stand in for N1–N5's distinct mechanisms.
- N3's correlation claim stays separate from authenticated cross-claim rejection, which #205 owns.
- A note on #205 about overlapping paths is useful but is a separate action that needs Tom's approval. It is not a prerequisite for implementation.

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

Resolved after c4 (2026-10-04):

8. Scope of the post-c4 fix: L7, L8 and Codex findings C1–C4 (section 10), each with regression tests, in one commit. No restructuring yet.

Resolved after c5 (2026-10-04):

9. Scope of the post-c5 fix: L11, L9 and L10 (section 10), each with regression tests, in one commit. L12 is measured off-cluster after the L11 fix before c6's rules for model answers (objective wording, attempts per case, model profile) are set. L13, L14 and the five items carried over from the post-c4 fix are not in this round.

Resolved for Slice 4 (Tom, 2026-10-04):

10. The fixture serves a second path, `/mcp-token`, rather than requiring the token everywhere, so Slice 3's inputs, controlled read and probe stay unchanged; the cleartext exception becomes two exact URLs.
11. The control plane reads the Secret through the Kubernetes API on every call, with a Role rule restricted by `resourceNames`; no Secret volume or env.
12. On kind, "missing" is a reference that resolves to nothing (a Work), and the server's refusal of a header-less request is shown once in the controlled read.
13. The Portal Work page's "Tool call (mock)" label is fixed in Slice 4, so token-failure screenshots do not contradict "no mock fallback".

Resolved for L12 (2026-10-05). Tom delegated the choice to the measurement's recommendation. The measurement ran the real worker binary and model adapter against the host Ollama and judged every run with the checker's own code. It is host only, not kind evidence; numbers are checker passes with their 95% interval, and the cell-by-cell record is in section 10.

14. E16's `coding-standard` model profile uses `qwen2.5:7b` (Q4_K_M, the same size class) instead of `llama3.1:latest`. With llama3.1 no positive answer passed in 120 runs: the original objective (60), a rewritten objective, a worker line that restates the objective on the answer turn, both together, and the rewrite with key definitions (15 each). The reference install keeps its own Platform and `llama3.1:latest`. `ui/installed/mcp.spec.ts` asserts the configured model, and `TestModelProfileMatchesThePortalSpec` keeps the two in step.
15. N8 keeps the three-file `e16-faults` catalog; G4 is unchanged. llama3.1 read a second fault file in 30 of 30 runs, and still did in 30 of 30 when the objective said to read no other file, so only an N8-only route would have hidden it. qwen2.5:7b read no second fault file in 135 runs on the three-file catalog, so no route change is needed.
16. The demo worker says a truncated observation in words. After a successful read whose host reply is marked truncated, the transcript gains one worker line: the text was cut off at its size limit, only its beginning was shown, rereading cannot show more, and the answer should say so. It follows the host's `Truncated` flag, never tool text (`TestReActNotesATruncatedObservationFromTheHostFlag`, including host-shaped text without the flag). Without it qwen2.5:7b called the cut-off timeline complete in most runs. It changes the worker image only; the prompt prefix the control plane checks is unchanged. It applies to every configured-tool read the host truncates; today only N8's timeline exceeds the 4,096-byte cap, and the reference demo's mock replies are never truncated.
17. Objective wording. Positive: two to four sentences first, then the eight lines separated by newlines; the first six record the files and the last two what the recommended fix would do (the old "each value taken from the files" contradicted the fix lines); `deadline_resets_each_attempt` and `first_attempt_seconds` say in their placeholders what they mean, never a value. N8: read no other file, one sentence per line, then the fact line on its own line. `TestObjectivesNameFactKeysNotValues` keeps every fact line `key: <placeholder>` with no digit, every yes/no placeholder naming both answers, and no key or digit earlier in the objective except the positive premise "exceed the 5 second total deadline", which predates the facts block; dropping it measured no better (6 of 15 against 9 of 15). `TestObjectivesKeepTheMeasuredWording` pins the measured phrases.
18. Attempts: at most five recorded attempts per case, the first passing attempt counts, and `work` refuses any attempt of a case that has passed (section 5). At the confirmation rates, five attempts pass positive with probability 0.87 (0.66 at the interval's lower bound) and N8 with 0.998; three would give positive only 0.70. An attempt costs about a minute on kind plus its parity, and every attempt stays on record.
19. Unchanged: the evidence checker and its facts-block rules, and the worker's tool-turn rule "reading every input is not required". That rule conflicts with the positive objective's "read both files", but qwen2.5:7b read both required files in 127 of 135 runs, changing it did not help in a diagnostic (section 10), and it would alter every Work's tool turns, the reference demo's included. `preflight` now requires the profile's model in the host Ollama (G5).

c6 order: preflight, build, protect, load, fixture, controlled-read, install, Works (positive, admission-deny, n6-timeout, n7-oversize, n8-truncation, token-valid, token-missing, token-wrong), probe, scan, restore. The three token Works add roughly 2–3 minutes to c5's 15, with parity per run unmeasured; every image is rebuilt uncached. Model answers follow decisions 14–19: each case may take up to five attempts and its first pass counts.

Open items, recommended by Codex after c4 and not yet decided:

- An operator-provisioned, dedicated acceptance cluster instead of sharing the fixed image tags with `agenova-system`. Separate namespaces do not isolate shared tags; a dedicated cluster would retire most of protect, export and restore.
- Backend acceptance checks consolidated in the Go checker, with Playwright kept for parity and rendering.
- A campaign record that freezes the source SHA and inputs and allows separately recorded attempts, instead of depending on the live HEAD and one uninterrupted Mac process.

Codex advised against a general orchestration framework, a semantic prose grader, or turning every deterministic negative case into another kind experiment.

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

Phase 1 quality review (2026-10-03) found the implementation incomplete. Dispositions:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| Q1 | P1 | Inventory and Pod queries fail open | Queries return failure; every caller refuses; unowned fixed-tag Pods block protect |
| Q2 | P1 | Checker accepts unknown traffic and malformed requests | Every entry inside the campaign must belong to one step's session and window |
| Q3 | P1 | Receipts set their own expectations; sessions reusable | Fixed case/step expectations, unique invocation IDs, window-bound sessions, no overlapping windows |
| Q4 | P1 | N11 passes after a provider failure; recording failures look generic | Probe records the refused fact and requires `ProviderOutcome::Succeeded`; explicit `tool evidence recording failed` and `tool-evidence-failed` |
| Q5 | P1 | Oversized SSE tail after the result passes | The stream is read to its end within the cap; new N7 test; interop with the real fixture rechecked |
| Q6 | P1 | Parity cannot run per Work; N7 absent; N6 reason loose | Per-case spec selection, per-Work archives, exact reasons, N7 test |
| Q7 | P2 | Identity recorded, not enforced | Node tag and Pod image checks at fixture, install, work and probe; claimed worker required |
| Q8 | P2 | Log continuity not enforced | Collector readiness and liveness, snapshot containment, fixture identity check, events and control-plane log capture |
| Q9 | P2 | No production Work oracle | `evidence work` with dataset-anchored answer facts |
| Q10 | P2 | Argument tests pass for unrelated failures | Each case asserts its own message and no downstream command or directory |
| Q11 | P3 | Checker cannot read `probes.jsonl` | Reads both formats; runner uses `probes.jsonl` |
| Q12 | – | Missing N7/N12 deterministic tests, incomplete build identity, plan wording | Added tests, platform/tags/toolchain/commands recorded, plan corrected |

Phase 1 re-review (2026-10-03) closed Q3–Q6, Q10 and Q11 and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| R1 | P1 | `install` treated a failed Deployment query as absence | `--ignore-not-found`; any query failure refuses before apply |
| R2 | P1 | Zero-call steps ignored GET, DELETE, notification and response entries | Any server entry carrying the step's invocation fails it |
| R3 | P1 | Answer oracle matched words, including negated or unrelated ones | Clause-level assertions with negation rejection; tighter fact patterns; adversarial tests |
| R4 | P1 | Work mode accepted incomplete or mistimed server proof | Per-invocation windows, zero traffic for denied calls, one session and handler per successful read |
| R5 | P2 | A missing collector log passed continuity | Readable files required; grep error distinguished; collector output frozen before each snapshot |
| R6 | P2 | A worker ignoring a recording error could finish Succeeded | A lost tool record always fails the Work with `tool-evidence-failed` |
| R7 | P2 | Control-plane Pod not rechecked per Work | Identity and image checked before and after each Work |

Phase 1 third review (2026-10-03) closed R1, R2 and R4–R7 and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| T1 | P1 | Positive prose oracle still accepted wrong or reversed facts | Replaced by an exact facts block with dataset-derived values; objectives ask for it |
| T2 | P1 | N8 accepted unrelated incompleteness wording | `timeline_complete` fact derived from the observation cap |
| T3 | P2 | A previous Work's late N6 completion invalidated the next Work | `-prior` reconciliation (completions only) plus a settle wait after timed-out calls |
| T4 | P2 | Delayed session DELETE after a successful outcome was rejected | Completion entries allowed for 2s after the outcome or step, requests never |
| T5 | P2 | A frozen follow log could end mid-record | Only complete records are frozen |

Phase 1 fourth review (2026-10-03) accepted the facts-block approach, closed T4 and T5, and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| U1 | P1 | `-prior` rejected earlier Works' own historical requests in full logs | Prior records carry each invocation's end and reason; history before the end is ignored |
| U2 | P1 | The facts block did not cover the requested fix | Two structured fix keys with dataset-derived values |
| U3 | P2 | The deadline fact was not truly derived from the dataset | Two supported dataset statements; anything else is rejected |
| U4 | P2 | Prior completions had no status, timing or count limits | Bounded windows per invocation; one late handler and one late response, timed-out calls only |
| U5 | P3 | The parser did not identify one authoritative block | Final plain lines only; keys anywhere earlier fail; fenced blocks and trailing text fail |

Phase 1 fifth review (2026-10-03) closed U2 and U3, accepted the facts-block contract for criterion 2, and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| V1 | P1 | A timed-out call's late session-close response broke later checks | Session close is a counted completion; timed-out calls may log it within a minute |
| V2 | P2 | The first 2s after an invocation's end skipped counting and state checks | One policy across both windows: per-kind counts, no late handler except for timed-out calls, nothing at all for denied calls |
| V3 | P2 | The runner picked an invocation's end by string order | Ends compared as instants with nanosecond fractions |
| V4 | P3 | An unclosed fence passed the plain-block rule | An odd number of fences before the block fails |

Phase 1 sixth review (2026-10-03) closed V1–V4, found no regression in earlier rounds and no remaining P1/P2. Its optional P3 (fence parity ignored the delimiter type) is fixed: the active fence's delimiter and length are tracked, and only a matching line closes it.

Phase 1 final-state review (2026-10-03) accepted the committed code, image identity and commit hygiene, and found one P1: the host CLI built by the runner aborted on macOS (`dyld: missing LC_UUID`) because the external-linking workaround depended on the caller's environment. Host Go builds and runs (the CLI and the evidence checker) now go through `host_go`, which links externally on macOS; `build` runs the CLI before recording it, and a runner test builds and runs it.

Phase 2 live check (2026-10-03), before any cluster change, found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L1 | P1 | `protect` exported the old images for the default platform only. Each archive's index still referenced its attestation manifest, which the archive lacked, and its check covered only the config digest. `restore` imports through `kind load image-archive` with `--all-platforms`, which failed with `content digest … not found` when replayed on a fresh kind node. On this node it would have worked only because refs from the 2026-09-20 load still held the old content. | Export with `--all-platforms`. `archive_complete` requires every blob reachable from `index.json` to be in the archive. Protect refuses before scaling; load and restore refuse before importing. Runner tests cover all three gates and the walk itself, and fail against the previous runner. |

Review of the Phase 2 fix (2026-10-03) confirmed L1 for well-formed archives, the port-forward refactor and the `inv-42` isolation, and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L2 | P2 | `archive_complete` treated a descriptor with a missing or unknown media type as a leaf, so a manifest's missing config or layers went unchecked. It also accepted an empty child index. | The walk follows descriptors by role: `index.json` and indexes may name only indexes and manifests, and manifests name a config and a layer list. Each digest must be `sha256:` plus 64 hex characters, and each blob must be `schemaVersion` 2 with its descriptor's media type. Indexes must be non-empty, and at least one non-attestation image manifest is required. Eleven malformed archives each fail for their own reason. |
| L3 | P2 | The controlled-read oracle counted totals only. DELETEs aimed at one session, or an extra handler entry, still passed. | The oracle checks the three calls in order, each `initialize`, initialized, `tools/call`, handler and DELETE in one distinct session, with exact handler outcomes and no other request or handler entry. It also requires one 2xx response per request. The real local fixture log passes, and twelve malformed logs, including both counterexamples, each fail for their own reason. |

Re-review (2026-10-03) confirmed L2 and L3 against their counterexamples, and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L4 | P2 | The walk skipped a digest it had already visited without comparing types, so the same blob named first as a manifest and then as an index passed. | Every descriptor of a digest, including configs and layers, must name the same media type. The runnable-manifest count is taken before the skip, so a manifest named first as an attestation and then as the image is not falsely rejected. Both cases are tested. |
| L5 | P2 | Responses were checked only by count, so a missing response could be offset by a duplicate (DELETE responses replaced by repeated initialize responses still passed). | Each response is matched in log order to the earliest open request with the same HTTP method, RPC method and RPC id. Unmatched responses, unanswered requests and non-2xx statuses fail. Ordering does not require a response before the next request, because the fixture logs a response after its handler returns. Duplicate, mismatched-id and early-response logs are tested. |

Third review (2026-10-04) confirmed L4 and L5 and found no new issue. It also found no regression in the archive gates, the port-forward or the `inv-42` isolation, and accepted the diff for commit and a rebuild in a new campaign directory. The kind import, the controlled read on kind and the cluster-DNS path remain to be proven.

Phase 2 on kind (2026-10-04), after `protect` and `load` succeeded, found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L6 | P1 | The fixture Pod never started. kubelet rejected it with `container has runAsNonRoot and image has non-numeric user (nonroot), cannot verify user is non-root`, because the fixture image ends on `USER nonroot:nonroot`. The probe image ends the same way and the probe Job also sets `runAsNonRoot`, so the probe would have failed later. | Both images end on `USER 65532:65532`, the same user (`nonroot` is 65532:65532 in the distroless base), as the test worker image already does. `TestNonRootManifestsUseNumericImageUsers` fails on a named user and passes on the numeric one. The campaign that stopped is restored and a new one is built from the fix. |

Review of the L6 fix (2026-10-04) found no new issue and no other Pod admission or creation blocker in the fixture or probe manifests. That covers Pod Security Restricted, seccomp, the read-only root and the ConfigMap volumes readable by UID 65532. Whether the fixed Pod starts is proven only by the next campaign on kind.

Campaign c4 on kind (2026-10-04) passed Phase 2: preflight, build, protect, load, the fixture on its recorded image, and the controlled read (three calls, each in its own session; 12 requests, each with one matched 2xx response; log continuity and fixture Pod identity unchanged). That closes the L6 follow-up for the fixture, whose Pod started as UID 65532 under `runAsNonRoot`. The probe image carries the same fix but has not run; the probe Job proves it. c4's Phase 2 does not carry over: the fixes below are a new commit, the image tags and build record follow HEAD, and one campaign never combines evidence from two commits.

Codex's combined review of Phases 1 and 2 (2026-10-04) confirmed the archived Phase 2 evidence and found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| C1 | Blocking before `install` | Validate, plan, the first apply, status and both registrations printed to the terminal only; only the reapply and the final status were kept. | `install` keeps every command's output, command line and exit status per attempt (G5). |
| C2 | Blocking for positive acceptance | Work mode accepted `README.md` reads alone with a correct facts block, and the Portal test accepted any positive-route file. | The positive case needs a successful correlated read of both `logs/timeout.log` and `src/retry.txt`; the Portal test needs both and checks each record. Codex's counterexample is a failing test. |
| C3 | Blocking for positive acceptance | No check joined `resultRef` to the file the server read, so `README.md` references on real timeout and retry reads passed. | Each successful `resultRef` must equal the scope plus the file its `tools/call` named. The counterexample is a failing test. Test logs now use the fixture's own line format and the real `repo:agenova/e16-fixture/<file>` values. |
| C4 | Insufficient evidence | Each runtime image mapping was checked, but the node image list it used was discarded. | Every identity check keeps that list and each line's resolution (G5). c4 archived the fixture mapping by hand. |

Phase 3 preparation in c4 (2026-10-04), before `install`, found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L7 | P1 | `work` captures the claimed worker from before `agenova run`, and every capture had to resolve to the worker config. The first Work creates the warm pool (the control plane's `Configure` on the first Allow), so the worker Pod can be captured Pending or ContainerCreating with an empty `imageID`, which resolves to nothing. A correct positive Work would then fail after it succeeded and could not be recorded again in that campaign. Reproduced offline with the runner's own functions against c4's node image list. | A capture with no container ID and no image ID is skipped. The claimed worker needs at least one started capture, and every started capture must resolve to the recorded config. Tests: a pre-start capture followed by a started one passes; pre-start captures only, a started capture on another image, and a container without an image each fail. c4 ended before `install`, because the fix needs a new commit. |
| L8 | P3 | `parity` stopped `agenova api connect` with one SIGTERM. Its `kubectl port-forward` child kept 127.0.0.1:8088, so the next parity run could not bind and its tests would have run through the earlier run's tunnel. | `parity` refuses to start while 8088 or 5177 has a listener, waits for its own tunnel's forwarding line, stops both servers with every descendant on every exit, including signals (an `EXIT`, `INT` and `TERM` trap, restored afterwards), and fails if a port still has a listener. A stand-in whose child keeps the port reproduces the leak against the previous runner, and so does a SIGTERM while the tests run. |

Codex's review of the fix (2026-10-04) found no P1 or P2 and no weakened gate. Its one P3, that the L8 cleanup ran only on explicit paths so a SIGTERM left both listeners, is fixed as above.

Campaign c5 on kind (2026-10-04), on `1f96467`, was a rehearsal and is not evidence: it ran every step from preflight to the probe once, recorded every defect it reached, and was torn down. Nothing from it goes into `docs/evidence`. Phase 2, install, admission Deny, N6, N7 and the probe (N1–N5, N11) passed first time with their parity archived, and L7 and L8 held. The positive Work and N8 each failed their acceptance twice; each rerun needed an uncommitted runner workaround. It found:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| L9 | P1 | A Work could not be rerun within one campaign. `work` submitted `work-<case>.yaml` unchanged as `e16-<case>`, and the control plane refuses a reused request name (`internal/console/service.go:139-141`), so a rerun would read back the earlier Work. | Fixed: recorded attempts (section 5). `work <case> --attempt N` submits a copy of the request renamed to `e16-<case>-a<N>`, keeps `work/<case>/attempt-<N>/` and refuses an existing attempt. `work show`, the checker (`-ref`, which must equal the evidence's `requestRef`), parity and the Portal spec use the attempt's name. The queried Work must match this invocation's own submission evidence (`run --json`), which Codex's review added (below). Tests: only the name changes; the name reaches run, `work show`, the checker and parity; the second attempt is checked against the first one's records; a recorded attempt is never rerun or changed; bad `--attempt` values and parity for an unrecorded attempt refuse; the checker refuses another attempt's evidence; a refused submission under a name that already holds a Deny, a queried Work with other facts, another outcome or a submission without facts each stop the attempt, while a Deny (run exits 1) and later facts pass. |
| L10 | P2 | A Work that failed after `work show` succeeded (evidence check, worker identity, settle) stopped before parity and before its invocations reached the prior records, so per-run parity depended on the operator, and its late completions would have looked unattributed in a later window. | Fixed: once `work show` succeeds, every remaining check runs, the invocations go to the prior records and parity is archived. The step then fails with the attempt's first failure, reported last even when parity fails too. A check stopped by a signal stops the runner. Tests: a failing checker; a failing worker check followed by a failing checker (the first failure is kept); a failing worker check with failing parity; an interrupted check; a failing `work show` still stops before both. |
| L11 | P1 | 23 of 29 non-final model turns across the six model-driven Works were rejected as invalid tool actions. | Fixed: `ActionSchema` marshals ordered structs instead of a map, so its keys follow `FinishSchema` and the prompt's examples (action, tool, resource, input, answer); the enums are unchanged. Test: the top-level and property key order equal `FinishSchema`'s and the prompt examples', with and without the input enum; it fails against the map version. Cause below. |
| L12 | P2 | The Work objectives and the worker prompt pull in opposite directions: the positive objective asks for both files and an eight-line facts block, while the prompt says reading every input is not required and asks for a concise answer in one JSON string. Of three answers that reached the checker, one block was well formed, one was joined on one line and one was missing, and values were wrong. | Fixed by decisions 14–19 (2026-10-05): E16 runs on `qwen2.5:7b`, the objectives and the worker's truncation line were rewritten from measurement, and each case gets up to five attempts with the first pass counting. The checker is unchanged. |
| L13 | P3 | `kill "$watcher"` makes bash print a "Terminated: 15" job notice into every Work's output. Harmless, but it reads like a failure. | Open. |
| L14 | P3 | `campaign.log` records only the end time of a successful step, so failed steps and durations cannot be read from it. | Open. |

L11 cause. Since Slice 2 (`94a8481`), `ActionSchema` built the schema by marshalling a Go map, so the properties came out alphabetically (action, answer, input, resource, tool); on `main` it was a hand-ordered literal. Ollama generates keys in schema order, and every enum allows `""` so that a finish action fits the same schema. After `"answer":""`, llama3.1 usually left `resource` (often `input` too) empty, and `ParseAction` rejected the action. Finish turns use the hand-ordered `FinishSchema` and were unaffected. Reproduced off-cluster with the real worker and model adapter against the same Ollama daemon and model: 1 of 15 tool turns accepted as sent on kind, 0 of 15 at temperature 0, 15 of 15 with the keys in prompt order and the enums unchanged. The same function builds the schema for the reference demo's mock-tool turns, which the fix covers too. The fix changes the control-plane and worker images.

L12 measurement after the L11 fix (2026-10-04, host only, not kind evidence): 30 whole worker loops per Work against the same Ollama model, judged with the checker's own code. Tool-grammar turns were rejected 9 of 221 times. N6 and N7 read their named file first in every run. The positive Work read both required files every time, but none of its 30 answers passed the facts-block rules; the yes/no facts were mostly wrong as well as malformed. N8 went on to a second fault file after the timeline in 29 of 30 runs, which fails the Work; with only the timeline in its catalog (a diagnostic variant), none of 30 answers ended with the `timeline_complete` line. c6's rules for model answers followed from the measurement below.

L12 decision measurement (2026-10-05, host only, not kind evidence; `.tmp/e16-l12/`). The harness was rebuilt at `3e33fd4` and re-judged the 240 runs above with identical verdicts. A 30-run baseline at `3e33fd4` repeated the result: positive 0 of 30, N8 0 of 30 (a second fault file every time), N6, N7 and the reference demo 30 of 30. Screening cells of 15 runs then varied one factor at a time. With llama3.1, the rewritten objective, a worker line restating the objective on the answer turn, key definitions and the N8 truncation line left positive at 0 of 15 in every cell, and N8 at most 4 of 15 even with a timeline-only catalog. With qwen2.5:7b the remaining failures were key meanings, not reading: `first_attempt_seconds` read as the 5 second per-attempt deadline, `deadline_resets_each_attempt` reversed, and the fix lines copied from the current code. Each was fixed by wording, measured one at a time. The confirmation ran the committed working tree, 30 runs per Work: positive 10 of 30 (95% interval 19–51%), N8 21 of 30 (52–83%), with no second fault file read, N6 30 of 30, N7 30 of 30, token-valid 30 of 30, reference demo 30 of 30. The same positive wording passed 9 of 15 in its screening cell; the attempt rule uses the 30-run figure. Two diagnostics on the confirmed tree were not adopted:
- Sampling temperature 0.2, added by the measurement driver, raised positive to 12 of 15 (55–93%), with both files read every time. The product cannot set it today. The openai-compatible model profile accepts only `model`, and Ollama's `/v1` endpoint ignores a Modelfile temperature: a derived model with `PARAMETER temperature 0.2` still sampled at 1.0. It needs a model-adapter configuration change, left for review.
- The worker tool-turn rule "Read every input the objective names before finishing" gave 7 of 15, still with 4 single-file reads, so decision 19 keeps the rule. qwen2.5:7b sent a tool action with text in `answer` in 148 of 1,108 (13%) of its tool turns; the worker rejects those and the model retries. One measurement cell was void: re-encoding the request body through a Go map to add a temperature sorted the schema keys, which reproduced L11 (0 of 75 valid tool turns), so temperature was not measured.

Codex's review of the post-c5 fix (2026-10-04) found no loosened check and no P1, and one P2 in L9: `work` recorded the submission's exit status but accepted any successful `work show`. If the service already held a Work under the attempt's name (for example from another output directory), the submission was refused with a conflict and `work show` returned the old Work; Codex reproduced an old Deny accepted as attempt 2. Fixed as above: the runner keeps `run --json` and requires the queried Work to continue this submission's own facts, request, decision and outcome, so a conflict, which prints no evidence, stops the attempt. Expected nonzero exits (Deny, N6, N7) still pass. The stale-Deny regression fails against the fix without that check. Codex's re-review confirmed the P2 closed with no new or reopened finding, and accepted the diff for commit and the c6 build; that is not c6 acceptance.

Slice 4 design review (2026-10-04, three lenses before Tom's decision): the per-call API read was confirmed over a Secret volume. The runner rules came out of that review: create Secrets only when absent and never regenerate, generate hex without a newline, capture the worker while it runs because cleanup deletes it, and build the Role rule as `[]any` so an identical reapply matches. So did the scan's handling of empty input, links, zips and base64 alignment, and the spec amendments for R1, R6 and N13/N14.

Slice 4 implementation review (2026-10-04, seven dimensions, each finding checked by a separate verifier) found no P1. Dispositions:

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| S4-1 | P2 | `secrets_test.go` was not gofmt-clean, which fails `check.ps1` | Formatted |
| S4-2 | P2 | The plan did not describe the token Works, the five routes or the final scan | This revision (G4, G7, sections 3, 5, 6 and 9) |
| S4-3 | P3 | `scan` created its directory before its preparation, so a transient kubectl failure blocked any rerun | Preparation runs in `scan.partial`, replaced on a rerun; only the search is once. Test s10 fails against the old order |
| S4-4 | P3 | A scan root that is itself a link was refused by the checker | Roots are resolved with `pwd -P` and extra roots must be absolute; tests s12 and s13 |
| S4-5 | P3 | `bash -x` or `SHELLOPTS=xtrace` would print token values | The runner refuses a traced run before any step; tested for both |
| S4-6 | P3 | Design and spec said 401/403 "at any request", but the closing DELETE's status is ignored | Wording corrected |
| S4-7 | P3 | Design understated the Secret key rule | The Kubernetes key rules are stated |

Found while integrating the parallel work (2026-10-04): the checker had made a rejected-token invocation silent after its end, but the fixture logs its 401 after answering, so that line can follow the recorded outcome and would have failed the next Work. A prior rejected token may now log exactly that 401 response within 2s and nothing else; a prior unresolved token still logs nothing. The probe-mode checker did not yet require `path=/mcp` and `auth=missing`; it does now.

Codex's review of the Slice 4 diff (2026-10-05) found no P1, P2 or P3 against criterion 8 and R1, R6, N13 and N14, found no token leakage path, and accepted the diff for commit and the c6 build. That is not kind acceptance, which c6 still has to show.

Codex's review of the L12 change (`3e33fd4..fd08ac0`, 2026-10-05) found no P1, P2 or P3. It found that the checker, the Go 1.22 baseline, the dependencies and the G4 routes are unchanged, and that the objectives add key meanings without expected values. The five-attempt cap, the first-pass rule and the model preflight passed its adversarial checks. Each new test failed when its fix was reverted. Re-judging the saved measurements gave identical verdicts.

Campaign c6 on kind (2026-10-05) is the formal campaign. It ran at `fd08ac0` with a clean tree.
- Every step passed on its first run, in the section 9 order, with no workaround and no deviation from the runner's procedure:
  - preflight, which recorded `qwen2.5:7b`;
  - build (33s);
  - protect, load, fixture and controlled-read;
  - install: 13 commands exit 0, reapply `changed: false`, Deployment revision 1, one ReplicaSet, restarts 0, and the three access reviews as wanted;
  - the eight Works, each on attempt 1 with parity archived: positive 35s, admission-deny 9s, N6 51s, N7 13s, N8 26s, token-valid 19s, token-missing 13s, token-wrong 14s;
  - probe (8s);
  - the sanitised export;
  - scan: 740 files across the output, the CLI state and the export, with no token value;
  - restore: the reference install Ready on `8d1738b7…` and `0e33be77…`.
- From protect to restore took about 22 minutes, most of it the export and its review.
- Tom approved protect and chose a reduced export. The worker captures, access reviews, Secret records and the reference install's records stay in the local campaign directory ([MANIFEST](../../docs/evidence/178/slice3/c6/MANIFEST.md)), and the scan searched them.
- Three Slice 4 assumptions that only kind could prove held: `kubectl exec` read `/proc/1/environ` as UID 65532, `auth can-i --as` worked under Tom's context, and the agent-sandbox controller added no env, volume or mount to worker Pods.
- Teardown: with Tom's approval, `agenova-e16-system` and `agenova-e16` were deleted at 02:06 UTC with their three Secrets, the fixture and the E16 sandbox template and warm pool. The fixture log collector exited, and ports 8088 and 5177 are free. The reference install stayed Running and Ready.
- Left after c6: there is still no teardown subcommand; L13, L14 and the earlier open items stand.

Codex's review on the PR (2026-10-05, at `bd18010`) raised one P1 and two P2. All three are fixed with tests, and each test fails with its fix reverted.
- **P1.** A grant of an installed operation whose resource scopes match none of its routes passed the operation-only check. The Work's catalog then came out empty, and it could finish without the tool it was granted. The installed service now refuses such a Work with `tool_route_unavailable`, and the CLI shows that code's fixed diagnostic. One routed scope among the grants is enough.
- **P2.** A token reference on the cleartext credential-free `/mcp` URL was accepted, so the token would have gone in cleartext to a route that must never receive one. Validation now rejects it with `token-on-credential-free-endpoint`. One existing test had paired a token reference with `/mcp`; it now uses `/mcp-token`.
- **P2.** If the Work stopped while a configured call waited or ran, the outcome was recorded as `Failed` (`tool-timeout` for the Work deadline). It is now `Cancelled` with `tool-call-cancelled`, because the external effect is unknown. A call's own timeout, with the Work still live, stays `tool-timeout`.

A further Codex review at `8d77bde` raised one more P2. The MCP client returned the scope and file as `resultRef` even when they held a character the shared result-reference contract rejects, such as a space, `?`, `#` or `@`. `Set.Invoke` then turned a successful read into a failed one. The client now omits such a reference, by the rule `toolbackend.ValidResultRef`, which `Set.Invoke` uses too. `TestMCPClientOmitsAResultRefTheContractRejects` drives the real client through the Set, and it fails without the fix. Every c6 scope and file forms a valid reference.

Left open by these fixes:
- **L15.** The evidence checker and the runner allow a late server handler and response only after `tool-timeout`, and `settle_timeouts` waits only for those. A Work stopped during a call is now `tool-call-cancelled`, so a later attempt could fail on that call's late server entries. No campaign case stops a Work during a call.
- **L16.** The synthetic `git.read` composition still admits a `git.read` grant with no resource scope, which yields an empty catalog. This is the reference path, and it predates E16.

None of these paths ran in c6: every c6 grant had a routed scope, the token backends use `/mcp-token`, and no Work stopped during a call. c6's result therefore stands for the changed code. The full gate, the race tests on the changed packages, the tagged bridge and the probe dry run pass.

Codex's review at `a85c342` (2026-10-06) raised two more P2. Both are fixed with tests that fail with the fix reverted.
- **P2.** The synthetic `git.read` catalog has one entry per granted scope, so a grant of more than `workerprotocol.MaxTools` (32) scopes was admitted and allocated, and failed only when the worker schema was built. A configured catalog holds at most 32 routes but can still exceed the schema budget with long scopes. The service now builds the Work's catalog schema at submission and refuses a Work it cannot carry with `tool_catalog_unsupported`, before a claim is journalled or a worker allocated; the CLI shows that code's fixed diagnostic.
- **P2.** The result-reference rule rejected only `://`, so `file:/etc/passwd` passed, and the MCP client builds exactly that from a scope `file:` and a file `etc/passwd`. `facts.ValidResultRef` now also rejects a leading `//`, a URI scheme followed by `/`, and the WHATWG special schemes plus `data` and `javascript`; scope kinds such as `repo:` stay valid. `toolbackend.ValidResultRef` now calls it, so the two rules cannot drift.

c6 granted at most one scope per Work, and every c6 reference starts with `repo:`, so c6's result stands for these changes too.

Codex's review at `a1b0de5` (2026-10-06) raised one more P2: the MCP client sent `tools/call` even when the server's `initialize` result did not declare the `tools` capability. The handshake now fails with a protocol error before any `tools/call` when that capability is absent or null; `TestMCPClientHandshakeFailuresStopBeforeToolCall` covers both and fails without the fix. The official-SDK fixture declares the capability, and the client still interoperates with it, so c6's result stands.
