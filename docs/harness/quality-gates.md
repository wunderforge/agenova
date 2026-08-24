# Quality Gates

Choose the strongest gate that directly proves the task's acceptance criteria. `-All` is the repository baseline, not a substitute for task-specific behavior evidence.

## Shared Check Profiles

Hooks and GitHub Actions route into the same `scripts/check.ps1` entry point.
Concrete checks live once under `scripts/checks/`; callers select a profile
instead of copying commands.

Pull requests also run `scripts/check-pr-body.ps1` against the GitHub event body.
This deterministic contract check requires a closing ticket link, completed
sections, exact verification results, backend-neutrality confirmation, and
explicit risks and blockers.

| Profile | Caller | Scope |
| --- | --- | --- |
| `Fast` | Optional pre-commit hook | Staged whitespace/conflict markers, Go formatting, and SPDX headers |
| `PR` | Pull requests to `main` | Full repository baseline plus the race detector |
| `Main` | Pushes to `main` | The same baseline against the committed `main` SHA; artifact builds can extend this profile later |
| `Backend` | Manual or scheduled real-environment lane | Full baseline plus the Agent Sandbox integration test |

`-ChangedOnly` is valid only with `-Profile Fast`. PR and Main always validate
the complete candidate repository. Existing focused switches remain supported
for local use.

## Repository Baseline

```powershell
.\scripts\check.ps1 -All
```

This checks:

- AIDLC source ownership, task-packet/template structure, current required docs, and local Markdown links;
- Apache-2.0 metadata, third-party attribution, SPDX source headers, and the public Go module path;
- retired phase/personal-doc paths are absent;
- stable architecture authority and backend-neutral source boundaries without requiring the same prose in several documents;
- Go formatting, module consistency, `go vet`, and `go test ./...`;
- the reference multi-agent E2E included in the Go test tree.

## Pull Request Automation

Every pull request to `main` runs the standard `CI / baseline` check defined in
`.github/workflows/ci.yml`:

```powershell
./scripts/check.ps1 -Profile PR
```

Pushes to `main` use `-Profile Main`. Both profiles currently run the full
repository baseline and race detector; the profile names keep trigger-specific
delivery behavior explicit without duplicating validation logic. Local Windows
environments with `CGO_ENABLED=0` should use `-All`; contributors with a
working CGO toolchain may also run `-Race`.

The workflow has read-only repository permissions, does not receive project
secrets, cancels superseded runs for the same PR, and also verifies pushes to
`main`.

Configure the `main` branch ruleset to require `CI / baseline` before merging.
The workflow alone runs the check but cannot make it mandatory.

## Focused Gates

| Change | Required evidence |
| --- | --- |
| Docs or harness only | `./scripts/check.ps1 -Docs` plus rendered/manual review when layout matters |
| Core lifecycle/API | Focused unit or contract test plus `./scripts/check.ps1 -All` |
| Gateway/facts/lineage | Allow and deny tests plus reference E2E and `-All` |
| Runtime adapter | Adapter unit tests, `-All`, and real backend integration output for provider claims |
| CLI/API/UI flow | Executable smoke/E2E output; rendered evidence for UI; `-All` |

## Agent Sandbox Integration

Not part of the PR or Main profile because it requires an external cluster:

```powershell
.\scripts\check.ps1 -Profile Backend -KubeContext kind-agenova-k8s-lab
```

A missing cluster is a blocker, not a passing backend result.

The real-backend gate remains manual until cluster creation and Agent Sandbox
installation are deterministic in CI. Adapter PRs must include its output or
an explicit environment blocker; a passing `pr / baseline` proves only that
the integration package compiles.

## Evidence Capture

```powershell
.\scripts\evidence.ps1 -Ticket AGN-123 -Gate reference-e2e -Command "go test -v ./harness/e2e/"
```

Keep one current accepted result per task/gate, or one explicit blocker. See `docs/evidence/README.md`.

## Gate Response

- **Pass:** report the exact command and result.
- **Fail:** stop scope expansion, fix the smallest responsible issue, and rerun.
- **Blocked:** record the missing environment, dependency, credential, or decision and do not claim completion.
- **Degraded:** state the unproven behavior and create a follow-up gate.
