# Task: Gated delivery to Neo's existing EKS cluster

- Ticket: [#222](https://github.com/wunderforge/agenova/issues/222)
- Mission: Deliver a tested release workflow without executing a real release in this task.
- Target: `.github/workflows/shared-eks-release.yml`, `scripts/release/`, release runbook.
- User value: A prepared main commit can be published and deployed through traceable, bounded automation.
- PRD outcome: [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path).

## Context to Read

- `AGENTS.md`, [PRD](../../docs/product/prd.md), this packet.
- [Architecture contract](../../docs/product/architecture-contract.md#reference-installation-and-bootstrap).
- [Existing CI](../../.github/workflows/ci.yml).
- [Control-plane Dockerfile](../../deploy/reference/Dockerfile), [Worker Dockerfile](../../harness/integration/agentsandbox/testworker/Dockerfile).
- [Release runbook](../../docs/development/shared-eks-release.md).

## Scope

In scope: offline tests, two-image selective publication, named image-only control-plane deployment, evidence, opt-in activation, existing OIDC roles.

Out of scope: real release in this task, cluster lifecycle, authentication implementation, secrets, Policy changes, general Platform reconciliation, Worker/template migration, production HA or zero-downtime guarantees.

## Acceptance Criteria

- PR and default workflow runs cannot acquire AWS credentials or mutate cloud resources.
- Explicit enablement and main ref are required for publication; deployment has a separate switch.
- Compare against the deployed source, not the previous push. Unknown baseline fails closed.
- Immutable full-SHA tags and digest references; reruns reuse existing tags.
- Guard the selected Deployment UID, Pod configuration, image and resource version. Preserve other containers and all configuration.
- Worker changes publish candidates but block control-plane rollout until the owner establishes a compatible pair.
- Record candidate/release digests and recovery result without credentials or complete Pod objects.

## Negative Case

Offline tests reject malformed/foreign refs, missing baselines, drift, incompatible Worker changes and failed rollout; PR validation never obtains OIDC credentials.

## Execution Todo

- [x] Scout current main and installed member-upgrade boundary.
- [x] Self-review against user scope: no release, merge or cluster changes.
- [x] Implement workflow, commands and runbook.
- [x] Add focused offline behavioral tests and workflow lint.
- [x] Run focused gates and `./scripts/check.ps1 -All`.
- [x] Review diff; submit PR with explicit live-verification deferral.

## Quality Gates

- `node --test scripts/release/release.test.mjs`
- `actionlint .github/workflows/shared-eks-release.yml .github/workflows/ci.yml`
- `./scripts/check.ps1 -All`

## Evidence Required

Exact local test/lint/gate results. No ECR publish or EKS deployment is claimed; the owner will test a separate release-ready commit.

Verified locally on 19 October 2026:

- Node release tests: 28 passed; all cloud/cluster subprocesses faked.
- actionlint v1.7.7: both release and existing CI workflows passed (ShellCheck not available in the Windows check).
- Actual local `plan(HEAD, HEAD)` using Git and Go dependency discovery: neither image selected, no cloud commands.
- `./scripts/check.ps1 -All`: passed, including Go tests, frontend contracts, 124 frontend unit tests and 52 Playwright smoke tests.
- `git diff --cached --check`: passed.
- Existing npm lockfile audit reports a high-severity development-only `source-map-js` advisory; production-dependency audit reports zero. No dependency versions changed in this delivery task.
- Real OIDC exchange, Docker image builds/pushes, EKS rollout and Google/Work smoke are deliberately not executed.

## Constraints

- Use a separate worktree; do not alter another conversation's release commit.
- No long-lived AWS keys. Publisher main-ref trust and deployer environment trust remain separate.
- Do not modify AWS/GitHub permissions established in the prior task.

## Decisions and Blockers

- The owner explicitly requested shared-demo image delivery, not production upgrade capability. Application contracts remain unchanged.
- Main does not yet contain the public member-login implementation; release enablement must wait for a compatible source commit.
- Worker registration is an immutable slot. Do not overwrite/delete it to simulate automatic version migration.
- Control-plane image delivery does not rewrite Platform configuration/revision; this limitation and future declarative reconciliation are explicit in the runbook.

## Owner-authorized first candidate delivery

The Owner subsequently authorized an exact candidate-branch CI/CD trial and
post-verification merge. This supersedes the no-live-release/main-only limits for
this one manual, SHA-pinned path; ordinary main releases retain their original
guards. Integration and real acceptance are tracked in
[the candidate packet](../0218-demo-cicd-integration/task.md).
No database migration, immutable template overwrite, external service deployment
or broad IAM permission expansion is included. Preserve the original main trust,
temporarily add only the exact candidate publisher subject/environment branch,
and remove that access after testing. The paired CP/Worker mode changes only
the CP image and its Worker digest value; all other configuration stays bound.
