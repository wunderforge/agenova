# Phase 0 Acceptance

## Acceptance Matrix

| ID | Requirement | Evidence |
| --- | --- | --- |
| FA-001 | Warm pool creates idle sandboxes | `go test ./...` and `smoke-warmpool-claim` |
| FA-002 | Claim binds to an idle sandbox | `go test ./...` and `smoke-warmpool-claim` |
| FA-003 | Claim input/config is not modeled as one tool invocation | docs, API shape, and harness naming checks |
| FA-004 | External credentials stay out of sandbox config | static harness placeholder in `smoke-tool-gateway-secret-boundary` |
| FA-005 | Terminal claim state is preserved after sandbox replacement | `go test ./...` |

## Current Evidence Boundary

FA-004 is intentionally static in Phase 0. The current evidence checks that scenario fixtures do not put external secrets into sandbox configuration. This is not a real security proof. Behavior-level evidence belongs to the Tool Gateway implementation phase.

FA-004 does not prove node-level isolation, workload identity hardening, network isolation, or resistance to container escape. It only checks that the static fixture does not place upstream credentials in sandbox configuration.

## Required Commands

```powershell
go test ./...
.\scripts\check.ps1 -All
```
