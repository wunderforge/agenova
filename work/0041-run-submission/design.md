# Design: reference `run -f` composition

## Sequence

```text
ClaimRequest YAML
  -> strict parse and validation
  -> trusted local principal + reference policy admission
  -> reference AgentTemplate resolution
  -> effective-authority resolution
  -> system-managed Pending claim issuance
  -> RunService
  -> selected RuntimeBackend
  -> terminal claim and teardown evidence
```

Denied or invalid requests stop before issuance and before any backend call.

## Boundary decisions

- `internal/cli` owns flags, exit behavior, and backend-neutral output only.
- `internal/app` owns the reference policy/template registry and composes the merged authorization, authority, issuance, and lifecycle services.
- The reference memory runtime is prepared at the application composition edge. The CLI does not import the concrete runtime.
- AgentTemplate `engineer` and runtime template `reference-engineer-runtime` are resolved separately, even though this demo maps one to the other.
- The reference slice uses a deterministic no-op work callback. Real agent execution and the Kubernetes composition are separate integration work.
