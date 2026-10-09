# Technical Design: Deliver scoped cross-Work memory with PostgreSQL

- Ticket: [#179](https://github.com/wunderforge/agenova/issues/179)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

- Baseline main is `ebb4c04f7b8cc76d63036c820e04c62af68fad3d`. Existing Memory scopes are strings in request/template/effective authority; there is no executable Memory operation grant or backend.
- The installed service rejects Memory authority. Platform currently configures deployment/runtime/model capabilities; optional Memory routing must follow accepted registry/version-lock patterns.
- The controlled worker is executed against an authoritative backend identity. Its callback captures the claim, validates target/context, and checks Running state. Extend this trusted reference path; do not expose a bare-claim-ID network endpoint.
- Journal invocation stages recognize Tool/Model decisions; a Memory decision must join those state checks before sharing provider attempt/outcome mechanics.
- The evidence view currently embeds the full request and successful model text. Redaction of invocation facts alone is insufficient; public projection must also handle those two echo paths.
- The project uses Go 1.22. Do not raise its toolchain incidentally to install a database driver.

## Decision

Use a small backend-neutral Memory service and a single direct PostgreSQL
adapter. Explicit Write and literal-keyword Search prove governance and durable
cross-Work knowledge without introducing an extraction model or Agent framework.
The adapter is replaceable; the Memory service alone owns invocation authority.

### Trusted Data Flow

1. Operator configuration identifies a backend and named logical scopes with trusted Team/Project ownership; validate this without retrieving secret values.
2. Admission verifies supported Memory operation/scope pairs and captures a defensively copied route binding from the effective Platform revision.
3. Worker startup receives only its allowed logical scope/operations. No backend endpoint, SQL filter, or long-lived credential is passed to it.
4. A run-owned handler establishes a private invocation binding from the issued principal/action, claim, and authoritative backend worker. Worker arguments cannot construct that binding.
5. Memory invocation validates input, target consistency, Running state, deadline, operation/scope grants, and route ownership. Deny before any PostgreSQL connection/query/embedding operation.
6. The journal records MemoryDecision and ProviderAttempt before an allowed adapter call. The adapter receives only a trusted namespace, bounded body/query, deadline, and host-issued invocation ID.
7. Record a typed sanitized outcome before returning a reply. Recheck active context before returning body data; fail Work if required evidence cannot be recorded.
8. Export public views through one redaction projector. The runner retains original request/body privately; connected clients validate the redacted shared view.

### Interface and Ownership Boundaries

- A new `internal/memory` boundary owns typed Write/Search requests, typed results, limits, trusted-session binding, and reusable adapter contract cases.
- The persistence interface takes a context plus trusted namespace and bounded input; it contains no SQL, Kubernetes, vendor SDK, or model framework types.
- The shared service text validator excludes NUL from body/query and returned body text, matching the adapter's defense-in-depth validation. Request rejection precedes Allow/attempt/dispatch and has one metadata-only denial. SQL-driver connection, transaction and statement counters prove denied calls never touch the host DB pool; this is unit proof, not a live PostgreSQL claim. Newline/tab/Unicode and literal backslash sequences remain data, not additional prohibited characters.
- The PostgreSQL implementation lives under its adapter boundary. It owns database connection/configuration, SQL, migrations, RLS, receipts, and backend error mapping.
- The composition layer constructs sessions from authoritative issued state, not from public EffectiveAuthority-shaped values. Reuse accepted E14/E16 construction helpers when merged; do not freeze a competing trusted-context API.
- Requested/template/effective `memoryOperations`, strict parsers, proof copying/digest/provenance, and canonical fixtures are one shared-contract slice.
- The fact journal adds a typed optional Memory metadata payload restricted to Memory outcomes. CLI/UI derive display from those same facts, not from a second Memory history model.
- Platform exposes only an opaque credential reference at the shared boundary; its Memory adapter configuration owns database address/name and connection policy. No credentials in task-authored fields.

### Persistence and Isolation

- Reference backend: PostgreSQL 18, patched stable official image pinned by digest after environment verification; no pgvector or embedding service in v1.
- Driver: stable pgx v5 release whose complete dependency graph supports Go 1.22, locked after a focused compatibility/security check. An incompatible current release is not a reason to raise the repository toolchain in this ticket.
- Adapter-core refinement: consume a host-owned `database/sql.DB` and implement PostgreSQL SQL/transaction semantics without selecting or registering a driver yet. The installed host composition must select a supported, security-reviewed driver and consume #155; no adapter DSN/env/secret resolver is introduced. pgx v5.7.6 requires Go 1.23, while v5.7.2 supports Go 1.21 but falls below the fix in [GHSA-j88v-2chj-qfwx](https://github.com/jackc/pgx/security/advisories/GHSA-j88v-2chj-qfwx). The advisory applies to particular non-default simple-protocol queries; do not add that old dependency merely to avoid a toolchain decision. Driver locking remains an explicit installation blocker.
- Records contain opaque Memory ID, team, project, logical scope, body, trusted source Claim ID, server timestamp, and a receipt-linked invocation ID. Index the ownership tuple and creation order. The adapter bounds each ownership component and claim/invocation identifier to 256 bytes; installed validation must expose these limits before accepting routes.
- Receipts contain the ownership tuple, invocation ID, private request digest, Memory ID, and committed response metadata. Insert row and receipt atomically with uniqueness on namespace/invocation ID.
- Write returns only after known commit. Distinguish uncertain acknowledgement from definite transaction failure. No automatic write retry under a new ID; an explicit authorized retry of the same invocation can resolve from its receipt.
- COMMIT SQLSTATE `40003` (`statement_completion_unknown`) is an uncertainty exception to class `40`, including wrapped driver errors. Return `ErrWriteUncertain` so the governed session retains its original receipt continuation. Known rollback/serialization/deadlock and existing constraint/failed-transaction cases stay definite failures; raw driver text never becomes a result or fact. [PostgreSQL's error-code table](https://www.postgresql.org/docs/current/errcodes-appendix.html) is the adapter mapping reference.
- Review correction: `WriteUncertain` returns an opaque host-only `WriteRetry` continuation, excluded from worker JSON. `Session.RetryWrite` preserves the original receipt ID and immutable request, is bound to the originating Session, and reruns current authority/deadline/ownership checks. Every dispatch has a fresh audit invocation ID so the journal retains one decision/attempt/outcome per attempt; the backend's write ID remains the original trusted logical invocation. Copies share a serialized continuation state. Caller/run cancellation while waiting stops before admission with a context error and creates no invocation or backend call. Known completion or evidence failure disables the continuation; transient failure leaves the original uncertainty unresolved. No worker-supplied ID, changed content, automatic retry, durable claim restoration or new credential boundary is introduced. Installed worker retry signaling/host retention remains part of the later controlled-worker integration, not this in-process fix.
- Application SQL uses explicit namespace predicates and parameter binding. Search uses `strpos(lower(body), lower(query))` and newest-time/ID ordering, so wildcard and SQL-like text remain literal data.
- Use enabled and forced RLS, a migration owner separate from the non-owner application role, and no application `BYPASSRLS` or schema-management permission. Missing namespace settings default-deny.
- Set team/project/scope only transaction-locally, under trusted adapter values. Search uses a read-only transaction. Test alternating namespaces on the same pool connection to catch context retention.
- Migrations are versioned and run through an operator-owned setup step, not ordinary worker calls. Check schema compatibility on adapter startup; a mismatch fails capability readiness explicitly.
- The initial migration is an apply-once transaction owned by the operator, with namespace-scoped primary/foreign keys and separate SELECT/INSERT RLS policies. Startup checks schema version, enabled/forced RLS, a non-owner/non-administrative application role, required SELECT/INSERT permissions and absence of UPDATE/DELETE/TRUNCATE/schema-CREATE permissions. Reapply/role provisioning belongs to the later installed slice; SQL text and driver spies are not live database proof.
- Migration-marker readiness is separate from the tenant-table aggregate: `schema_version` requires SELECT, no ownership/owner-role membership, and no INSERT/UPDATE/DELETE/TRUNCATE or column-level INSERT/UPDATE. It does not require RLS or INSERT. A false, missing, NULL or malformed safety result cannot produce a ready backend. The same single bounded query owns all readiness checks.
- Tenant readiness also rejects column-level UPDATE, including inherited-role and PUBLIC grants, on both records and receipts. Real PostgreSQL tests execute the production readiness query and distinguish table-level from column-level privileges. The separate opt-in SQL gate additionally verifies the production migration, RLS, transaction-local ownership reset and database-container restart through psql; it does not select a Go driver, consume #155 or prove installed worker/Pod-PVC behavior.
- Resolve DB credentials only on the trusted host through #155's accepted resolver. Rotation closes/rebuilds idle host connections through the documented restart/reconcile path; active claim authority is unchanged. Never print DSNs or raw driver errors.
- Reference database uses a private Service and a task-scoped StatefulSet/PVC. Keep data through ordinary restart/reapply; cleanup explicitly names the owned namespace/PVC and never deletes a cluster or unrelated storage.
- Enforce database ingress from the Control Plane only. Test fresh worker startup and existing worker reachability using the actual CNI, not policy text alone.

### Evidence and Redaction

- MemoryDecision participates in the existing one-decision/one-attempt/one-outcome correlation state machine. Metadata includes result class, logical scope/operation, returned count, bounded duration, truncation, and validated logical references.
- Stable reason codes classify Memory denial, unsupported/unavailable configuration, timeout/cancel, backend failure, write uncertainty, and evidence failure. Raw database or worker input does not become a reason string.
- Default projection for any Memory-requesting Work sets `Request.Spec.Task.Input` to a new empty map, clears `Outcome.Text`, and adds the two documented `contentRedactions` paths.
- Keep original canonical data private; redact a copy only at every public submission/query/list/detail serialization boundary. Preserve authority-bearing fields so readers can still validate scope/operation/no-expansion and invocation correlation.
- Use explicit `evidence.ProjectPublic` for application queries and `evidence.MarshalPublic` at HTTP/CLI outputs. A custom View serializer conflicts with the repository's reflection-generated contracts; do not change the generator or canonical task-input `omitempty` tag. Only the evidence serializer restores the required empty input object on the wire. Standard serialization remains available for private execution-state cloning, not public exports.
- Validation rejects unknown/missing redaction, hidden fields carrying data, ungranted MemoryDecision targets, orphan/reordered provider facts, and cross-Work identity reuse.
- UI displays metadata and a fixed withheld-content label. No reveal/write management UI is added. Non-Memory Work retains existing behavior.
- Reader slice: CLI and Portal accept Memory facts only with explicit content projection, issued/requested operation and scope checks for Allow, claim/policy/target correlation, ordered decision/attempt/outcome and bounded metadata. Every Memory decision, including Deny, requires an earlier Running fact consistent with `Session.Bind`; allocation/readiness or a later Running event is insufficient. Deny has no attempt/outcome; `memory.invalid` is Deny-only. Reject free-text reason/unknown reason codes or unrelated payloads on Memory facts. Memory in-flight writes can report known commit or uncertainty after cancellation/expiry; late reads must be withheld (Cancelled/Timeout), and no new Allow may start after terminal authority. Terminal Work requires all allowed invocations closed. Shared public-view/corruption/lifecycle vectors cover both readers; rendered fixture proof remains separate from installed acceptance.
- The campaign's private oracle receipt verifies computed use of a generated fact, while screenshots and ordinary CLI/API exports contain no raw Memory content.

## Ownership and Contract Boundaries

| Producer | Owns | Consumer action |
| --- | --- | --- |
| E17 | Memory operation/scope authorization, data path, persistence, privacy projection, acceptance | Keep all Memory changes in the task's five implementation slices |
| E14 #155 | Typed host-side credential resolver and opaque references | Consume the accepted contract before credentialed live evidence; no Memory-specific secret system |
| E14 #204/#207 | Generic immutable Policy evaluation/constraints | Add Memory caps through accepted evaluator fields and preserve missing-versus-empty semantics |
| E14 #205 | Authenticated claim-bound worker identity | Reuse after merge and rerun authenticated negatives; reference closure binding is the initial path |
| E16 #192 | Shared routing/worker/fact additions for real MCP | Coordinate overlapping modules and adapt to accepted producer code without merging its whole work branch |

## Alternatives Considered

- Mem0 OSS: useful later for automatic extraction/semantic retrieval, but adds an HTTP service, provider/model configuration and version-dependent API semantics beyond this acceptance baseline.
- LangGraph Store/LangMem: appropriate inside an existing LangGraph Worker, but introducing that framework solely for persistence into this Go control plane is unnecessary.
- Graphiti/Zep: useful temporal/relationship context, with additional graph/model or managed-service dependencies not required for E17.
- Letta: an Agent runtime choice rather than the smallest independent persistence boundary for this product.
- AgentCore Memory: a cloud adapter candidate after local acceptance, not the selected self-hosted baseline.
- Process memory or audit facts as retrieval storage: rejected because they do not prove restart durability or authorized reusable knowledge.
- pgvector/embeddings now: deferred until a measured retrieval requirement justifies semantic capability and its governed provider path.

## Verification Strategy

| Layer | Positive evidence | Negative evidence / oracle |
| --- | --- | --- |
| Contract | read/write exact intersection and immutable issued copy | missing operations, unrequested additions, malformed shapes, changed proof input |
| Memory boundary | active captured session and valid owner call | spy proves zero calls for wrong target, phase, grant, owner or input |
| PostgreSQL | atomic row/receipt, literal search, explicit empty, repeat receipt | real RLS/pool isolation, duplicate mismatch, SQL-like text, timeouts and uncertain commit |
| Installed worker | A Write, independent B Search/use, restart then C Search/use | backend-side zero-call controls; B/C never receive expected values |
| Network | Control Plane can reach PostgreSQL | fresh/steady-state worker cannot, with independent probes and actual CNI output |
| Evidence/UI | consistent CLI/API/Portal operation/result metadata | content/query/error/credential sentinels absent; corrupted attribution/order/redaction rejected |

- Each slice runs its focused gate followed by `./scripts/check.ps1 -All`.
- Real campaigns require explicit context/namespace/version and fail when the selected live environment is unavailable. Compile-only/unit/fixture checks are reported separately.
- Capture sanitized raw output, exact commands, database image/driver/CNI identities, resource/PVC identities, browser screenshots, and scope-limited cleanup instructions.
- Completion requires independent teammate reproduction; absent evidence remains a blocker rather than a production-readiness claim.

## Risks and Compatibility

- Credential resolver delivery may extend the 12-15 day estimate. Fake-boundary development is possible first; live acceptance cannot bypass #155.
- Shared-file conflicts with E16/E14 require rebasing and replaying focused integration tests. Accept their reviewed contracts rather than hard-coding current drafts.
- The declared PostgreSQL/runtime/network configuration is not verified until the live campaign. kind storage classes and CNI must be preflighted.
- In-flight Write may commit before cancellation is observed. Outcomes must preserve commit/uncertainty truth rather than claim distributed rollback.
- Database data and application journal are not one atomic store. A lost outcome after commit fails Work explicitly; Memory durability does not imply durable evidence.
- Current reference identity remains fixed/in-process. This design does not claim OIDC tenancy or authenticated hostile-worker protection; #205 integration gets a separate rerun.
- New Memory views require coordinated service/CLI/UI deployment because strict older readers reject unknown additive fields. Existing non-Memory documents/views omit new optional fields.
- Task-input/outcome redaction means default public Memory views are not replay artifacts. Reproduction uses the controlled task-owned manifests and private campaign inputs.
- On 2026-10-08 the user explicitly authorized latest main's assignee-self-review workflow, superseding the initial preparation stop. Normal independent PR review still applies.
