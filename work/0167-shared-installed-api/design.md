# Technical Design: Shared installed-service API for CLI Work query and local Portal parity

- Ticket: [#167](https://github.com/wunderforge/agenova/issues/167)
- Feature spec: `spec.md`

## Current State and Constraints

- The installed Pod serves ready/status on `:8080` and the existing canonical console API on in-Pod `127.0.0.1:8081`. Existing CLI run uses `kubectl exec ... /agenova-control-plane submit|evidence`, which calls that private API. The local Portal currently proxies to an unrelated `agenova-console` on port 8088. Installed `/api/setup` incorrectly reports hardcoded demo Policy/template. The installed service stores evidence in-process.

## Decision

Keep the API private in the Pod. Add a CLI `api connect` command that starts an explicit, loopback-only `kubectl port-forward deployment/agenova-control-plane 8088:8081` using the applied Platform's context and namespace. `platform status` prints the address as usable *after* connection, never as live API health. Local Vite `/api` proxy targets that loopback address; no Kubernetes credentials or provider secrets reach browser JavaScript. Extend the existing private command transport with bounded list/show so CLI run and queries call the same handler. Replace installed setup's hardcoded Policy/template with registry-backed data. Do not pretend the tunnel is authentication.

## Ownership and Contract Boundaries

- `console`: setup provider option; handler reuses canonical evidence/query/list; fail closed on registry errors.
- `registration`: read active Policy and selected registered template; no HTTP management mutation.
- `connectedclient`: bounded list/show via private API commands and strict evidence decoding.
- The installed service remains the sole owner of admission, runtime transitions,
  invocation decisions and recorded evidence. CLI validation is a read-side
  integrity boundary for the service response (schema, size, identity,
  chronology and cross-record correlation), not a second governance engine or
  a source of invented outcomes. A client rejection reports unavailable or
  invalid evidence; it never repairs or reclassifies the server's facts.
- `cli`/`cmd`: `work list|show`, `api connect`, Platform status connection metadata and explicit usage errors.
- `ui`: server-side loopback proxy configuration, connected-mode diagnostics and browser parity checks; no new governance state model.

## Alternatives Considered

- Publicly mounting `/api` on the ClusterIP listener is rejected: any in-cluster workload could call the fixed reference identity. A separate `agenova-console` is rejected because it creates another record source. Browser-side Kubernetes token/env configuration is rejected. Full production auth/durable store is separate scope.

## Verification Strategy

- HTTP unit tests for actual setup and failures; CLI tests for list/show, bounded refs and connection errors; port-forward command tests; Playwright against a real dev proxy and installed API; real kind/Ollama allow/deny with ID/evidence parity; `scripts/check.ps1 -All`; independent PR review.

## Risks and Compatibility

- Loopback is reachable by other trusted-machine processes and does not provide user authentication. A fixed local port may conflict and must fail clearly. Pod restart loses Work history and active run; disconnection must show unavailable, never fixtures. Keep this as a reference demonstration, not production deployment advice.
