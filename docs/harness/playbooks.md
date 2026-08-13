# Harness Playbooks

## Change a Core Contract

Use when changing claim lifecycle, authority, facts, lineage, or shared API types.

1. Read the PRD and architecture contract.
2. Define the compatibility impact and negative cases in the task.
3. Update the reference implementation and contract/unit tests together.
4. Confirm backend-specific types remain isolated.
5. Run focused tests and `./scripts/check.ps1 -All`.
6. Update project design/status only when the public contract or implementation truth changed.

Do not reshape the contract to match one backend's API.

## Add or Change a Runtime Backend

1. Identify which shared semantics the backend supports, maps, or cannot satisfy.
2. Keep provider code inside its adapter package.
3. Run reusable contract cases where possible and focused adapter unit tests.
4. Run real integration evidence for claims about provider behavior.
5. Update the backend note and project status with verified gaps.
6. Run `./scripts/check.ps1 -All`; run `-Integration` when the environment is available.

Do not mark static manifests or mocked responses as real backend verification.

## Add a User-Facing Demo Slice

1. Start from the PRD acceptance scenario.
2. Choose one end-to-end outcome, such as run, deny, or query evidence.
3. Keep CLI/UI code behind the shared Agenova contract.
4. Add an executable smoke/E2E path with stable fixtures.
5. Capture output or rendered evidence a teammate can reproduce.
6. Update README/quickstart and `docs/project-status.md`.

Do not build a broad UI or framework before the evidence schema and golden path are stable.

## Repair the Harness

1. Classify the failure: implementation, acceptance, missing check, environment, or harness design.
2. Fix the smallest responsible issue and rerun the gate.
3. Record the first reusable occurrence in `learnings.md`.
4. On repetition, add a gotcha or mechanical check.
5. Remove superseded guidance so the harness stays small.
