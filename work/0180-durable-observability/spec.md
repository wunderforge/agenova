# Feature Specification: Durable evidence and OpenTelemetry observability

- Ticket: [#180](https://github.com/wunderforge/agenova/issues/180)
- PRD outcome: [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability).

## Intent

Governance evidence answers what Agenova authorized and observed. Operational telemetry explains failures and latency. Evidence remains authoritative across restart; lost telemetry cannot change a governance conclusion.

## In Scope

- Durable request/principal/policy/template/authority snapshots, claim state, immutable facts, invocation attempts/outcomes and results.
- Permission-filtered history, shared diagnosis, OTLP export and independently deployed reference stack.

## Out of Scope

- New identity provider/read policy, Work resurrection, multi-replica database operation, production compliance/HA, safe revision cutover and automatic remediation.

## Requirements

### Durable governance

- Commit admission request, trusted principal, original policy snapshot and decision before allocation; retain template/effective authority/profile revision for allowed Work. Denied Work has no fabricated claim.
- Facts have stable identity, monotonic sequence and immutable request/claim/invocation ownership. Transactionally append facts and update projections; rollback publishes neither partial state nor reused identity.
- Commit decision/attempt before external dispatch. Failed critical writes block continuation and trigger bounded cancellation/cleanup where possible. No implicit in-memory fallback or successful evidence response after a failed commit.
- Reopen the same PVC after Pod replacement. Corrupt/unavailable storage or unsupported schema fails readiness without deleting records.
- Incomplete provider attempts remain unknown after restart; append one idempotent interruption observation, never replay calls or register recovered claims as live authority. Preserve existing terminal claim phases and distinguish unavailable final output and unknown cleanup from Work result.

### Queries and compatibility

- Preserve existing request/claim evidence endpoints. Add `/api/history?limit=<n>&cursor=<opaque>` returning summaries and an optional next cursor. Default 20, maximum 100; reject malformed/oversized cursors and invalid limits.
- Descending immutable creation order plus request identity determines paging; first page establishes an upper bound so newly submitted Work does not reshuffle the traversal.
- E14 supplies verified viewer context/read policy. Filter before pagination; reauthorize every page and item. No subject/team/operator header or request field grants access. Missing viewer/authorizer denies authenticated queries.
- Unauthorized and missing items use the same not-found response. Cursors carry ordering only and disclose no hidden records/counts.
- Summaries contain bounded task summary/type, request/optional claim identity, status/timestamps, failure stage/code and optional trace ID, excluding full input and provider payloads.
- Optional evidence trace/diagnosis fields are additive. Do not replace the legacy `/api/requests` array with an envelope; keep it bounded/authorized and migrate CLI/Portal history discovery to the new endpoint.
- Explicit fixed-identity local demo remains reference-only, never fallback for failed authenticated mode or evidence of E14 completion.

### Telemetry and diagnosis

- Emit Work root and child spans for admission, allocation/readiness, worker execution, governed calls and cleanup; retain context across asynchronous execution.
- Use request/claim/invocation/trace identifiers for correlation, never metric labels. Metrics use fixed component/operation/result/reason categories for counts and duration histograms.
- Structured logs and span attributes use typed allowlists of safe fields. Never serialize requests/facts/raw errors, authentication headers, full prompts, provider payloads or tool/Memory bodies.
- Bounded export queues, timeout and shutdown drop telemetry on outage/saturation and expose safe health. Export failure cannot block governance or rewrite Work outcome. Trace absence is explicitly unavailable.
- Diagnose policy/invocation denial, runtime allocation/start/cleanup failure, provider failure, deadline/cancellation and restart interruption using stable codes and trusted observations.

### Installation

- Explicit named observability instances resolve an OTLP adapter; shared Platform types hold references, while Kubernetes component/PVC configuration stays in the deployment adapter.
- Platform apply installs five separate single-replica components: Collector, Grafana, Tempo, Loki and Prometheus, with pinned images, internal Services, health checks and persistent backend volumes.
- Provision Grafana datasources/dashboards for results/denials, faults, latency and export health. Work trace links use an operator-configured loopback URL and validated trace IDs.
- Grafana credentials use server-side Secret references; no plaintext Platform value or worker/backend credential propagation.
- Backend retention/compaction is configured for seven days; evidence has no automatic pruning. Cleanup preserves PVCs by default and separates explicit data deletion.
- Validate/plan are read-only and do not retrieve secret values; apply preflights storage/RBAC and reconciles idempotently. Report partial failures and distinguish component readiness from verified signal ingestion.

## Negative Cases

- Storage rollback before allocation/attempt produces zero corresponding downstream calls; crash after dispatch retains unknown outcome and never retries.
- Cross-team history, item and cursor reuse expose no unauthorized data or existence signal.
- Collector outage/full queue leaves governance intact; secret/payload canaries never appear in serialized signals or dashboard labels.
- Missing storage/RBAC/Secret reference or backend failure cannot report ready/ingested or fall back to mocks.

## Compatibility

- Old Platform files without observability remain valid and install no telemetry stack. Memory mode remains an explicit local reference fixture.
- Connected-client validation accepts additive fields without weakening correlation or decision checks.
- E14 owns real identity/read policy; injected trusted contexts validate the boundary only. Keep #180 open until real-user integration passes.
- Persistence does not satisfy #169's active-Work rollout/cleanup contract.

## Open Decisions

- Product choices are settled by the selected plan. Packet approval and the accepted E14 integration interface remain implementation gates; update this packet if upstream contract changes require adaptation.
