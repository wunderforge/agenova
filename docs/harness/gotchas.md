# Harness Gotchas

- Keep scenario names tied to behavior, not implementation internals.
- Do not write fixture checks that only prove the fixture contains its own expected string.
- Static checks are acceptable in Phase 0 only when clearly labeled as placeholders.
- Avoid adding future architecture surfaces without behavior evidence.
- Do not let `SandboxClaim` wording drift into per-tool-call semantics.
- Do not treat "secret absent from sandbox config" or ordinary Pod isolation as a complete security proof.
- Do not expose upstream Agent Sandbox CRD shape through application-facing Agenova APIs.
- Do not mark Agent Sandbox capabilities as verified without upstream docs, install output, or behavior evidence.
