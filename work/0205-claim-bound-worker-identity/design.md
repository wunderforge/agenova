# Technical Design: Claim-bound worker identity at governed Gateways

- Ticket: [#205](https://github.com/wunderforge/agenova/issues/205)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

`RunService` owns claim phase and backend worker identity. Controlled Agent Sandbox uses a claim-derived control token for start/status/stop; it is not verified Gateway authentication. Tool and Model Gateways consume claim authority but must not treat caller-nominated claim IDs as live identity. In E13, only the trusted control plane has EKS Pod Identity for Bedrock.

## Decision

Use a short-lived, server-signed Agenova workload JWT for the reference live path. Issue only after `Bound` publishes authoritative backend worker identity. Bind fixed issuer, intended Gateway audience, claim ID, worker ID, issued-at and expiry capped at claim deadline. Keep signing keys server-side and distinct from provider credentials. Verification produces an internal identity value. Gateways then confirm current Running phase, bound worker and non-revocation before reading effective authority. A requested target claim ID can only match or fail against this value. Never write token contents to facts or diagnostics.

Deliver identity through the existing controlled Agent Sandbox launch seam only to its bound worker. If its image cannot safely consume the token, record a backend gap and fail closed in live mode; do not reuse its claim-derived control token as authentication.

## Ownership and Contract Boundaries

- `internal/app`: authoritative binding, lifecycle and revocation lookup; no Kubernetes/AWS types.
- Internal workload identity: bounded issue/verify, audience/expiry checks, sanitized errors and deterministic fixtures.
- `internal/modelgateway` and `internal/toolgateway`: shared verified identity before authority lookup or invocation; no bare-ID live fallback.
- `internal/runtime/agentsandbox`: worker-specific delivery and manifest/environment absence checks; backend mechanics stay in the adapter.
- E13's Bedrock adapter receives only Gateway-approved calls. IAM caps the trusted control plane; no per-claim IAM claim is made here.

## Alternatives Considered

- Bare claim ID or existing control token: caller-known correlation/control data, not verified worker identity.
- Give workers AWS roles: couples core claims to EKS/IAM and permits direct provider bypass.
- Shared static Gateway token: cannot distinguish or revoke individual claims.

## Verification Strategy

Test issuance after binding, expiry/audience/issuer validation, current worker and phase, revocation and defensive snapshots. Both Gateway adapter spies prove positive calls and zero calls for each negative. Installed kind uses a real worker to demonstrate allowed, cross-claim and post-terminal outcomes. Scan worker manifests and evidence for signing keys, user tokens and provider credentials. Run focused and full repository gates.

## Risks and Compatibility

A process controlling a worker may use that worker's own temporary authority; E15 separately proves egress/direct-provider isolation. Signature verification alone is insufficient without current claim state. Clock skew or key rotation must not extend claim lifetime. Kind evidence does not establish EKS/Bedrock enforcement.
