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

## Deliver Phase 1-3 With Workers

1. Start from a non-`main` delivery branch.
2. Write a task packet from `docs/harness/claude-worker-playbook.md`.
3. Give each worker one bounded mission and one evidence gate.
4. Review worker output before merging into the delivery branch.
5. Capture evidence with `./scripts/evidence.ps1 -Phase <phase> -Gate <gate> -Command "<command>"`.
6. Run `./scripts/check.ps1 -All` after integration.
7. Keep commits readable; rewrite only unpublished delivery history and preserve backup tags before any cleanup.

## Validate Kubernetes Evidence

1. Prefer kind or minikube for local reproducibility.
2. Record cluster version, installed CRDs, relevant pods, events, and logs.
3. Prove the Agenova API stayed backend-neutral.
4. Store the evidence under `docs/evidence/<phase>/<gate>/`.
5. If upstream Agent Sandbox cannot be installed or verified, record the blocker and keep the adapter boundary intact.
