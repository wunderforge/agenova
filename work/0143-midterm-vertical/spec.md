# Checkpoint behavior

One canonical request, one server-trusted principal, one issued claim and one application lifecycle owner. Readiness means Bound; actual Start acknowledgement precedes Running; outcome and cleanup are separate.

## Submission and identity

The internal server binds loopback by default. Request JSON is ClaimRequest only, not a principal or effective-authority envelope. A fixed Team A or Team B preset is operator startup configuration, not caller-granted identity; no browser identity switch is exposed. Exercise Team B through a separately configured server composition. Reject cross-origin submissions, oversized payloads, unsupported methods and invalid shapes. No general claim mutation or policy administration.

## Facts and view

Reuse existing decision, claim, authority and invocation types. Facts correlate requestRef, decision ID, claim ID and invocation ID; pre-claim denials never fabricate a claim. Facts append rather than update; provider attempt and completion are distinct from Allow permission. A defensively copied view includes request intent, current authoritative state, ordered facts and a task result. Missing data is explicit. CLI/API/UI consume the same view.

## Worker and model

The actual kind worker evaluates its objective through a governed model client and receives a real task-dependent response. Transport establishes the worker-to-claim binding before gateway eligibility/authority checks; bare IDs and deterministic control tokens are correlation, not authentication. Provider credentials stay behind host-side adapters. Static mock Tool calls may exercise Allow and Deny; they do not imply real repository/provider changes.

The chosen local-demo transport is an authenticated Kubernetes exec/stdio session pinned to the explicitly configured context/namespace/container and the actual system-allocated worker identity. Start acknowledges a live waiting process; RunService then publishes Running; WorkFunc opens the task interaction; the worker requests inference and receives its result. A worker-nominated different claim is rejected before gateway invocation facts or provider calls. Premature/terminal requests, malformed or late results, and identity mismatches have deterministic negative tests. Session cancellation/closure and final gateway state checks prevent continued provider use after terminal outcome.

## Sources and live proof

Demo mode uses only labeled fixtures. Connected mode uses actual API data, bounded polling and honest pending/empty/failure states. Synthetic provider replay is suitable for deterministic tests only. Release live evidence must include the actual worker, provider response/usage metadata, outcome and cleanup from the same request. Do not call a successful model request successful coding work without an observable result.
