# Feature Specification: Deliver scoped cross-Work memory with PostgreSQL

- Ticket: [#179](https://github.com/wunderforge/agenova/issues/179)
- PRD outcome: Bounded E17 elaboration of [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [backend neutrality](../../docs/product/prd.md#3-backend-neutral-execution).

## Intent

One independently issued Running worker claim may write or search persistent
Memory only within its own granted operation and logical scope. Data survives
service/Pod restart, but authority never transfers with the data. Every consumer
uses the same decision, invocation, and redacted evidence contracts.

## In Scope

- Explicit append-only Write and literal-keyword Search through one governed interface.
- Optional operation grants, immutable trusted ownership, typed results, bounded execution, and metadata evidence.
- One persistent reference backend and one controlled-worker path.

## Out of Scope

- Automatic extraction, embedding/semantic ranking, arbitrary provider filters, object-ID authority, agent orchestration, and general Memory administration.
- Durable Work history and production multi-user/workload identity claims.

## Requirements

### Access and Ownership

- Add optional `memoryOperations` to requested access, template capability ceiling, and effective authority. Accepted values are exactly `read` and `write`; reject duplicates, blank/unknown values, invalid JSON/YAML shapes, and caller-authored effective grants.
- Missing/empty requested or template operations grant none. Intersect operations and scopes through the existing resolution/issuance proof; clone the new field, bind it in the full request digest, and report narrowing using the existing provenance mechanism.
- When organizational Policy constraints are supported, absence means no additional cap; explicit empty means no grant. Use the accepted E14 evaluator rather than implementing a competing Policy engine.
- The first installed path supports at most one Memory scope per Work. The shared fields express operations applied to all granted scopes, not a per-scope matrix.
- A requested Memory capability that lacks either a nonempty effective operation set or scope set fails before allocation. Legacy `memoryScopes` alone never become implicit read/write.
- Logical scopes have operator-established Team/Project ownership. Team is read from the trusted issued principal and Project from the authorized issued action. Workers may nominate only a logical scope, operation, and bounded payload; they cannot author ownership or backend routing.
- Bind one trusted session to one claim and backend worker. A nominated target ID is a consistency check; unknown/foreign targets cannot select authority or receive victim-attributed facts.
- Capture the accepted route/ownership revision for each Work. Later configuration changes cannot reassign active Work's scope or expand its rights.

### Operations and Results

- Write takes one scope and a nonblank valid UTF-8 body, at most 8192 bytes. It appends a record and returns an opaque reference. Search takes one scope, a nonblank valid UTF-8 query of at most 512 bytes, and a bounded result limit. Body and query must not contain NUL (U+0000), including decoded JSON escapes; reject them with `Denied` / `memory-invalid-input` before Allow, provider attempt or backend access. Returned body text follows the same text rule. Other valid text, including line breaks, tabs, Unicode and literal backslash sequences, remains permitted within existing bounds.
- Search requires `read`; Write requires `write`. Search uses literal case-insensitive substring matching, treats `%`, `_`, SQL fragments, and backslashes as data, and orders by newest creation time with opaque ID as a stable tie-breaker.
- Default result limit is 3; permitted range is 1-10. Total encoded Memory reply is at most 32768 bytes, including its envelope. Omit excess complete records and return `truncated=true`; do not corrupt UTF-8 or silently truncate a record's content.
- Reuse the same maximum constants in worker decoding, host validation, adapter execution, and tests. Reject malformed or oversized calls before backend invocation.
- Use distinct results: `Written`, `Found`, `Empty`, `Denied`, `Unsupported`, `Unavailable`, `Timeout`, `Cancelled`, `Failed`, and `WriteUncertain`. These do not change the shared claim phases.
- A successful empty search is `Empty`, not Deny or backend failure. A failed/unconfigured backend never substitutes fake or in-process records.
- No general Get/Update/Delete/reset endpoint is introduced. Object references and prior invocation success do not convey authority.

### Lifecycle and Durable Writes

- Validate trusted binding, live call context, Running claim, issued operations/scopes, and ownership before dispatch. Record an Allow and an attempt before the external call; failed evidence preconditions prevent dispatch.
- The backend deadline is the minimum of five seconds, the call-context deadline, and the remaining Claim deadline.
- A post-terminal new call is denied with zero backend operations. Cancel already admitted calls on revocation and recheck before returning retrieved data; never publish fresh Memory content to an inactive worker.
- Cancellation cannot promise that a previously committed write was rolled back. If the backend cannot establish whether a commit completed, return `WriteUncertain` and do not retry with a new ID.
- Commit the Memory row and trusted invocation receipt in one database transaction. A duplicate of the same trusted invocation returns the stored result after current authorization checks. Different content under that ID fails without a second write.
- Deduplication applies to the same host-issued invocation, not repeated agent intent or a new independent Work. Worker-supplied IDs are not accepted as authoritative receipts.
- If a write commits but recording its outcome fails, fail Work with a stable evidence-error code; do not repeat the write. No cross-store atomicity guarantee is claimed.

### Evidence and Privacy

- Each resolved invocation carries RequestRef, the trusted Claim ID, invocation ID, Policy reference, operation, logical target scope, decision, and ordered attempt/outcome metadata. Denied calls have no backend attempt/outcome.
- Add `MemoryDecision` to the journal's invocation state checks. Memory metadata records result class, count, duration, truncation, and allowed logical references; it contains no body, query, SQL, connection string, or raw backend error.
- Unknown context cannot fabricate a claim fact. A mismatched target denial is attributed only to the established session's claim when that context is known.
- Memory-requesting Work means either requested scopes or requested operations are nonempty, including denied submissions. Its public view carries `contentRedactions` with `request.spec.task.input` and `outcome.text`.
- The projected task input is an empty object and free-text outcome is omitted. Preserve request reference, template/project, task type, requested/runtime fields, issued authority, and invocation metadata. Maintain the original canonical request privately for resolution and execution.
- A redacted request is an explicitly marked evidence projection, not a replayable submission or a replacement for the signed/issued input. Readers reject unknown redaction paths, nonempty hidden fields, or missing required redaction on Memory-requesting Work.
- All submission/query/list/detail response paths apply the same projection before serialization. UI displays a fixed content-withheld label; there is no reveal endpoint or Memory editor in this slice.
- Query/body data are private worker observations and may enter that Work's governed model context, but never acquire system-instruction or organizational Policy status. Avoid raw backend error or model-output echo in journal reasons and diagnostics.
- Normal non-Memory Work keeps its current request/outcome display behavior.

### Reference Acceptance

- Work A writes a fresh synthetic fact. Work B has a new claim and newly granted rights; its input excludes the expected value. An independent oracle checks a bounded computed result derived from the retrieved fact.
- Restart the Control Plane and database Pod independently while preserving storage. Work C receives new authority and retrieves the fact; old Work history is not required to survive.
- Unauthorized Team/Project/scope/operation and terminal/mismatched-context probes make zero Memory backend calls. Independently measure actual backend traffic rather than trusting only application decision text.
- A private test receipt may retain B/C's computed diagnostic for the independent oracle; default CLI/API/Portal evidence remains metadata-only.
- Protect database ingress from worker Pods at startup and steady state; use a successful Control Plane probe as a positive control. A declared policy with unenforced CNI fails the network acceptance gate.

## Negative Cases

- Cross-context claims, forged ownership/filter values, ambiguous scopes, unsupported capabilities, oversized payloads, invalid UTF-8, and unknown operations.
- Read-only Write, Search after expiry, route changes during active Work, and connection-pool reuse between different trusted scopes.
- SQL-like query text, duplicate invocation with different body, lost commit acknowledgement, and committed side effect followed by lost outcome evidence.
- Stored prompt-injection text trying to change authority, query/error echoes, missing redaction, and malformed public invocation ordering or cross-claim attribution.
- Database outage masquerading as empty/success, a missing selected integration environment being reported as pass, and startup-only worker network reachability.

## Compatibility

- Shared requests remain backend/provider-free; `memoryOperations` is additive, and unchanged non-Memory documents retain behavior.
- Existing scopes without operations remain non-executable; do not silently backfill read/write into old fixtures or registered templates.
- New fields need strict schema, fixtures, cloning, digest/provenance, generated UI contracts, journal and connected-client updates in the same reviewed slice.
- Memory-enabled evidence requires matching service/CLI/UI versions. Older strict readers may reject the additive fields explicitly; normal non-Memory views omit them.
- No `RuntimeBackend` operation, lifecycle phase, parent/child authority, or general Policy language changes.

## Open Decisions

- Product defaults are selected in the task/design. On 2026-10-08 the user authorized implementation under latest main's assignee-self-review workflow.
- The accepted E14 host credential contract must exist before credentialed live-backend acceptance. Its contract is owned by #155, not resolved by this specification.
