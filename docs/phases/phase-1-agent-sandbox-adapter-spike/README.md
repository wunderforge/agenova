# Phase 1: Agent Sandbox Adapter Spike

Status: scaffolded.

Purpose:

- preserve the Phase 0 runtime contract;
- define the smallest useful `RuntimeBackend` boundary;
- keep the in-memory runtime as the reference backend;
- evaluate Kubernetes SIG Apps Agent Sandbox as the first runtime backend adapter;
- decide with evidence whether Agent Sandbox should be the default Phase 1 substrate.

Strategic rule:

- `RuntimeBackend` is a pluggable boundary. Agent Sandbox is the first backend to evaluate, but Agenova must be able to use another backend adapter without changing application-facing APIs.

Out of scope:

- Tool Gateway implementation;
- Model Gateway implementation;
- memory, checkpoints, rollback, or UI;
- cloud control plane work;
- self-built Kubernetes sandbox lifecycle controllers;
- direct application dependency on upstream Agent Sandbox CRD shape.

Read with:

- `docs/product/architecture-contract.md`
- `docs/product/agent-sandbox-pivot.md`
- `docs/product/runtime-backend-mvp-contract.md`
- `docs/phases/phase-0-foundation-alpha/spec.md`
- `harness/phase-1-agent-sandbox-adapter-spike/README.md`
