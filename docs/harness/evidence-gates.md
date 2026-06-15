# Evidence Gates

Evidence gates are mandatory acceptance checks. A task is not complete until its gate passes or the failure is recorded as a blocker.

## Common Gate

Run after every integrated change:

```powershell
go test ./...
.\scripts\check.ps1 -All
```

For evidence capture:

```powershell
.\scripts\evidence.ps1 -Phase phase-1 -Gate runtime-contract -Command "go test ./..."
```

For phase completion, run the common gate plus the phase gate:

```powershell
.\scripts\check.ps1 -All
.\scripts\check.ps1 -Phase2Evidence
.\scripts\check.ps1 -Phase3Evidence
```

Do not run a later phase gate until that phase claims completion. The gate is expected to fail while the phase is still under construction.

## Gate Levels

### Static

Use for docs, harness, architecture boundaries, and generated diagrams.

Evidence examples:

- `.\scripts\check.ps1 -All`
- JavaScript syntax checks for `docs/arch-viewer`
- Link or file existence checks

### Contract

Use for backend-neutral behavior.

Evidence examples:

- Contract tests run against the in-memory backend.
- The same contract tests run against an adapter backend when available.
- API tests prove no upstream CRD shape leaks into application-facing types.

### Deploy

Use for Kubernetes-backed behavior.

Evidence examples:

- Cluster version and node list.
- Installed CRDs or upstream install failure logs.
- Resource state, pod state, events, and worker logs.
- E2E output for claim lifecycle behavior.
- Backend-neutral API test output proving upstream CRD shape is not application-facing.

Phase 2 completion requires passing evidence under these gates:

- `cluster-bootstrap`
- `upstream-agent-sandbox-install-or-blocker`
- `claim-lifecycle-e2e`
- `kubectl-runtime-state`
- `backend-neutral-api`

### Governance

Use for Phase 3 user value.

Evidence examples:

- Tool and model gateway integration tests.
- Authorization allow/deny tests.
- Parent/child claim lineage tests.
- Facts query output for `RuntimeEvent`, `ToolInvocation`, and `ModelInvocation`.

Phase 3 completion requires passing evidence under these gates:

- `tool-gateway`
- `model-gateway`
- `authorization-negative`
- `claim-lineage`
- `facts-query`
- `multi-agent-reference`
- `multi-agent-kubernetes-or-blocker`

## Evidence Directory

Store durable evidence under:

```text
docs/evidence/<phase>/<gate>/
```

Each gate directory should include:

- `summary.md`: what was tested, result, date, branch, commit.
- Raw command output where useful.
- Screenshots or rendered artifacts where visual behavior matters.
- Known blockers and follow-up tasks.

If a gate records a blocker instead of a pass, `summary.md` must include:

- The command attempted.
- Full failure output in `output.txt`.
- Why the blocker is outside the current worker scope.
- Alternatives considered.
- Confirmation that the RuntimeBackend boundary remains intact.

## Acceptance Rules

- Passing unit tests without deploy evidence does not complete Phase 2.
- Passing deploy tests without governance negative tests does not complete Phase 3.
- A worker report is not evidence unless Codex can reproduce or inspect the underlying output.
- Prefer a small real test over a broad static assertion.
- Phase 2 is not complete until `.\scripts\check.ps1 -Phase2Evidence` passes.
- Phase 3 is not complete until `.\scripts\check.ps1 -Phase3Evidence` passes.
