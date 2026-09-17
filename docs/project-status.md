# Implementation Evidence Snapshot

Updated: 2026-09-17

This records what the repository currently proves, not the target architecture or mutable ticket priority. It does not track ticket owners, readiness or sequence. Product requirements live in the [PRD](product/prd.md); ticket status and dependencies live in GitHub. See the [AIDLC source-of-truth map](development/AIDLC.md).

## Verified Reference Path

The supported operator path is `adapters catalog` followed by `platform validate/plan/apply/status`, create-only `policy apply` and `agent-template apply`, then `run -f`. The [kind + Ollama CLI guide](reference-cli-kind-ollama.md) gives exact prerequisites and commands. `platform apply` installs the reference Control Plane service on an existing kind cluster; it does not create kind or install the upstream Agent Sandbox controller. The effective Platform revision selects the supported Kubernetes deployment, Agent Sandbox runtime, and OpenAI-compatible Ollama model configuration. `platform status` reports installation/readiness and adapter availability, not proof that a Work has succeeded.

The reference service accepts the canonical ClaimRequest through its trusted application boundary. The registered PolicyBundle and AgentTemplate drive admission, effective authority, worker artifact and profiles. One allowed Team A run was verified on kind with a real Agent Sandbox worker and Ollama inference. The CLI reports decision, claim, outcome/failure and model usage; `--json` returns the shared evidence view. The documented out-of-policy request denies before worker allocation or provider use. Identical registration reapply is idempotent; conflicting identity/content fails. The Policy and template registry survives a service restart via cluster ConfigMaps. The full repository gate, including frontend contracts and browser smoke, passes for this slice. Reproduction details are under [`evidence/165/`](evidence/165/).

## Other Implemented Surfaces

- Backend-neutral ClaimRequest, AgentTemplate, SandboxClaim lifecycle, RuntimeBackend contract, trusted-principal reference authorization and system-issued effective authority.
- In-memory runtime and governance reference path, isolated behind explicit `--backend memory` for CLI Work.
- Claim-scoped Tool and Model authorization/facts, evidence projection, and a controlled-worker ReAct demonstration. Example tool content in the kind/Ollama path is labelled synthetic.
- Bundled adapter catalog and exact version activation for the supported deployment, runtime and model adapters.
- React Portal with fixture and connected modes, bounded polling, lifecycle/outcome/authority evidence and browser smoke coverage. Connected mode requires the installed Platform identity and revision from `/api/setup`; the old standalone demo service is not accepted as an installed Platform.

## Limits and Unverified Claims

- The installed-service CLI submits and queries (`work list/show`) through the same private HTTP API used by local `npm dev` through `agenova api connect`. The reference API is bound through a loopback-only Kubernetes port-forward, not published as a public Service. CLI and UI evidence parity for allowed and denied Work was checked with real kind/Ollama plus the opt-in installed Playwright test (see [`evidence/167/`](evidence/167/)).
- Active Work and evidence are in one Control Plane process. Restart or Platform rollout during a run can interrupt it and lose historical evidence; no draining, durable handoff, HA or production upgrade guarantee exists.
- The reference principal is fixed by service configuration. Kubernetes RBAC protects operator registration, but loopback/port access alone is not production authentication or tenancy. There is no external identity-provider integration.
- The verified real-provider path is one compatible Agent Sandbox worker plus local Ollama. Arbitrary agent images, production-grade adapter/plugin lifecycle, public-provider credential handling, real Tool/Memory/Observability backends and network-enforced gateway isolation are not proven.
- Backend-neutral runtime semantics beyond the supported kind path, complete cross-claim gateway correlation, filesystem/isolation proof, persistent history and production-authorized Work listing remain separate work. The reference list is limited to the current service session and fixed local Team A identity.

## Next Delivery Slice

Harden the reference API with production identity and tenancy, durable Work/evidence across rollout, and backend-independent deployment/access packaging. Those remain separate follow-up tickets; `platform apply` intentionally does not install the local React UI.
