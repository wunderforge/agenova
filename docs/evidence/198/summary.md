# #198 kind claim-boundary evidence

- Run date: 19 October 2026 (Australia/Melbourne; raw timestamps are UTC).
- Baseline: `ae24a67`, with the uncommitted test harness in this packet.
- Context: `kind-agenova-k8s-lab`; disposable namespaces `agenova-e15-198-a` and `agenova-e15-198-b`.
- Worker: digest in [`runtime/environment.json`](runtime/environment.json); reference layout, UID 65532, controlled-v1. No product source was changed.
- Status: **partial evidence; does not close #198**. No authority leak was observed in this campaign. Required Gateway-denial evidence, same-install two-Running-claim evidence and model-originated injection remain unproven.

## Results

| Scenario | Ollama delta | Recorded model attempts | Phase | Outcome code |
|---|---:|---:|---|---|
| control | 2 | 2 | Succeeded | run-Succeeded |
| profile | 1 | 1 | Failed | execution-failed |
| tool | 1 | 1 | Failed | execution-failed |
| scope | 1 | 1 | Failed | execution-failed |
| malformed | 1 | 1 | Failed | execution-failed |
| unsupported | 1 | 1 | Failed | execution-failed |
| turn-limit | 6 | 6 | Failed | agent-turn-limit |
| cross-claim | 1 | 1 | Failed | execution-failed |
| after-expiry | 1 | 1 | Expired | run-deadline |
| after-failure | 1 | 1 | Failed | agent-invalid-final-result |

Every negative worker scenario deliberately performs one allowed model call first. Thus delta 1 is its positive control, with **zero additional provider calls**, not zero total calls. The turn-limit scenario performs six allowed calls; the seventh is refused. The control has no tool grant and returns the model-selected final answer after two model turns; it reaches Succeeded.

Ollama's independent counter counts `[GIN]` entries containing `/v1/chat/completions` in the host log. The measurement assumes no unrelated Ollama chat traffic during each window. CLI/API evidence and the counter agree; no concurrent unrelated traffic was deliberately generated. A counter alone cannot show which rejected operation or guard caused a failure.

- `cross-claim` targets the successful control claim in the **same** service session. The target's facts before/after are identical (control.show.json versus cross-claim.target-after.json). B is terminal, not Running or Pending.
- `cross-install` targets B while B is Running in a second install. A fails; delta is 1 (its initial allowed call); B remains Running and its facts are unchanged. See [`runtime/cross-install-counts.json`](runtime/cross-install-counts.json). B is unknown to A's authority store, so this is not evidence for two Running claims known to the same control plane.
- `profile`, `tool`, `scope`, `malformed` and `unsupported` produce generic `execution-failed`, with no recorded invocation Deny. Source inspection locates pre-Gateway guards in `exchangeWorker`. Tool and scope attempts can hit the model-action correspondence guard first; these runs do not isolate resource containment or prove a Gateway policy evaluation.
- `after-expiry` reaches Expired and has no second provider attempt. The session is cancelled before the delayed second operation can be observed; a post-terminal operation rejection is **not proven**.
- `after-failure` returns an unmatched final result and attempts another operation. It fails with `agent-invalid-final-result`. The claim is still Running when the invalid result is processed; this is **not** a direct operation against an already Failed claim.
- The unapproved billing project yields a recorded admission Deny, no Agenova claim/backend identity in the view and delta 0. The initial campaign lacked a continuous Kubernetes-object watch. A subsequent [supplement](supplement/summary.md) adds zero-event watches for the denial interval and successful creation detection as a positive control.
- Synthetic `apiKey` input is refused locally by the CLI. Sending the same bytes directly to the private API also yields HTTP 400 (`invalid_request`), delta 0; see [`runtime/credential-canary.api.json`](runtime/credential-canary.api.json). A subsequent [supplement](supplement/summary.md) adds a positive-controlled residual scan in the isolated namespace, agent environment and current logs; this is not a cluster-wide absence claim.

## Acceptance coverage and remaining work

| Requirement / matrix | Evidence | Remaining limitation |
|---|---|---|
| AC-5 cross-claim | Same-install terminal target; second-install Running target, no target facts or excess provider calls | Two known Running claims in one service not exercised; reference service serializes execution |
| AC-1 / SR-4 Gateway Deny with policy and reason | Forged worker operations are blocked before Gateway | No Gateway Deny, no actual model/tool-originated injection; not satisfied |
| AC-4 terminal operations | Cancellation stops further provider activity; existing Go tests exercise inactive-claim Gateway behavior | No observed post-Succeeded/Failed/Expired request on installed path |
| FL-1 turn bound | Six real calls; seventh refused with agent-turn-limit | Proven only on this reference path |
| SR-3 | Admission Deny and zero provider calls | A later positive-controlled watch proves no observed allocation during its bounded interval |
| AC-3 | CLI refusal plus private API 400 and zero provider calls | Bounded positive-controlled scan completed in supplement/summary.md |
| AC-2 policy narrowing re-test | Current schema rejects constraints, active Policy unchanged; template narrowing independently verified | Policy capability constraints remain unimplemented, tracked by E14 #204 |
| ES-1 decision evidence | CLI JSON and API rejection response captured | Generic execution-failed omits rejected operation/category; real Succeeded/Failed/Denied pages and template comparison verified in supplement/summary.md |

The scripted worker exercises A1 (worker-originated operations). Its preceding real model call does **not** turn a scripted operation into A2 prompt-injection evidence. The reference action schema limits the real worker to git.read plus a file input, and the worker pins resource scope/profile. A deterministic injection case that actually reaches the Gateway still needs a supported test method; no product restriction was weakened here.

## Replay

1. Build CLI and the test image from this checkout:

```bash
go build -o .tmp/agenova ./cmd/agenova
docker build -f harness/security/scriptedworker/Dockerfile -t agenova-e15-scriptedworker:kind .
kind load docker-image agenova-e15-scriptedworker:kind --name agenova-k8s-lab
```

2. Resolve the built image's OCI index digest with `docker image inspect`. On the kind node, add a containerd image reference `docker.io/library/agenova-e15-scriptedworker@sha256:<digest>` pointing to that same image (do not replace the reference worker tag). The captured platform/template files use the exact digest of this run; substitute the newly built digest when replaying.
3. Validate, plan and apply each captured platform with a separate `--state-dir`; register captured engineer.yaml in each. The captured files select the explicit test context/namespaces, controlled-v1 protocol and matching pinned image. Only use disposable namespaces that do not already exist.
4. Run sequentially with no other Ollama chat traffic:

```bash
python3 harness/security/run-claim-escape.py --state-dir .tmp/e15-198/state-a --output .tmp/e15-198/replay
python3 harness/security/run-claim-escape-extra.py --state-a .tmp/e15-198/state-a --state-b .tmp/e15-198/state-b --output .tmp/e15-198/replay
```

5. For the server-side canary check, open `agenova api connect --state-dir .tmp/e15-198/state-a --port 18089` and POST the captured credential-canary.request.json to the loopback `/api/requests`. Save HTTP status/body and fresh before/after counts. The primary script's CLI canary check alone is not server-side evidence.
6. Save evidence before restarting/deleting any service: storage is in-process. Delete only namespaces this replay created, with their recorded UID preconditions.

## Cleanup and validation

Both test namespaces were deleted with UID preconditions after capture. The original `agenova-system` installation was not reconfigured: [`runtime/reference-status-after.json`](runtime/reference-status-after.json) reports no changes and installationReady=true. Test images remain cached on the local node; no registry publication occurred.

Focused Go tests for the scripted worker, workerprotocol, Agent Sandbox execution, console and both Gateways passed. Full-gate results are recorded in validation.txt. Findings are drafted in follow-up-draft.md and have not been published.

## Follow-up feasibility analysis

[blocker-analysis.md](blocker-analysis.md) records the two independent serialization locks, the absence of a supported model-selected profile/scope route to Gateway Deny, and the terminal request/evidence distinction. Supplementary public-seam tests now confirm all three terminal phases refuse model/tool requests with a fresh context, but record no new decision fact. They also confirm the Gateway can produce policy-attributed Deny when called through an injected Executor. These are explicitly in-memory composition tests; the installed-kind acceptance gaps above remain open. Exact race-test output is in [boundary-probe.log](boundary-probe.log). A concrete maintainer comment is staged in ticket-comment-draft.md, not published.

## Supplement completed

[Supplementary report](supplement/summary.md): continuous denial-allocation watches with successful-creation control, positive-controlled bounded canary scanning, real CLI/API/UI parity and visually inspected screenshots are complete. Policy constraint schema rejection is now recorded; actual Policy narrowing remains unavailable in this baseline and is tracked by E14 #204. The three core acceptance blockers remain unchanged.
