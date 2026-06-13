# Phase 0 PRD: Foundation Alpha

## Problem

Agent workers need isolated execution environments, but starting a fresh sandbox for every run is expensive. Teams also need a clean runtime vocabulary before adding real Kubernetes controllers and gateways.

## Users

- platform engineers evaluating agent runtime governance;
- agent application developers who need a sandbox lease contract;
- maintainers building the next Agenova phases.

## Requirements

FA-001: A pool can hold warm idle sandboxes.

FA-002: A `SandboxClaim` can bind to an idle sandbox for one agent worker run.

FA-003: A claim carries run input/configuration without being modeled as one tool invocation.

FA-004: The design keeps external system credentials out of sandbox configuration. Phase 0 uses static harness evidence only; behavior-level proof belongs to the Tool Gateway phase.

FA-005: Claim terminal state is preserved as `Succeeded`, `Failed`, or `Expired`. Sandbox cleanup evidence is represented as status fields/conditions, not by overwriting the claim phase.

## Non-Goals

- no real Kubernetes controller-runtime integration;
- no generated CRDs;
- no real Tool Gateway or Model Gateway;
- no memory, rollback, Vault, SPIFFE, DAG, or UI;
- no `ToolInvocation` or `ModelInvocation` implementation;
- no entrypoint allowlist behavior in Phase 0 code.
