# CI/CD application integration evidence

The local-only entries below are historical. Owner subsequently authorized real
delivery: see [19 October AWS/CLI acceptance](aws-acceptance-20261019.md) for the
successful paired rollout, real allowed/denied Work and revoked temporary access.

19 October 2026; local candidate only. No cloud resources, image publication,
shared deployment, GitHub merge, issue closure or outbound comments.

## Provenance and changes

- Base #216: `2ad6daef564f252a909cdc873761952ce426bf00`.
- Tool #192: `127d793853b79b69d5fd61dda88dd570fb2d8a47`, integrated locally in `e82d23c`.
- Diagnosis #217: `296d2d0d253cede029f0d1adf71fa9a471ff1eb9`, integrated locally in `5100b1e`.
- Conflict resolution retains PrepareFor/SetupFor, project registry, current
  membership checks, durable journal commits and adds Tool provider/catalog.
  `appendTool` is assigned **after** durable journal configuration.
- Diagnosis reservation cleanup retains owner/session binding. Comments from the
  old memory-only implementation were corrected: durable commits can fail and
  must latch storage failure, not be described as unreachable.
- Combined regression covers member identity, catalog filtering, allowed calls,
  out-of-scope denial with zero tool calls, owner isolation, and SQL-backed
  service reconstruction/idempotency without replaying providers.
- Carried prior local-only Windows path fixtures, synthetic DB-clock fixture
  fixes and CSS/HTML LF+CSP assertions; no production TLS weakening.
- Native Windows formatter initially exceeded 32K arguments (266 files, >36K
  characters). Check now batches all files, with a regression preserving later
  findings and native errors. Context-argument checks remain intact.
- Full-gate typecheck found the old MCP installed test assumed a non-null
  template. It now explicitly rejects a missing visible template before access,
  retaining its assertions rather than weakening the member schema.

## Executed results

| Check | Result / scope |
| --- | --- |
| Native focused console/CP/client/facts | Passed after Windows fixture fixes; initial sandbox loopback transport failures rerun with normal local-network permission |
| `go test -count=1 ./...` | Passed on native Windows; opt-in SQL/cluster tests without their env are not counted as real backend proof |
| Disposable TLS PostgreSQL six-package campaign | Passed: authentication/postgres, websession, console, workstore, connectedclient, CP. Combined Tool durable subtests executed. verify-full TLS; fresh test container removed |
| Independent MCP fixture module | `go -C harness/integration/mcpfixture test -count=1 ./...` passed; official SDK/local test data, not EKS |
| Native and Linux binaries | Windows CLI/CP and linux/amd64 CP/demo-worker/workerctl built; local output `.tmp/cicd-integration/bin` |
| Synthetic Google login/browser | Passed label, actual CSS/CSP, invitation fragment removal, Origin and mobile layout; real Google not called |
| Synthetic access approval/browser | Passed pending/role approval/mobile; no real membership modified |
| Complete repository gate | Final `scripts/check.ps1 -All` exited 0: docs/contracts, formatter regression, module tidy, vet, all Go tests, tagged compilation/probes, 128 UI unit tests, production build and 55 Playwright smoke tests passed. Earlier formatter/typecheck failures remain recorded above |
| Actual Dockerfile image builds | Both actual Dockerfiles built linux/amd64 successfully from allowlisted contexts; no-network/read-only/cap-drop fail-closed executable smokes passed. No image push or live-service claim |

Local images: `agenova-cicd-local:control-plane-5bad29605884` and
`agenova-cicd-local:worker-50aeacbeb581`. Local image identities are respectively
`sha256:39ed50a52909d2c8b78394bdb617e97d618ba4d6a38729c6eb568415739ec0d7`
and `sha256:b4b3a14870d7373b83a2dbb15e21a7e09693cf2185a071dcd816a11990db1cc6`.
They are local image identities, not published ECR digests. Build source hashes
are in `.tmp/cicd-integration/images/summary.json`. The synthetic database summary
is `.tmp/cicd-integration/local-postgres-summary.json`.

Native Windows race tests were not run (no C compiler located); the separately
owned Linux CI Main profile remains the race-test gate. Cluster integration and
live-model checkpoint packages were compiled, not executed. Canonical browser
tests use isolated fixtures; the actual MCP kind installed suite was not rerun.

The first combined test omitted explicit member setup/configure callbacks and
correctly failed construction. It was fixed; a subsequent fresh SQL campaign
passed. Synthetic credentials were generated only for the disposable database;
no existing private file was read. The temporary Docker container was removed
even on the earlier failing run.

## Handoff and unmet gates

See [application deployment contract](../../deploy/production/cicd-candidate.md).
Tool credential finding remains open; no complete OTEL implementation, business
task-condition policy or actual AWS multi-component acceptance is certified.
The independent-login lab infrastructure is not included. Original worktrees and
the separately owned CI/CD branch were not modified.

## Latest #216 refresh, 19 October

Owner requested bringing the latest reviewed member implementation into the test
candidate. Merged source `88888a28994dd16f54cd3d1faccf9e9885881ccc` locally over
`5da048d`; Git resolved the merge without conflicts. This includes operational
three-role administration, the project governance editor, session-capacity repair,
Portal toolbar cleanup and upstream production-directory reorganization. Existing
Tool/member/durable composition and Windows fixture fixes remain in place.
Task-type conditional authorization remains explicitly excluded.

GitHub still reported #216 OPEN and main `ae24a67` when checked. This is local
integration of the reviewed source, not a claim that #216 was merged upstream.
The historical #223 [release preflight](release-preflight.md) remains applicable;
no workflow was enabled, no image pushed, no deployment or AWS state move made.

Re-executed on the refreshed source:

- Fresh verify-full TLS PostgreSQL campaign passed all six packages: identity
  postgres, websession, console, workstore, connectedclient and control-plane.
  New role/session tests and the combined member/Tool/durable tests ran against
  this disposable database. Its container was removed; no live Google/provider
  or cloud database was used.
- Administration, access-request and governance-editor browser harnesses passed,
  including protected Owner/self behavior, explicit role changes, denied writes,
  policy/template save requests, malformed JSON, immutable conflicts, project
  target reset and desktop/mobile layout. These use synthetic API fixtures;
  desktop governance screenshot was inspected, not treated as live API proof.
- Independent official-SDK MCP fixture module passed again.
- Actual linux/amd64 Control Plane and Worker Dockerfiles built; both no-network,
  read-only, capability-dropped fail-closed executable smokes passed. New local
  tags are `agenova-cicd-local:control-plane-8d78fda42375` and
  `agenova-cicd-local:worker-50aeacbeb581`. Current local image identities:
  `sha256:f481f63892e7ac4c8be5f3c0a99e71107566f4bcce9e01a053d4bd5fb2a07306`
  and `sha256:a9d887b2f5f810054734b07207d9a324e56dd5bab6610b488d7a1572d3f15b74`.
  These are local images, not ECR publication digests.

- Complete `scripts/check.ps1 -All` exited 0 on the refreshed source: docs,
  contracts, formatter, module tidy, vet, all Go tests, opt-in package compilation,
  probe tests, 128 UI unit tests, production UI build and 56 Playwright tests.
  Existing Windows configuration fixtures passed after retaining the local fixes.
- CI file is byte-identical to the observed main version, as required by the
  latest #216 decision. This refresh does not install the separate #223 workflow.

Previous full-gate/image results above describe the earlier candidate. Native
race and live release restrictions, the open anonymous-login-capacity issue, and
the Tool credential finding are not resolved by this refresh.
