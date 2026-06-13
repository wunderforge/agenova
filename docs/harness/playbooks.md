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
