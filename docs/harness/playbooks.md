# Harness Playbooks

## Add a Phase 0 Behavior

1. Update the smallest relevant spec section.
2. Add or update a focused Go test.
3. Add harness fixture evidence only when it improves reviewability.
4. Run `go test ./...`.
5. Run `./scripts/check.ps1 -All`.

## Add a New Phase

1. Create phase docs: `README.md`, `prd.md`, `spec.md`, `acceptance.md`, and `progress.md`.
2. Create a phase harness directory with one smoke scenario.
3. Update `docs/product/roadmap.md`.
4. Update `scripts/check.ps1` only after the new phase has executable or static evidence.

## Run the Phase 1 Adapter Spike

1. Read `docs/product/agent-sandbox-pivot.md`.
2. Read `docs/phases/phase-0-foundation-alpha/spec.md` to preserve baseline semantics.
3. Update `docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md` only with evidence-backed status.
4. Keep upstream Agent Sandbox details behind the adapter boundary.
5. Run `go test ./...`.
6. Run `./scripts/check.ps1 -All`.
