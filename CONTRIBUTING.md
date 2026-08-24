# Contributing to Agenova

Contributions should strengthen the shared MVP path without creating a second source of truth or changing the product boundary by accident.

## 1. Choose a Bounded Task

Create work with the GitHub MVP delivery ticket form. The Issue is the delivery contract and must contain:

- one mission and parent Epic;
- explicit in-scope and out-of-scope boundaries;
- observable acceptance criteria;
- exact quality gates and evidence;
- a link to the MVP flow it improves.

Split work by one independently reviewable behavior and evidence gate, not by estimated coding hours. AI may compress implementation time, but it does not remove contract, integration, or review boundaries.

During elaboration, use [`specs/README.md`](specs/README.md) to choose the smallest useful planning depth. A bounded change uses the Issue alone. Add a feature spec or design only when a public contract, cross-component behavior, security/authority rule, or parallel dependency needs durable agreement.

## 2. Read Only What You Need

Start with the active Issue and [`AGENTS.md`](AGENTS.md). The [development workflow](docs/development/workflow.md) routes each question to its authority; load only the applicable product, architecture, spec, backend, or harness material.

Product requirements belong in the PRD, architecture invariants in the architecture contract, implementation evidence in project status, and mutable delivery state in GitHub. Update an authority only when the accepted change alters what that authority owns.

## 3. Implement and Verify

Work on a ticket-specific branch or worktree. Stabilize a shared contract, schema, fixture, or accepted spec before dependent adapter, UI, or example work claims integration readiness.

Add or update the strongest relevant behavior check with the implementation.

Enable the optional fast pre-commit hook once per clone:

```powershell
.\scripts\install-hooks.ps1
```

The hook checks staged whitespace, conflict markers, Go formatting, and SPDX
headers. It is a fast developer feedback loop and can be bypassed, so GitHub CI
remains authoritative.

```powershell
.\scripts\check.ps1 -All
```

If the task touches Agent Sandbox behavior, run the integration gate when the required cluster is available. Do not replace real backend evidence with static YAML or a prose claim.

## 4. Open a Reviewable PR

Start from `.github/pull_request_template.md`. CI rejects a PR body that leaves
required content empty, omits a closing ticket link, lacks an exact verification
result, or does not confirm backend neutrality.

The PR must pass the required `CI / baseline` GitHub check. GitHub invokes the
same repository entry point with the `PR` profile; it does not replace
task-specific evidence or real-backend evidence.

Include:

- what user-visible or contract behavior changed;
- files and boundaries touched;
- exact commands run and results;
- evidence location;
- remaining risks or blockers;
- confirmation that the change did not leak backend-specific types into shared APIs.

The Issue and Delivery Project remain authoritative for owner, priority, dependency, sequence, status, and completion evidence. Do not mirror that state into repository planning files.

## Contribution License and DCO

All contributions are submitted under the repository's
[Apache License 2.0](LICENSE).

Every commit must include a `Signed-off-by` line certifying the
[Developer Certificate of Origin 1.1](https://developercertificate.org/). Use:

```text
git commit -s
```

The sign-off certifies that you created the contribution, or otherwise have
the right to submit it under the project's license. It also records your name
and email permanently in the public Git history. Do not submit confidential,
employer-owned, or incompatibly licensed material.

## Definition of Done

A task is done when:

- acceptance criteria are observable and pass;
- focused and repository gates pass;
- docs match implemented behavior;
- a teammate can reproduce the evidence;
- no unrelated abstraction or refactor was added;
- blockers are recorded instead of being described as completed work.

## Review Priorities

Review in this order:

1. Product and scope boundary.
2. Behavior and negative cases.
3. Evidence quality and reproducibility.
4. Backend neutrality.
5. Code clarity and maintainability.
