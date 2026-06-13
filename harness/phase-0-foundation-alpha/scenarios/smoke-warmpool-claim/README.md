# Smoke: Warm Pool Claim

This scenario describes the minimum Phase 0 lifecycle:

1. a template defines an agent worker image and command;
2. a warm pool keeps idle sandboxes available;
3. a `SandboxClaim` requests one worker-run lease from that pool;
4. claim status moves through the runtime lifecycle in Go tests.

The claim is not a per-tool-call object. Tool calls inside the worker run are future `ToolInvocation` facts.
