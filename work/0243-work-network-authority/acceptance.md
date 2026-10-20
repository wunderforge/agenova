# Integrated acceptance — 20 October 2026

Scope: [#243](https://github.com/wunderforge/agenova/issues/243), jointly released
with [MCP #242](../0242-mcp-incident-demo/acceptance.md). These are distinct layers:
Policy computes authority; the runtime enforces socket boundaries; the trusted
Tool Gateway enforces MCP scope. A model's refusal is not an enforcement test.

## Local evidence

- API, Policy, authority and issuance tests pass, including repeated execution,
  unknown/duplicate profile rejection, explicit request/template/Policy
  intersection and omitted-request default-none behavior.
- Runtime/application tests pass: activation after Running, original claim and
  Pod incarnation, guarded Pod patch, create-only rule ownership, replacement
  and concurrent-activation races, cancellation/deadline release, and cleanup
  despite evidence-write or rule-removal failure.
- Console tests pass, including initial recovery `Failed` append failure:
  verified revocation still runs. Telemetry preserves optional backend
  capabilities and labels configuration/simulation rather than packet proof.
- Frontend contracts/types/build pass; 136 unit tests and all 72 browser smokes
  pass. The offline/online request and historical-unknown screenshots were
  inspected. Historical missing network evidence is not retroactively offline.
- Actual local SDK/Worker MCP composition passes. Inference is scripted and
  allocation simulated in this harness; see #242's precise evidence.
- Independent review has no remaining known major integration defect. Live
  enforcement is still a separate gate, not inferred from this review.

The pinned Linux campaign completed successfully (read-only source mount, no
cloud credentials, D:-scoped cache/temp and automatic owned-container removal):

| Command | Result |
| --- | --- |
| `go test -p 2 -count=1 -timeout 5m ./...` | pass |
| `go test -p 2 -race -count=1 -timeout 5m ./internal/app ./internal/runtime/... ./internal/operator ./internal/console ./internal/authority ./internal/policy ./internal/adapters/bundled ./internal/toolgateway` | pass |
| `go test -p 2 -race -tags integration -run '^TestIncidentDemo' -count=1 -timeout 5m ./cmd/agenova-control-plane` | pass, 47.342s |
| `go vet ./...` | pass |
| `./scripts/check.ps1 -Docs` | pass |
| `./scripts/check.ps1 -Profile Fast -ChangedOnly` | pass |

Pinned image:
`golang@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8`.
Windows has an existing unrelated owner-invitation file synchronization test
failure; it is not skipped or changed. The Linux source gate includes that test.
The test-only native observer was also tested/vetted separately, and its exact
Worker-derived image retains the original control handshake; #242 records this
local image evidence without claiming AWS enforcement.

## Approved deployment boundary

The Owner approved existing-cluster deployment and repairs after joint local
acceptance, namespace-local NetworkPolicy get/create/patch/delete, and guarded
Pod patch. No new Secret/IAM/cross-namespace privileges or paid infrastructure are
authorized by these RBAC approvals. Existing exec permissions are unchanged.

Use the existing CNI, nodes, model and image repositories. Drain Work before
rollout, preserve member login/proxy/model configuration, and remove only
verified obsolete runtime allows/pools. Never recover availability by restoring
open egress. A normal PR/CI gate still applies; branch-protection bypass needs its
own explicit authorization.

## AWS campaign — pending

Do not describe this section as passed until actual retained receipts exist:

1. Independent positive controls for the same public TLS/DNS targets.
2. Worker-first-process probes, idle/offline deny, explicitly granted public-web
   success, no direct MCP/private/metadata access, and same-template peer isolation.
3. Terminal and deadline release with original Pod absence; existing connection
   termination and control-plane-interruption expiry measured separately.
4. Admission denial before allocation and provider dispatch, with independent
   observations wherever available. HTTP rejection alone is not a call counter.
5. Real-model payments/billing tasks, same user/template/Policy and disjoint task
   scopes, independent MCP receipts and factual answer review. Retain failed
   attempts and distinguish business reliability from security acceptance.
6. Login, durable history, observability and independently selected compatible
   Worker-image regressions. No credentials in arbitrary Worker Pods.

There is currently no public Work-cancel API. Closing a CLI/polling connection is
not cancellation evidence. Any live cancellation claim requires an actual
trusted cancellation path; source-level cancellation tests remain labelled local.
