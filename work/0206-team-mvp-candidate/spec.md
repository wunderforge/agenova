# Feature Specification: Team MVP candidate

- Ticket: [#206](https://github.com/wunderforge/agenova/issues/206)

## Flow

An operator installs one kind Platform, registers versioned Policy and AgentTemplate, and configures the existing company OIDC issuer through a Kubernetes Secret reference. Two externally authenticated users submit byte-equivalent `ClaimRequest.spec` through connected clients. The service verifies each user, evaluates Policy, issues immutable authority, runs an Agent Sandbox worker through claim-bound Gateways, and exposes one evidence view.

## Requirements

- Installation is repeatable without publishing OIDC tokens or provider credentials.
- Allowed Work completes real Ollama inference; terminal evidence retains its Policy and authority snapshot.
- Users read only their own evidence; over-scope, cross-claim, and terminal worker calls do not invoke external adapters.
- A host-held token is required for the controlled provider fixture; the worker receives only bounded Agenova identity and denied attempts never reach the fixture.
- CLI, API, and Portal display the same projection of one service record.
- A second teammate can reproduce from documented clean-environment steps without code edits.

## Failure and Compatibility

- Missing or invalid OIDC, Policy, template, runtime, Gateway identity, and provider configuration fail at their responsible boundaries. No silent fallback to local identity, memory runtime, broader authority, or synthetic success.
- Principal-free `ClaimRequest`, backend-neutral claim/evidence types, and explicit reference mode retain their meanings.
- The release candidate is local/kind; AWS EKS/Bedrock remains separate E13 evidence.
