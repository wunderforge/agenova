# Implementation Evidence Snapshot

Updated: 2026-10-23

This records what the repository currently proves, not the target architecture or mutable ticket priority. It does not track ticket owners, readiness or sequence. Product requirements live in the [PRD](product/prd.md); ticket status and dependencies live in GitHub. See the [AIDLC source-of-truth map](development/AIDLC.md).

## Verified Shared Member Trial

In-progress #256 follow-up: optional agent-reported completion, recoverable
reference-agent tool errors, original-Pod native HTTPS observations and
walkthrough UI corrections. Focused Go/browser and real PostgreSQL checks pass;
isolated kind/Calico same-image/node/IP allow/deny checks pass with deterministic
model-response fixtures. Real-model AWS acceptance, independent PR review and
deployment remain pending. See [task packet](../work/0256-demo-security-completion/task.md).

Local-only owner acceptance follow-up (23 October 2026, uncommitted): the
permission-first Work detail and deterministic Researcher missing-tool correction
are implemented on `codex/final-demo-product-local`. UI/SQL/Gateway/MCP checks
remain separate from shared EKS, real Google/model and native network evidence.
See [local acceptance](development/local-product-acceptance.md). This does not
change the deployed-version claims below.

Local display follow-up: failed Work now puts its recorded refusal above
Permissions as a compact amber `<target> blocked` / `Evidence` banner. Build,
161 UI unit tests, 83 browser smoke tests and nine local API/MCP journeys passed;
owner acceptance remains pending. No commit, merge or deployment.

The 19 October shared EKS trial ran the integrated member/Tool/diagnosis candidate, delivered by real GitHub Actions OIDC/ECR paired rollout, at source `44dc383c9d8cb1ed5679677d78ed6a289a1944bc`. Personal Google application identity, approved project membership, role checks, project-scoped immutable registration and SQL-backed Work/evidence are present in this mode.

Real Agenova CLI registration, one allowed Agent Sandbox Work with three successful Bedrock calls, one admission denial with no Claim/provider allocation, post-run query, retained history and identical-request non-replay were verified. AWS CLI independently matched both immutable ECR image digests; the executed Worker was absent after cleanup. See the [trial commands](development/shared-eks-trial.md) and [exact acceptance evidence](../work/0218-demo-cicd-integration/aws-acceptance-20261019.md).

On 20 October, the control plane was updated to the exact #234 merge,
`53f584235e2ff43c629e6170630a1b99da710727`, with the June Portal, public landing,
schema 009 and the reviewed proxy configuration. The Worker and Platform registry
were unchanged; later main/#231 Worker changes were not part of this release.
The deployment and both containers were healthy. Live HTTPS checks passed for
the landing, existing Google login entry, anonymous private-API 401 responses,
desktop/mobile rendering and absence of page errors. The frontend passed 132 unit
tests and 61 Playwright tests; PR CI and the exact merged-source Linux gate passed.
A fresh real Google login and authenticated live Work submission were not
performed in this UI acceptance. See the
[release evidence and schema-compatible recovery image](https://github.com/wunderforge/agenova/pull/234#issuecomment-6282259695).

The later #241 release enabled guarded automatic main control-plane deployment
and separated user Worker image selection from that pipeline. Its shared-cluster
A/B/A image acceptance and negative cases are recorded in the
[sanitized release evidence](https://github.com/wunderforge/agenova/pull/241#issuecomment-6287334683).
Worker image publication does not itself require rebuilding the control plane.

This does not certify production security or all adapters. Real external MCP,
general credential-store integration, HA/recovery and hostile-agent/network
isolation require their own acceptance. Temporary migration Jobs and Secrets were
removed. Temporary candidate branch access was revoked.

The #242/#243 integrated candidate adds real MCP composition and explicit
per-Work network intent/ceiling/Policy resolution, startup-deny configuration and
allocation-bound activation/revocation. At this documentation checkpoint it is
local implementation, **not an AWS isolation result**. See the
[MCP acceptance](../work/0242-mcp-incident-demo/acceptance.md) and
[network task gates](../work/0243-work-network-authority/task.md).

## Verified Reference Path

The supported operator path is `adapters catalog` followed by `platform validate/plan/apply/status`, create-only `policy apply` and `agent-template apply`, then `run -f`. The [kind + Ollama CLI guide](reference-cli-kind-ollama.md) gives exact prerequisites and commands. `platform apply` installs the reference Control Plane service on an existing kind cluster; it does not create kind or install the upstream Agent Sandbox controller. The effective Platform revision selects the supported Kubernetes deployment, Agent Sandbox runtime, and OpenAI-compatible Ollama model configuration. `platform status` reports installation/readiness and adapter availability, not proof that a Work has succeeded.

The reference service accepts the canonical ClaimRequest through its trusted application boundary. The registered PolicyBundle and AgentTemplate drive admission, effective authority, worker artifact and profiles. One allowed Team A run was verified on kind with a real Agent Sandbox worker and Ollama inference. The CLI reports decision, claim, outcome/failure and model usage; `--json` returns the shared evidence view. The documented out-of-policy request denies before worker allocation or provider use. Identical registration reapply is idempotent; conflicting identity/content fails. The Policy and template registry survives a service restart via cluster ConfigMaps. The full repository gate, including frontend contracts and browser smoke, passes for this slice. Reproduction details are under [`evidence/165/`](evidence/165/).

## Other Implemented Surfaces

- Deployed member-service public landing: anonymous root serves finite public product
  assets, Sign in uses the unchanged `/login` form/Google flow, and authenticated
  root retains Connected entry. All private APIs remain gated. Login palette now
  matches June. Deployed and publicly checked in the UI release above.
  [Acceptance scope](../work/0215-invited-team-access/landing.md).
- June Portal visual refresh: fixed-light surfaces, loaded-list summary/activity chart,
  separate admission/outcome columns, and a selected-record inspector backed by
  the existing activity projection, account menus, quick guide and view settings.
  No API/authentication contract change. Deployed in the UI release above;
  [scope and local evidence](../work/0143-midterm-vertical/june-portal.md).
- Backend-neutral ClaimRequest, AgentTemplate, SandboxClaim lifecycle, RuntimeBackend contract, trusted-principal reference authorization and system-issued effective authority.
- In-memory runtime and governance reference path, isolated behind explicit `--backend memory` for CLI Work.
- Claim-scoped Tool and Model authorization/facts, evidence projection, and a controlled-worker ReAct demonstration. Example tool content in the kind/Ollama path is labelled synthetic.
- Bundled adapter catalog and exact version activation for the supported deployment, runtime and model adapters.
- React Portal with fixture and connected modes, bounded polling, lifecycle/outcome/authority evidence and browser smoke coverage. Connected mode requires the installed Platform identity and revision from `/api/setup`; the old standalone demo service is not accepted as an installed Platform.

## Limits and Unverified Claims

- The installed-service CLI submits and queries (`work list/show`) through the same private HTTP API used by local `npm dev` through `agenova api connect`. The reference API is bound through a loopback-only Kubernetes port-forward, not published as a public Service. CLI and UI evidence parity for allowed and denied Work was checked with real kind/Ollama plus the opt-in installed Playwright test (see [`evidence/167/`](evidence/167/)).
- In reference mode, active Work and evidence are in one Control Plane process. The member mode has SQL-backed history, but general draining, durable execution handoff, HA and production upgrade acceptance are not established by this single rollout.
- The reference principal is fixed by service configuration. Kubernetes RBAC protects reference operator registration, but loopback/port access alone is not production authentication or tenancy. The separate member mode uses Google login and approved project membership.
- Verified real-provider paths include the documented kind/Ollama reference and the shared EKS/Bedrock member trial above. Arbitrary agent images, production-grade adapter/plugin lifecycle, complete credential security, real Tool/Memory/Observability backends and network-enforced gateway isolation are not proven by these runs.
- Complete cross-claim gateway correlation and filesystem/isolation proof remain separate work. The reference list is limited to the current service session and fixed local Team A identity; member-mode listing uses current membership and stored Work history. Backend-specific acceptance must not be inferred for untested runtime implementations.

## Next Delivery Slice

Complete the remaining recovery/revocation acceptance, then connect teammate-owned external Tool/Observability services with real-provider and security evidence. Schema 009/proxy and the June UI are deployed; their release does not establish the remaining operational/security acceptance. Do not recreate the already demonstrated shared CLI/model path or treat Tool application integration as external-service deployment. `platform apply` intentionally does not install the local React UI.
