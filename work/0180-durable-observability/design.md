# Technical Design: Durable evidence and OpenTelemetry observability

- Ticket: [#180](https://github.com/wunderforge/agenova/issues/180)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

- `facts.Journal` holds correlation/order in memory; `console.Service` separately retains views/results. Live evidence reads synchronize through the runner lifecycle lock.
- Runtime/Gateway observers return write errors, but some outcome appends are ignored. Persisting terminal views alone would lose pre-dispatch attempts and crash evidence.
- Installed composition uses fixed Team A identity; E14 must supply real viewer context/authorization. Platform currently supports only model service instances and adapter-owned deployment configuration.

## Decision

### Repository and recovery

Introduce an internal repository interface for atomic admission/snapshots, validated fact/state append, final outcome, item lookup and ordered history queries. Use SQLite through a pinned pure-Go driver compatible with the repository toolchain, using the existing in-memory Journal as the correlation/ordering oracle. Repository behavior is exercised against real SQLite plus an injected failure decorator; no second durable memory repository is added. SQL/driver types stay in the adapter.

Version schema metadata; store Work ownership/admission snapshots, immutable ordered facts and indexed claim/request mappings. Preserve the original evaluated policy/template, not current registrations. Reuse Journal correlation validation as shared logic; transactions atomically append and update projections before publishing in-memory state. Avoid a separately authoritative memory journal.

Use WAL, foreign keys, full durability, bounded busy timeout, a local PVC, one control-plane replica and exclusive database ownership. Fail initialization/readiness on unavailable or unsupported storage. No shared filesystem/multiwriter promise. Migrations preserve records and reject unsupported versions.

Propagate critical storage errors through admission, observers and finalization, replacing ignored append errors. Dispatch is forbidden until its required decision/attempt commits. After a dispatched call, failed outcome persistence leaves a durable unknown attempt, not successful evidence. Cancellation/teardown must remain possible during storage outage; unrecorded cleanup is uncertainty, never claimed success.

Recover unfinished records transactionally and idempotently. Preserve terminal phases, mark missing result text unavailable, append interruption metadata and derive unknown attempts from absent outcomes. Never populate the runner authority registry from recovered records or automatically replay/resume/claim cleanup. Retain backend identities for operator investigation; #169 remains separate.

### Queries and E14

The application query service accepts trusted viewer/authorizer interfaces from E14 composition, independent of HTTP token parsing and storage. Missing context or authorizer denies authenticated access. No ad hoc user/team/operator headers.

Implement the history envelope/cursor rules in spec.md. Encrypt/authenticate cursor ordering state with AES-GCM and a repository-local persisted key; a base64 global sequence would expose hidden Work counts. Schema v2 adds this key transactionally while retaining v1 records; older plaintext cursors are rejected. Filter candidates before filling the page; cursor progression tracks scanned records to handle sparse permissions. Reauthorize all requests. Load historical views on demand instead of loading the complete database into the service map. Preserve lifecycle-consistent active reads.

CLI/Portal use paginated summaries then existing evidence routes. Keep the old list wire shape bounded and authorized. Add optional correlation/diagnosis fields derived from trusted facts; never turn incomplete provider status into Failed. E14 fixtures are explicitly fixtures until real integration passes.

### OTEL

Add a small internal telemetry package configured at control-plane composition. Use official Go OTEL trace/metric/log SDKs and OTLP/HTTP exporters, pinning a compatible release set after checking supported Go versions. Public contracts contain only diagnostic identifiers, no SDK types.

Create a Work root span before admission, carry its context across asynchronous execution and finish after result/cleanup observation. Add child spans at runtime and governed dispatch boundaries; existing exec/stdio workers need no provider credentials or privileged telemetry endpoint. Persist optional trace correlation as diagnostic metadata only.

Use typed event/attribute allowlists and fixed metric categories. Never pass arbitrary errors or raw requests to instrumentation. Export uses bounded asynchronous batches/timeouts/shutdown; failure health must avoid recursive exporter logging. Reference E2E explicitly uses full trace sampling. Disabled mode is a no-op; enabled invalid configuration fails clearly, never mock fallback.

### Platform and stack

Extend service schema/resolver/registry with explicit observability instances using existing instance-reference patterns. OTLP configuration belongs to its adapter. Managed component/PVC settings and evidence volume belong to deployment-adapter configuration, not Work or runtime worker input.

Render independent Collector, Grafana, Tempo, Loki and Prometheus components through current reconciliation. Collector receives OTLP/HTTP, exports traces/logs to Tempo/Loki and exposes a metrics scrape endpoint for Prometheus. Grafana provisions those three datasources and versioned dashboards. Separate PVCs persist backend/Grafana state; bounded ephemeral Collector queues are acceptable observable telemetry loss.

Configure seven-day retention in each backend, including Loki compaction/deletion prerequisites. Pin exact tested image versions/digests, declare requests/limits and readiness probes, and record the pins in integration evidence. Avoid floating charts/tags. Resolve Grafana credentials only through server-side Secret references. Use internal endpoints with explicit loopback Grafana port-forward instructions.

Extend preflight/plan/apply/status with storage/RBAC checks, stable owned resources, partial-failure reporting and idempotent drift behavior. Readiness is not ingestion verification; live backend queries establish signal delivery. Cleanup removes only owned resources and preserves PVCs unless separately explicitly deleted. Add no automatic drain, upgrade or rollback mechanism.

## Ownership and Contract Boundaries

- Application owns repository/query interfaces, correlation and diagnosis; SQLite adapter owns schema/transactions.
- E14 owns verified identity/read policy; #180 consumes it and tests filtering before pagination.
- Telemetry owns safe OTEL records; export health cannot influence governance conclusions.
- Platform owns generic references/capabilities; adapters own SQL/Kubernetes/provider particulars. CLI/Portal render shared evidence; Grafana is diagnostic.

## Alternatives Considered

- Terminal JSON snapshots lose attempts and transactional ordering.
- PostgreSQL adds an operated service unnecessary for the single-instance reference path; keep repository replaceable.
- LGTM all-in-one was offered; user chose independent components.
- Separate shell installer cannot satisfy Platform command idempotency.
- Implementing identity here would duplicate E14 and hide its acceptance dependency.

## Verification Strategy

- Repository behavior against real SQLite and the existing memory Journal: uniqueness, snapshots, defensive copying, immutability, concurrent append, rollback, paging and reopen.
- Inject write/commit failures and assert zero allocation/provider calls with allowed positive controls. Use a subprocess crash after attempt to prove no replay/authority restoration and idempotent recovery.
- Test cross-team item/page/cursor behavior with injected trusted E14 contexts; require real two-user installed E2E later.
- Capture all three OTLP signals locally and inspect allowlisted fields, canary absence, bounded outage and shutdown.
- Adapter render/drift/preflight/partial failure tests plus real kind first/repeated apply, backend queries, component restarts and Collector outage.
- CLI/API/Portal restart parity and Grafana/Portal rendered screenshots; focused/full checks and exact sanitized revisions/commands/pins.

## Risks and Compatibility

- SQLite single-writer/PVC limits are explicit; history persistence is not HA or safe rollout.
- Refactoring mutable state/ignored appends is sensitive: prevent duplicate authoritative stores and inconsistent lifecycle/fact projections.
- Database contains user task/results even though telemetry excludes them; protect volume and authorized read boundaries.
- E14 is absent from the inspected baseline. Fixtures are not real identity evidence; keep #180 open while its integration gate is blocked.
- Interrupted Work may leave runtime resources. Record known/unknown cleanup honestly and retain the existing operator process.

## Reconciliation with current E14 implementation

PR #235 now retains upstream authenticated PostgreSQL Work storage and live session/project authorization. SQLite is reference-only; the service rejects competing storage owners. Authenticated history is implemented by the PostgreSQL adapter using the same membership/project SQL predicate before keyset pagination. Its encrypted cursor key belongs to the exclusive writer lifetime, so a restart invalidates pagination hints without deleting Work; SQLite retains its persisted cursor key. Both paths preserve governance/telemetry separation and never restore stored claim authority. An admission's original reference-policy/template snapshots remain private storage metadata; generic Policy evaluators remain backend-neutral and may optionally describe their snapshot.
