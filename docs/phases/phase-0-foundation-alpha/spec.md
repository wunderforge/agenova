# Phase 0 Spec

## API Sketch

Phase 0 defines local Go types that mirror the future Kubernetes vocabulary.

### AgentSandboxTemplate

Defines an agent worker image and command shape for local validation.

Important fields:

- `Image`
- `Command`

### SandboxWarmPool

Defines the desired warm sandbox pool for a template.

Important fields:

- `TemplateRef`
- `Replicas`

### SandboxClaim

Represents one agent worker run / sandbox execution lease.

Important fields:

- `PoolRef`
- `Input`

`Input` is phase-local task configuration. It is not a general memory, prompt, or tool-call model.

### SandboxClaimStatus

Tracks claim lifecycle and evidence.

Phases:

- `Pending`
- `Bound`
- `Running`
- `Succeeded`
- `Failed`
- `Expired`

Cleanup evidence:

- `SandboxReplaced=true` records cleanup/replacement evidence after a terminal claim state.

## Lifecycle

Successful run:

```text
Pending -> Bound -> Running -> Succeeded
```

Failed run:

```text
Pending -> Bound -> Running -> Failed
Pending -> Bound -> Failed
```

`Bound -> Failed` covers a sandbox that is lost or unusable after bind and before start.

Expired claim:

```text
Pending -> Expired
```

A claim terminal phase is not replaced by sandbox cleanup state.

## Runtime Behavior

The local runtime can:

- create templates and pools;
- maintain warm idle sandbox state;
- bind a pending claim to an idle sandbox;
- mark claims running, succeeded, failed, or expired;
- replace consumed sandboxes to maintain pool size.

The runtime is an in-memory Phase 0 model. It is not concurrency-safe. Add locking only when a later behavior harness introduces concurrent access.
