# Quality Gates

Choose the strongest gate that directly proves the task's acceptance criteria. `-All` is the repository baseline, not a substitute for task-specific behavior evidence.

## Repository Baseline

```powershell
.\scripts\check.ps1 -All
```

This checks:

- current required docs and local Markdown links;
- Apache-2.0 metadata, third-party attribution, SPDX source headers, and the public Go module path;
- retired phase/personal-doc paths are absent;
- stable architecture phrases and backend-neutral source boundaries;
- Go formatting and `go test ./...`;
- the reference multi-agent E2E included in the Go test tree.

## Focused Gates

| Change | Required evidence |
| --- | --- |
| Docs or harness only | `./scripts/check.ps1 -Docs` plus rendered/manual review when layout matters |
| Core lifecycle/API | Focused unit or contract test plus `./scripts/check.ps1 -All` |
| Gateway/facts/lineage | Allow and deny tests plus reference E2E and `-All` |
| Runtime adapter | Adapter unit tests, `-All`, and real backend integration output for provider claims |
| CLI/API/UI flow | Executable smoke/E2E output; rendered evidence for UI; `-All` |

## Agent Sandbox Integration

Not part of the default gate because it requires an external cluster:

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

A missing cluster is a blocker, not a passing backend result.

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
