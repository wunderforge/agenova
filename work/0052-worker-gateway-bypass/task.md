# Task: Verify the Worker gateway-bypass boundary on kind

- Ticket: [#52](https://github.com/wunderforge/agenova/issues/52)
- Mission: Determine, with worker-side network and credential evidence and an oracle outside Agenova, whether a claim worker on the kind reference can reach the configured model provider or another protected endpoint without going through Agenova, and record the result without remediating it.
- Target: `harness/security/` (new probe and oracle scripts, reused by #198/#199), `docs/evidence/52/`, `docs/backends/agent-sandbox.md` Known Gaps, `docs/project-status.md` claim wording. No product code.
- User value: The final demo sentence "the worker cannot bypass the Gateway" is either backed by reproducible evidence on kind or replaced by an exact, recorded limit.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture contract: Authority and Credentials](../../docs/product/architecture-contract.md#authority-and-credentials)
- #194 security claim matrix, rows AC-4, AC-6, AC-7, AC-8, AC-9, ES-5, RI-2, TM-1, TM-3, plus "Meaning of the worker cannot bypass the Gateway" and Disclosure (`work/0194-security-claim-matrix/spec.md`, PR #195, not yet merged)
- [Agent Sandbox backend note](../../docs/backends/agent-sandbox.md) and [#51 kind evidence](../../docs/evidence/51/kind-run.md)
- [#167 installed run evidence](../../docs/evidence/167/summary.md) for the governed-call commands
- [Reference Platform for kind](../../deploy/reference/platform.kind.yaml) (provider endpoint `http://host.docker.internal:11434/v1`)
- [Test worker image](../../harness/integration/agentsandbox/testworker/Dockerfile) (`alpine:3.20`; busybox `nc`, `wget`, `nslookup` available in the worker container)
- #52 comments: owner calibration (16 Sep), Richard's plan (17 Sep), yanyang's review (20 Sep), wuy10 additions (27 Sep)

## Scope

In scope:

- Baseline of the reference install: tool versions, kind node image, CNI (kindnet) version, Agent Sandbox version, NetworkPolicy objects, the rendered SandboxTemplate, and the worker Pod spec (SA, token automount, env names, volumes).
- Network probes from inside a claim worker and a warm-pool worker to: Ollama (`host.docker.internal:11434`, by DNS and by resolved IP), control-plane Service `:8080` and Pod IP `:8080`/`:8081`, Kubernetes API (`kubernetes.default.svc:443` and its ClusterIP), `169.254.169.254:80`, kube-dns `:53`, and one public HTTPS endpoint.
- Controls (see Probe Design) so that "blocked" is distinguishable from "unreachable target" or "broken probe".
- Credential exposure scan in the worker (env, `/proc/1/environ`, mounts, `/var/run/secrets`, readable config paths) with two synthetic canaries; names and paths only, never secret values.
- Exposure audit of the installed Deployment, ConfigMaps, Secrets (names/keys only) and local CLI config for admin credentials (RI-2).
- Reusable scripts in `harness/security/`: worker probe, Ollama request counter, canary scan.
- Documentation of the result: evidence file, Known Gaps entry, and claim wording in `docs/project-status.md`.
- One follow-up issue per open gap, for the owning Epic (drafted for owner approval before filing).

Out of scope:

- Any remediation: NetworkPolicy changes, CNI swap, template rendering changes, workload identity (#121).
- Filesystem layout, mounts as isolation, cross-claim filesystem isolation (#51).
- Cross-claim forgery and prompt-injection authority tests (#198); failure injection (#199).
- EKS / IMDS / IRSA probes (E15-EKS after #175). No kind result is claimed for EKS.
- Editing the #194 matrix (separate branch); rows are linked after PR #195 merges.
- Container escape, port scanning beyond the listed targets, DoS.

## Acceptance Criteria

- A pinned baseline transcript of the reference install and the actual worker Pod (identity, SA, token automount, env names, volumes, NetworkPolicy objects and the CNI in use) is committed under `docs/evidence/52/`.
- A governed `agenova run` succeeds through the Gateway, and the Ollama request count from the host-side server log rises by the expected amount (positive control for the provider oracle).
- For every target, the evidence records the result from the worker, from the non-worker control Pod, and from the host where applicable, by DNS and by IP, with the exact safe command and the Ollama request count before and after the worker probes.
- Each target is classified as exactly one of: blocked by a named mechanism (shown by a control where the same target is reachable without that mechanism); reachable / open gap; or inconclusive for an environment reason. A NetworkPolicy object alone is never classified as enforcement.
- The planted in-worker canary is found by the scan, and the control-plane-side canary is not visible in the worker (AC-6, AC-7, AC-8).
- A warm-pool worker, before binding, is shown to hold no usable credential and its reachable endpoints are recorded (AC-4 warm/network part, AC-6).
- Worker-side ES-5 result (`:8081` and `:8080` on the control-plane Pod) and the RI-2 exposure audit are recorded.
- `docs/backends/agent-sandbox.md` Known Gaps and `docs/project-status.md` state the observed result with the exact scope "kind reference only".

## Negative Case

- A direct worker connection to Ollama that produces zero new Ollama log entries counts as blocked only if the same probe from the control Pod or host raises the count; otherwise it is inconclusive.
- A probe target that is unreachable from the control Pod and the host is reported as inconclusive, not blocked.
- A scan that does not find the planted canary invalidates the "no credential found" result.
- Public egress (TM-1) being open is recorded as an exfiltration gap with a follow-up issue, not as a Gateway bypass.
- The experimental default-deny NetworkPolicy run, if performed, is labelled as a second probe and never described as an Agenova Platform capability.

## Probe Design

| Probe | Purpose |
|---|---|
| Governed run (`agenova run -f deploy/reference/demo/work.yaml`) | Positive control for the Gateway path and the Ollama counter. |
| Same targets from a control Pod (same namespace, no worker labels, not selected by the generated policy) | Separates "blocked for the worker" from "unreachable in this cluster". |
| Same Ollama request from the macOS host (`curl 127.0.0.1:11434/api/tags`) | Proves Ollama is up and the counter moves. |
| Public IP `1.1.1.1:443` from the worker | Allowed by the generated policy; proves the worker's probe tooling works when a path is open. (kube-dns is not usable here: `10.96.0.10` falls inside the policy's excluded `10.0.0.0/8`.) |
| By-DNS and by-IP variants | Separates DNS failure from connection blocking. The worker uses `dnsPolicy: None` with public resolvers, so cluster names never resolve; by-IP results decide. |
| Unique request path per probe (`/e15-<round>-<pod>`) | Docker Desktop shows every client as `127.0.0.1` in the Ollama log, so probes are attributed by path, not by source address. |
| Startup probe on a freshly created worker (`startup-window.sh`) | Checks whether the generated policy already applies at the worker's first request, not only after it has been running. |
| Second probe: experimental default-deny egress NetworkPolicy, created and removed by the harness | Shows whether kindnet enforces any policy at all, so a "blocked" result can name its mechanism; labelled experimental (yanyang, 20 Sep). |

Worker source: a worker bound to a real Agenova claim during `agenova run` when it stays Running long enough to exec into; otherwise a `SandboxClaim` created directly on the Agenova-generated template `agenova-tmpl-engineer`, with a recorded Pod-spec comparison against a real run's worker. The evidence states which was used.

Oracles outside Agenova: Ollama server log line counts on the host (`~/.ollama/logs/server.log`), connection results seen by `nc`/`wget` with exit codes, `kubectl get -o yaml` Pod spec, and the canary scan output.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [x] Bring up the environment: Docker Desktop, `kind get clusters` shows `agenova-k8s-lab`, `bash harness/local/macos-bootstrap.sh verify`; pin versions.
- [x] Write `harness/security/` scripts (probe, Ollama counter, canary scan) with an explicit kube context and namespace, read-only except for the labelled control Pods, experimental policy, held claim and canary they create and delete, and the temporary control-plane canary env var they revert.
- [x] Record the baseline transcript.
- [x] Run the governed positive control and record the Ollama count delta.
- [x] Run probes from the claim worker, warm-pool worker, control Pod and host; record counts.
- [x] Run the canary scan and the RI-2 / ES-5 audits.
- [x] Classify each target; apply the Disclosure rule before writing any public evidence.
- [x] Experimental default-deny second probe, with creation and cleanup recorded.
- [ ] Write `docs/evidence/52/`, Known Gaps and status wording; draft follow-up issues for owner approval.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `bash -n harness/security/*.sh` and `shellcheck harness/security/*.sh` when available
- `bash harness/security/probe-worker.sh --context kind-agenova-k8s-lab --namespace agenova-system` (real kind run; output committed as evidence)
- `.\scripts\check.ps1 -Docs`
- `.\scripts\check.ps1 -All` (known macOS-only failure `ui/smoke/console.spec.ts:74`, passes on CI)

## Evidence Required

- `docs/evidence/52/` transcript: versions, baseline, per-target result table (worker / control Pod / host, DNS / IP), Ollama counts before and after, canary scan output, warm-pool result, ES-5 and RI-2 audit, cleanup confirmation.
- The governed run's claim ID and outcome next to the Ollama count delta.
- Per-target classification with the named mechanism, the open gap, or the environment reason.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Report only; no fixes in this ticket (#177 maintainer decision, 2 October 2026).
- Disposable, explicitly selected cluster only: every command passes `--context kind-agenova-k8s-lab`; no shared environment.
- Never print secret values; scans report names, paths and canary matches only.
- Do not weaken `AGENOVA_ALLOWED_WORKER_IMAGE`; probes use the admitted worker image.
- Disclosure: if a worker reaches a protected backend (Ollama, Kubernetes API, control plane, metadata) without a Gateway decision and the finding is new, the reproduction is sent to the maintainer privately before anything is published, and public files state only the finding, its matrix row and its status. Unenforced NetworkPolicy on kindnet is already listed as unproven in `docs/project-status.md`; the maintainer decides borderline cases.

## Decisions and Blockers

- Planning depth: Task. The method is fixed by the #194 matrix rows and the #52 comments; no new spec or design.
- Starts from merged controlled-worker path (#134/#136/#144) without waiting for #51 (board reconciliation note in the #52 body, 21 Sep).
- The meaning of "bypass the Gateway" follows the owner calibration on #52 (16 Sep) and the #194 spec: public egress is TM-1 exfiltration, not bypass, on kind.
- Branch was cut from `main` at `c56ba3a` and rebased onto `ebb4c04` after PR #195 merged; matrix links are added in a follow-up change.
- Private vulnerability reporting is disabled on the repository; high-severity findings go to the maintainer privately until it is enabled.
- Assignee decisions (5 Oct): probe a worker bound by a real `agenova run` first, falling back to a `SandboxClaim` on `agenova-tmpl-engineer` with a recorded Pod-spec comparison; run the experimental default-deny second probe.
- First run (5 Oct): a real `agenova run` ends before a full probe completes (one bound-worker datapoint only), so the full probe used a held `SandboxClaim` with the same `sandboxTemplateRef` and `warmpool` as the adapter. The control-plane canary needs a temporary env var on the control-plane Deployment; it was reverted and `platform status` returned `changes: 0`.
- Disclosure (5–6 Oct): the AC-9 startup result was sent to the maintainer privately under the #194 Disclosure rule. The maintainer confirmed it as a real vulnerability and asked for a public issue: #208. Public egress (TM-1) is #202.
- Related: #201 (E13) owns the EKS worker egress boundary; the kind public-egress result (TM-1) is filed separately.
- Dates: packet 9 Oct, probe results 16 Oct, conclusion and docs 23 Oct 2026.
