# E14 Host Credential Foundation

Ticket: [#155](https://github.com/wunderforge/agenova/issues/155),
[Task/Spec/Design](../../../work/0155-host-credentials/task.md).
Baseline: accepted main `ebb4c04f7b8cc76d63036c820e04c62af68fad3d` plus this
isolated producer slice, not the E17 branch. On 2026-10-09 the user authorized
taking #155 and reported no existing kind context. The Ticket is assigned to
`yanyang15037755`; independent review/reproduction is not yet complete.

## Scope and Proof

The internal host-only boundary captures a typed opaque reference, exact
operator allowlist, explicit qualified resolver identity/version and immutable
binding. Scoped use resolves once per call under the caller/five-second minimum
deadline, bounds values to 64 KiB, clears owned buffers and returns only stable
errors. Registry/binding/registration formatting and JSON/YAML are redacted.
Raw callback bytes remain trusted adapter-private data; Go cannot prevent a
trusted consumer copying them. This is best-effort hygiene, not hostile-host
isolation, general secret management or a new authority issuer.

The Kubernetes adapter captures one namespace and exact name/key ceiling,
requires matching v1 Secret identity and Opaque type, bounds JSON to 2 MiB and
rejects missing/invalid/oversized material. Its real transport constructs an
explicit-context/namespace exact `get secret` call, bounds stdout and discards
stderr. It performs no discovery, list, mutation or fallback. All development
tests use synthetic data/scripted getters; no real Secret was read.

Named executable evidence:

- [Contract tests](../../../internal/credentials/credentials_test.go): invalid,
  unknown/unauthorized references and invalid/typed-nil/duplicate configuration;
  defensive capture, buffer clearing, non-cached reload, redacted formatting,
  JSON/YAML, cancellation/deadline, unavailable source and concurrent use.
- [Gateway composition](../../../internal/credentials/gateway_test.go): real
  existing Tool/Model admission plus a credential-backed spy adapter. Pending,
  Bound, terminal, unknown and ungranted claims/profile/tool/scope, reserved
  worker credential input and evidence failure cause zero resolver/provider
  calls. Missing admitted credentials cause one resolution but zero provider
  calls. Positive controls prove the counters; public invocation facts contain
  no material. Worker parameters cannot retarget the captured host binding.
- [Secret/transport tests](../../../internal/credentials/kubernetes/secret_test.go):
  exact authorized namespace/name/key, immutable allowlist, deadline, synthetic
  reload, missing/forbidden/unavailable/wrong-kind/type/identity/key, invalid
  base64/JSON/size, cancellation, explicit command arguments, sanitized raw
  errors and the `io.Copy` output-cap regression.
- Existing #36 request/template/launch/Gateway/worker-manifest exclusions pass
  unchanged in the focused and full gates. No public secret-acceptance path is
  enabled by this internal library.

Early tests reproduced the embedded-buffer `ReaderFrom` size-limit bypass;
private buffer composition fixed it before the final run. A Tool test initially
used a dotted tool plus action and was corrected to the existing contract, not
by weakening production validation. The first WSL full run failed on Windows
gitdir interpretation; only ignored runner configuration was changed. Failed
output is not the selected passing campaign.

## Gates

Environment: existing WSL Ubuntu 24.04, Go 1.27.1, Node 24.12.0, PowerShell 7.6.6,
GCC 13.3, Playwright 1.63.0/Chromium 153. The isolated worktree uses explicit
Linux Git directory/worktree mapping and a dependency junction after verifying
identical package-lock SHA256. Go 1.22 requirements, module/dependency locks,
generated frontend contracts and baseline tests are unchanged.

| Gate | Exact command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/credentials/... ./api/v1alpha1 ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/app ./internal/runtime/agentsandbox` | pass |
| Race | `go test -race -count=1 ./internal/credentials/... ./internal/gateway ./internal/toolgateway ./internal/modelgateway` | pass |
| Repeated | `go test -count=20 ./internal/credentials/...` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 124 frontend tests / build / 52 browser cases |

[Final source output](../../../work/0155-host-credentials/slice1-host-boundary-linux.log)
owns these results. Only evidence/publication documentation followed the passing
source run. The [publication gate](../../../work/0155-host-credentials/slice1-publication-linux.log)
records documentation, PR-body and whitespace checks. CI and reviewer acceptance
for the published commit are separate from local results.

## Remaining Acceptance

Platform typed reference acceptance, credential adapter catalog/activation/lock,
resolver-owned readiness and installed consumer wiring are not implemented by
this first slice. Current Platform still rejects credential references; the
test resolver is not a fallback. Provider-specific integrations stay in their
consumer Tickets. Qualified credential IDs refine the Ticket's provisional
example and remain an internal prototype until coordinated review.

Real Secret/RBAC/reload, installed worker/public scans and independent
reproduction need an explicitly selected existing cluster. The user reported
none; no cluster, Secret, PVC or authentication setup was created or changed.
Keep #155 open and its PR draft. This does not yet unblock credentialed live
E17 or prove database durability, network isolation or hostile-worker security.
