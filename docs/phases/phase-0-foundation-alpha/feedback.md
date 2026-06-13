# Phase 0 Feedback

## Review Notes

- `SandboxClaim` must remain a worker-run lease, not a per-tool-call object.
- Tool and model access belong behind gateway boundaries in later phases.
- Claim terminal phases must preserve business outcome: `Succeeded`, `Failed`, or `Expired`.
- Sandbox cleanup/replacement is resource evidence, not a claim phase.
- Configuration fields should stay tied to implemented behavior.
- Human design rationale is tracked, but excluded from default agent context.

## Watch Items

- Do not introduce `ToolInvocation` or `ModelInvocation` until gateway phases need behavior evidence.
- Do not add Kubernetes controller-runtime before Phase 1 scope is explicit.
- Keep harness scenarios small and tied to observable behavior.
