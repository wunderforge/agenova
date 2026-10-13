# Feature Specification: Claim-bound worker identity at governed Gateways

- Ticket: [#205](https://github.com/wunderforge/agenova/issues/205)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority).

## Intent

A live worker proves it represents one system-issued, Running claim before a governed Tool or Model call uses that claim's effective authority. Verified identity, not a request field, selects the claim. Invocation facts never contain credential bytes.

## In Scope

- Short-lived Agenova workload identity issued after authoritative claim/backend-worker binding.
- Verification at both live Gateway entry points before authority lookup or external invocation.
- Denial after expiry, revocation or terminal claim state; fail closed when current state is unavailable.
- Agent Sandbox delivery of only claim-scoped Agenova identity material.

## Out of Scope

- AWS IAM, EKS Pod Identity, Bedrock/S3 permissions, provider credentials in workers, enterprise federation and parent/child delegation.

## Requirements

- Given a bound Running claim and its intended worker, a valid unexpired credential for the intended Gateway permits only authority-checked calls under that claim and records correlated facts.
- Credential binds issuer, audience, claim ID, worker ID and expiry no later than claim deadline. Its trust root remains server-side.
- Caller-supplied target claim ID can only match or fail against verified identity; it cannot select another authority snapshot.
- Terminal transition or revocation stops later calls even before encoded credential expiry.

## Negative Cases

- Missing, malformed, expired, wrong-issuer, wrong-audience, wrong-worker, revoked, terminal or unverifiable credentials deny with zero adapter calls.
- Valid claim-A credential with claim-B target denies without reading B authority or adding B-attributed facts.
- Live mode cannot accept a bare claim ID or fixture token when verification is absent.

## Compatibility

- Preserve backend-neutral request, claim, authority and facts. Explicitly labelled in-process fixtures may remain for tests but not as installed-live fallback.
- E13's trusted control plane may continue holding AWS workload identity. The worker credential authenticates to Agenova, never directly to AWS.

## Open Decisions

- None requiring product scope change; exact local signing and delivery mechanics remain implementation decisions within #205.
