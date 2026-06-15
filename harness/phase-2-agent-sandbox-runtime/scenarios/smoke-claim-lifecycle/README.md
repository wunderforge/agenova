# Scenario: Smoke Claim Lifecycle

Verifies the minimal create→bind→start→succeed lifecycle through the
`SpikeAdapter` against the upstream Agent Sandbox controller.

## Given

- A SandboxTemplate with image `busybox:stable` and command `["sh", "-c", "sleep 10"]`.
- A SandboxWarmPool with 1 replica referencing the template.
- A SandboxClaim referencing the pool.

## Expected

1. `AddClaim` creates an upstream `SandboxClaim` CRD; local state = `Pending`.
2. `BindClaim` polls until the controller assigns a sandbox; local state = `Bound`.
3. `StartClaim` polls until the sandbox pod is ready; local state = `Running`.
4. `SucceedClaim` deletes the upstream claim; local state = `Succeeded`, `SandboxReplaced=true`.
5. Pool replenishes to 1 idle sandbox (via OnReplenish strategy).

## Semantic Gaps Observed

- Steps 2 and 3 are controller-driven; `BindClaim`/`StartClaim` poll rather than trigger.
- Step 4 is adapter-local; the upstream CRD is deleted, not transitioned to `Succeeded`.
- Pool status breakdown is approximated from local claim state.
