# Local acceptance — independent images and automatic delivery

Date: 20 October 2026. Base: `f659605e1cd24ddf826f2a5caef0c433269996f0`.
Scope: local implementation/backend proof; **no AWS deployment or live switch changes**.

## Real-container path

`TestIndependentAgentImages_Kind` passed on a separately created local Kind
cluster `worker-images-240`, pinned Agent Sandbox v0.4.6, new disposable namespace
`image-contract-240-rerun`. One application service/runtime instance registered
three immutable templates and ran them without service restart/configuration
replacement. Model responses were explicitly synthetic; Worker processes,
protocol exchange, gateways, facts and cleanup were real.

| Artifact | Execution | Provider calls | Assigned Worker release |
| --- | --- | --- | --- |
| `agenova-worker-240:a` | Succeeded | 2 | Confirmed |
| Independently built `agenova-worker-240:b`, shell-wrapper entrypoint | Succeeded | 2 | Confirmed |
| `agenova-worker-240:incompatible`, no Agenova executables | Failed, `runtime-worker-control-unavailable` | 0 | Confirmed |

The variants have distinct image manifests (`7c7fd052…`, `416933f2…`,
`cc155e98…`) and distinct assigned Worker identities. Same-identity image
replacement is rejected by immutable registration. The test also confirms removal
of its owned namespace. The initial harness run failed only on namespace cleanup's
30-second command timeout; cleanup was verified independently, then the harness
was corrected to poll explicit absence and reran successfully (68.148s).

Reproduction commands and flags are in the [image guide](../../docs/development/agent-images.md#reproduce-the-local-image-regression).
Raw generated local output is ignored under `.tmp/worker-images-240/`; no secrets
or member data are included in this summary.

## Focused and repository gates

- Go: app registration, bundled adapters, runtime, console, evidence and CLI
  tests pass. Missing/malformed acknowledgements remain fail-closed; raw worker
  text is not returned as diagnostics. Structured pull-failure classification and
  pending-Pod creation are unit tested. Existing cross-claim and cleanup tests run.
  Runtime command output capture is bounded to 1 MiB independently of timeout.
- Release transport: **38/38** offline Node tests pass. Main defaults automatic;
  obsolete opt-in `false` values cannot leave it silently disabled. New explicit
  `*_DISABLED=true` stops remain. Worker-only publication does not patch/restart
  the platform. PR/docs-only events never enter cloud jobs. Active claims and
  configuration drift block patching; guarded failure rollback remains tested.
- Actionlint v1.7.7 passes for CI and release workflows.
- Frontend `npm --prefix ui run check` passes, including **69/69** browser smokes,
  contracts, types, components and production build.
- `scripts/check.ps1 -Docs` passes. Full `scripts/check.ps1 -All` was executed on
  Windows: module/format/vet pass; the pre-existing owner-invitation directory-fsync
  test fails on Windows. The initially stale image-default CLI assertion was
  corrected and rerun successfully. No unrelated test is skipped/weakened in CI.
  Full Linux/race CI is required before acceptance.

## Deliberate limits

- Local in-process reference identity is not production Google/OIDC proof.
- No new generic execution protocol, hostile-image isolation attestation or real
  LLM behavior is claimed. Registry auth/image-pull capability remains an operator
  prerequisite. Image failure before binding can remain an allocation timeout.
- Quiet-window rollout is not a drain/admission lock. A team must avoid submissions
  during single-instance Recreate rollout; a racing new submission remains a known
  operational limit. Automatic schema migration is not included.
- Production EKS/CI release execution is intentionally deferred until the Owner
  approves this PR. Existing templates/images are not automatically updated.

## Independent review fixes — PR #241

Both findings against `a5b8f2f` were accepted and fixed without changing the
execution protocol or shared authorization contracts:

- **P1, same-name project collision:** production configuration now uses one
  adapter-owned `ConfigureAgentTemplate` path. It derives resource identities
  from the full immutable resolved template (160-bit content key with upstream
  name/suffix headroom). Different images, commands and ceilings cannot repoint
  the same runtime template/pool. Equal-content warm capacity is not claim or
  authority sharing; old name-based resources are not adopted or deleted.
- **P2, candidate bypass of emergency stops:** snapshot/publication/deployment
  conditions apply the stops outside both mode alternatives; candidate script
  entry also checks the common stop gate before any subprocess/cloud action.

Focused proof:

- Concurrent unit regression configures two same-named image snapshots repeatedly
  and checks both immutable template/pool pairs still bind the correct image.
  Separate command-only and ceiling-only changes produce separate runtime keys.
- The real-container regression now registers `engineer` in three independent
  project registries, prewarms every variant, runs A -> B -> A -> incompatible,
  and independently reads each successful assigned Pod's image and command.
  One unchanged application service/runtime passes all four runs: model calls
  2 / 2 / 2 / 0; all Work cleanup confirmed; test-owned namespace removed.
  `TestIndependentAgentImages_Kind` passed in 105.341s on disposable local cluster
  `worker-review-241`; raw output is ignored in `.tmp/review-241/`.
- **39/39** Node release tests pass, including evaluation of the actual workflow
  conditions for main and candidate modes. Release stop blocks snapshot,
  publication and deployment; deploy-only stop permits publication but blocks
  deployment. Invalid values fail closed. Candidate CLI gates are tested too.
- Focused Go tests and Actionlint pass. Full Windows `check.ps1 -All` again
  reaches only the existing owner-invitation directory-fsync failure. Local race
  execution is unavailable with this host's CGO disabled; complete Linux/race CI
  remains mandatory and is not weakened or skipped.

Identity/model responses in this local test remain explicit fixtures. It is not
AWS, Google/OIDC or a claim of hostile-container isolation. No live release
switches, AWS resources, shared-cluster configuration, merge or deployment change.
