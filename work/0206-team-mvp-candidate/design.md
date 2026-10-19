# Technical Design: Team MVP candidate

- Ticket: [#206](https://github.com/wunderforge/agenova/issues/206)
- Feature spec: [spec.md](spec.md)

## Composition

Reuse M1's evaluator, M2's OIDC verifier and owner-filtered HTTP boundary, M3's immutable Policy constraints, and M4's worker-to-Gateway credentials. The reference installer mounts a named, pre-existing OIDC configuration Secret into the control plane. The operator acquires company user tokens outside Agenova; CLI and local Portal proxy read protected token files. No browser or Work payload receives a token.

The runbook records Platform/Policy/template revisions, submits identical request specs under two subjects, and reads each user's record from all surfaces. It attempts over-scope, cross-claim, and terminal calls while observing adapter counters. Agent Sandbox and Ollama are the real backend path. A separate controlled provider fixture requires a host-side token; it does not simulate Bedrock or imply AWS IAM enforcement.

## Evidence and Release

The runbook runs checks and renders Portal proof locally. It checks absence of tokens and signing/provider secrets in Work, worker manifest, and generated output. A release checklist records one maintainer and one second-teammate run from clean environments, with versions and cleanup. Generated test artifacts stay outside Git; unavailable checks remain blockers.

## Boundaries

- Platform and OIDC deployment fields stay in the Kubernetes adapter; Policy and claim authority remain backend-neutral.
- Token acquisition, refresh, and company identity management remain outside Agenova.
- AWS EKS/Bedrock and per-claim IAM are not inferred from local fixture results.
- Installation and cleanup target only resources created and identified by the procedure.
