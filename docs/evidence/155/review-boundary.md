# E14 Credential Boundary Review Corrections

Ticket: [#155](https://github.com/wunderforge/agenova/issues/155),
[packet](../../../work/0155-host-credentials/task.md), draft
[PR #213](https://github.com/wunderforge/agenova/pull/213).
Source baseline: `a9eabf4a8f99fa4060bb89ddeed57231f94bf58b` plus these corrections.

## Resolver Deadline

[The first P2](https://github.com/wunderforge/agenova/pull/213#discussion_r4226778082)
identified the resolver's timeout being passed to the consumer. `Binding.Use`
now resolves under a dedicated caller/five-second-minimum child context, captures
resolution cancellation, and releases that child before invoking the callback.
The callback receives the original caller-owned invocation context; Tool/Model
owners, not the credential resolver, determine provider invocation deadlines.
Caller cancellation and resolution failure still stop before the callback.

The new regression failed before the fix for background, one-second and
twenty-second caller contexts. It now proves a bounded resolver child, exact
original consumer context and an independently cancelled resolver child while
the consumer remains active. Existing cancelled/deadline/zero-call tests pass.
This checks deadline ownership without a timing-sensitive five-second sleep.

## Secret Material

[The second P2](https://github.com/wunderforge/agenova/pull/213#discussion_r4226778088)
identified immutable strings for every encoded Secret value. The private JSON
projection now retains `json.RawMessage` byte buffers instead of value strings.
Only the selected string field is decoded with the standard Secret JSON byte
codec. Raw JSON and all owned encoded field buffers, including unrelated keys,
are cleared on return; invalid decoded material is cleared, and valid decoded
material remains owned by the scoped binding.

The selection tests cover valid/JSON-escaped data, unrelated invalid base64 that
is never decoded, missing/empty/null/non-string/invalid/oversized selections and
zeroed encoded buffers on every return. Existing lookup, identity, failure,
privacy, release, concurrency and zero-consumer controls remain passing.
Selected empty or malformed bytes do not become a credential fallback.

Sources: [context regression](../../../internal/credentials/credentials_test.go),
[selection cleanup regressions](../../../internal/credentials/kubernetes/secret_test.go),
[scoped use](../../../internal/credentials/credentials.go) and
[Secret projection](../../../internal/credentials/kubernetes/secret.go).
This proves cleanup of addressable owned buffers, not heap-wide erasure of Go
runtime/JSON-library temporaries or protection from trusted consumers copying
material. It remains best-effort hygiene, not hostile-host isolation.

## Gates and Limits

Native focused tests passed. The final campaign uses existing WSL Ubuntu 24.04,
Go 1.27.1, Node 24.12.0, PowerShell 7.6.6, GCC 13.3 and Playwright 1.63.0.

| Gate | Exact command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/credentials/... ./api/v1alpha1 ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/app ./internal/runtime/agentsandbox` | pass |
| Race | `go test -race -count=1 ./internal/credentials/... ./internal/gateway ./internal/toolgateway ./internal/modelgateway` | pass |
| Repeated | `go test -count=20 ./internal/credentials/...` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 124 frontend tests / build / 52 browser cases |

[Source output](../../../work/0155-host-credentials/review-boundary-linux.log)
owns these results. Only documentation/publication edits followed this run;
[publication output](../../../work/0155-host-credentials/review-boundary-publication-linux.log)
records final Docs, PR-body, staged and whole-branch whitespace checks. CI and
independent review of the published commit remain separate.

Public Platform acceptance, installed resolver/consumer composition, live
Secret/RBAC/reload and independent reproduction remain outstanding. The user
reports no existing kind context. No real credential was read and no cluster,
Secret or PVC was created/modified. Keep #155 open and #213 draft; E17 remains
separate and credentialed live integration is not accepted by these tests.
