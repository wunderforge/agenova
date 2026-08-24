# Harness Playbooks

## Start a GitHub Ticket

Use when a contributor is ready to turn an accepted GitHub Ticket into coding-agent execution context.

1. Read the Ticket, `AGENTS.md`, and `docs/product/prd.md`.
2. Confirm the Ticket has one bounded outcome, acceptance criteria, a negative case, evidence requirements, Owner, Reviewer, and dependencies.
3. Choose Task, Task + Spec, or Task + Spec + Design using `docs/development/AIDLC.md`.
4. Run `./scripts/new-task.ps1` with the Issue number, slug, title, and required planning-depth switches.
5. Replace every `TODO` marker, linking only task-relevant context.
6. Check the packet against the Ticket and PRD, then stop until Owner/Reviewer approval is recorded in the Ticket.
7. Begin implementation only after the task packet is approved.

Do not ask the Agent to implement directly from a conversational prompt or create a packet that silently changes the Ticket.

## Change a Core Contract

Use when changing claim lifecycle, authority, facts, lineage, or shared API types.

1. Read the Issue, task packet, PRD, architecture contract, and applicable feature spec.
2. If no accepted spec exists, add one to the task packet and elaborate behavior and compatibility before dependent work proceeds.
3. Define negative cases in the Issue/spec and update the reference implementation and contract tests together.
4. Confirm backend-specific types remain isolated.
5. Run focused tests and `./scripts/check.ps1 -All`.
6. Update only the sources of truth whose owned behavior changed.

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

## Elaborate Parallel Work

Use when an adapter, UI, example, or test contributor needs an upstream contract that is not yet stable.

1. Keep the consumer ticket blocked or fixture-only until the producing contract is accepted.
2. Put shared behavior in a feature spec when the Issue cannot express it unambiguously.
3. Land a backend-neutral schema and representative fixtures before consumers claim integration readiness.
4. Let consumers prototype against fixtures without changing the producer's contract from their subtree.
5. Replace fixtures with integration evidence when the real producer is available.

Do not use parallel delivery pressure to freeze an unreviewed contract.
