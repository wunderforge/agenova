# Phase 1 Acceptance

Phase 1 is accepted when the repository has evidence for the backend decision, not when a large integration exists.

## Required Evidence

- A minimal `RuntimeBackend` interface exists and is exercised by contract tests.
- The in-memory runtime passes those contract tests.
- Contract tests are backend-neutral: every new backend must pass them without application-facing API changes.
- The Agent Sandbox adapter spike records what was verified against upstream resources or local installation steps.
- The backend capability matrix is updated with verified status.
- `go test ./...` passes.
- `.\scripts\check.ps1 -All` passes.

## Decision Output

At the end of the spike, choose one:

- proceed with Agent Sandbox as the default backend substrate;
- proceed with Agent Sandbox plus Agenova-side adapter orchestration gaps;
- defer Agent Sandbox and design a different backend adapter;
- defer Agent Sandbox and onboard a different backend through the same `RuntimeBackend` contract;
- build a native Kubernetes controller only for semantics Agent Sandbox cannot carry.
