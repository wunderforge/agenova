# Local acceptance — 20 October 2026

Owner: [#242](https://github.com/wunderforge/agenova/issues/242). Branch:
`codex/mcp-incident-demo`. No push, PR, merge, cluster apply or AWS resource/config
change was performed for this implementation. The separately authorized sanitized
#241 summary was published before this work.

## Delivered

- [Scenario and reproduction](../../harness/demo/incident-mcp/README.md): one
  existing `read_file` MCP tool, four synthetic files, two disjoint profiles,
  one template/policy, two successful task inputs and one admission-deny input.
- Minimal internal-service/NetworkPolicy/data manifests, rendered locally only.
- Real SDK process, verified local HTTPS bridge, actual reference Worker process,
  installed tool builder, real application admission/resolution/identity/Gateway
  and HTTP submission/evidence API exercised together. Runtime allocation is the
  **memory reference** and model replies are **scripted, not inference**.
- Both tasks read their own two files. Trusted platform invocation IDs match
  independent SDK handler receipts. While Running, cross-scope and disguised-file
  probes add zero downstream tools/call. Terminal replay and admission deny do
  not dispatch; denied admission allocates no Worker.
- Valid/missing/wrong synthetic service credential cases passed against the SDK:
  valid reads; missing sends no HTTP; wrong sends no tools/call; token values are
  absent from receipts/results. This does not inspect actual Secret stores or Pods.
- One bounded production diagnostic addition: pre-Gateway parameter validation
  records `WorkerActivity / ToolRequestRejected / tool-arguments-rejected`, never
  raw arguments, endpoints or tokens. Failure to write it fails the Work closed.

## Exact local verification

| Command | Result |
| --- | --- |
| `go test -tags integration ./cmd/agenova-control-plane -run '^TestIncidentDemo' -count=1 -timeout 3m -v` | pass: input/resolution, real local SDK/Worker chain and three credential subcases |
| Same command with `-count=10` | pass, 60.197s; deterministic integration repetitions, NOT real-model reliability |
| `go test ./internal/console -run 'TestArgumentRejectionEvidenceFailureStopsWork\|TestConfiguredServiceAdvertisesGrantedRoutesAndRejectsUnconfiguredArguments' -count=1` | pass |
| `go test ./...` in `harness/integration/mcpfixture` | pass |
| `kubectl kustomize harness/demo/incident-mcp` | pass; Namespace, ConfigMap, Service, Deployment and NetworkPolicy rendered; no API contact |
| `./scripts/check.ps1 -All` on Windows | docs/contracts/format/tidy/vet pass; Go reaches the existing `TestOwnerBootstrapDeliveryGuards` invitation-file synchronization failure. It was not changed/skipped. Remaining frontend check run separately below. |
| `go test -count=1 ./...` in local pinned Go 1.27.2 Linux container | pass, including the Windows-blocked owner-bootstrap test; source mount read-only, no AWS credentials or Docker socket |
| `go test -race -tags integration -run '^TestIncidentDemo' -count=1 -timeout 3m ./cmd/agenova-control-plane` in that container | pass |
| `go test -race -count=1 ./internal/console ./internal/adapters/bundled ./internal/toolbackend ./internal/toolgateway` in that container | pass |
| `npm --prefix ui run check` | contracts, types, 135 tests, build and 69 Playwright smokes pass |
| `git diff --check` and final documentation check | pass |

Linux runtime: `golang@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8`.
Container `agenova-incident-local-242` used `--rm`; local fixture/Worker processes
and generated test-token files are owned by test cleanup. Checks requiring an
external database/backend remain environment-dependent; a passing source gate is
not a live PostgreSQL/EKS/network-isolation assertion.

## Failures retained in the reasoning

- Initial test compilation used a typed schema-validation error variable for a
  general error return; corrected in tests.
- Initial local HTTP routing reached the SDK but failed its loopback Host guard
  (HTTP 403). The final harness uses verified HTTPS with a local test CA and a
  loopback bridge preserving the SDK's real Host, rather than disabling either
  Host checks or production endpoint validation.
- No existing local Ollama endpoint was available. No fake inference claim or
  cloud-provider fallback was introduced.

## Deployment remains a separate approval

The [scenario guide](../../harness/demo/incident-mcp/README.md#minimum-future-deployment-for-owner-confirmation)
lists the final namespace/image/selector/network/config/credential checks and
targeted cleanup. A shared deployment must preserve current login/model/policy
configuration; the standalone Platform/policy files must not be applied unchanged
over it. Real selected-model rehearsal, actual credential placement and Worker ->
MCP direct-network denial still need final-environment verification. No claims of
AWS or hostile-container isolation are made from these local results.

## Parallel #243 integration follow-up — 20 October 2026

Owner approval now includes one integrated release and cloud verification after
joint gates. This MCP lane still performed no cloud mutation, commit, push or
merge. The original evidence above remains historical, not cloud evidence.

Added a version-controlled connected campaign runner under
`harness/demo/incident-mcp/acceptance` and a sibling AWS Kustomize overlay under
`harness/demo/incident-mcp-aws`. The local manifests/data are unchanged. The AWS
overlay uses the verified shared control-plane selector, a deliberately invalid
digest placeholder, internal-only Service, no token mounts, and MCP egress deny.

The runner has an explicit execution gate, existing CLI login, 20 sequential
tasks, durable start/result records, no retry of unknown submissions, exact
caller/template/Policy/scope/model checks, correlated independent MCP read
receipts, bounded subprocess output and mechanical business screening. Every
answer still requires factual review; native attacks and network isolation are a
separate acceptance. It has not been executed against AWS or a real provider.

The local allocation-counting wrapper now forwards the required NetworkBackend
capability. Both offline incident tasks retain zero public-network grants and
record exactly one **Simulated** activation/revocation. This checks composition
with the new lifecycle, not packet isolation.

Additional verification:

- `go test ./harness/demo/incident-mcp/acceptance -count=1 -v`: pass, including
  missing/duplicate/wrong-scope receipts, missing/unexpected real-model metadata,
  cross-Work facts, unintended public grant, vague answers, no-execute guard and
  output bound.
- `kubectl kustomize harness/demo/incident-mcp-aws`: pass, five objects rendered;
  no API calls. A first nested overlay attempt correctly failed Kustomize's
  parent/child cycle check; using the conventional sibling layout fixed it
  without disabling the loader's restrictions.
- `go test -tags integration ./cmd/agenova-control-plane -run '^TestIncidentDemo'
  -count=1 -timeout 3m -v`: pass, 19.154s, with required local loopback execution
  approval, owned D: build cache/temp directories and no cloud calls. Real SDK
  file reads and Worker process; model remains explicitly scripted.
- `go test ./... -count=1` in `harness/integration/mcpfixture`: pass, 5.281s,
  with the same local loopback approval and owned D: build cache/temp.
- `git diff --check`: pass for the working tree at this checkpoint.

Environment failures were not hidden: default sandbox builds exhausted C: temp
space, so subsequent builds used owned `.tmp/mcp-242-go-*` directories on D:.
Sandboxed fixture sockets then failed with Windows access-permission errors;
the same local gate passed after normal execution approval. No cache directories,
user files or cloud resources were deleted to make the tests pass.
