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
- React Portal with fixture and connected modes, bounded polling, lifecycle/outcome/authority evidence and browser smoke coverage. A local demo service can feed connected mode, but it is not yet the installed kind service's shared API endpoint.

## Limits and Unverified Claims

- The installed-service CLI currently submits and polls through a Kubernetes-authenticated private command transport. There is no documented host-accessible Agenova API address for both CLI queries and `npm dev` UI, and no standalone `work show` command yet. Do not present the local UI and installed CLI as one live source.
- Active Work and evidence are in one Control Plane process. Restart or Platform rollout during a run can interrupt it and lose historical evidence; no draining, durable handoff, HA or production upgrade guarantee exists.
- The reference principal is fixed by service configuration. Kubernetes RBAC protects operator registration, but loopback/port access alone is not production authentication or tenancy. There is no external identity-provider integration.
- The verified real-provider path is one compatible Agent Sandbox worker plus local Ollama. Arbitrary agent images, production-grade adapter/plugin lifecycle, public-provider credential handling, real Tool/Memory/Observability backends and network-enforced gateway isolation are not proven.
- Backend-neutral runtime semantics beyond the supported kind path, complete cross-claim gateway correlation, filesystem/isolation proof, persistent history and authorized Work listing remain separate work.

## Next Delivery Slice

Expose the installed service's existing evidence boundary through one safe local API access path. Make the CLI submit and query Work through that API, and configure local `npm dev` to read the same endpoint without installing UI as part of Platform apply. Verify allow/deny, CLI/UI evidence parity, negative transport cases and real kind/Ollama execution with automated browser evidence. Then reconcile overlapping tickets by their actual remaining acceptance criteria.
