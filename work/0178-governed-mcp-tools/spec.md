# Feature Specification: Installed governed MCP tools

- Ticket: [#178](https://github.com/wunderforge/agenova/issues/178)
- PRD outcome: [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [accountability](../../docs/product/prd.md#5-facts-and-accountability) and [installed service](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).
- Status: independent Claude review F1–F5 incorporated; D1 B selected (Go 1.22 minimal client). Slice 1 contract/composition implementation is available for review; live transport and worker propagation remain Slice 2. D2 fixture selection is required before Slice 2.

## Intent

A Work can use an operator-configured logical tool through the installed service. Permission is checked independently of model output, MCP metadata and tool responses. A real observation returns to the agent without changing authority, and independent server records corroborate platform evidence.

## In Scope

- One bounded read-only file operation on a reproducible dataset served by a real MCP server on kind.
- Streamable HTTP, validated Platform configuration, trusted catalog delivery, per-invocation enforcement and shared evidence.
- Credential-free first integration and an E14-coordinated token path before full Epic completion.

## Out of Scope

- Arbitrary server discovery, model-selected MCP endpoints/methods, write operations, production multi-user identity, Worker caller authentication (#121), a general MCP proxy or legacy transport fallback.
- Defining prompts/reasoning as a core RuntimeBackend or authority contract; worker prompt adaptation stays at the agent composition edge.

## Requirements

### R1. Installed configuration

`Platform.spec.services.toolBackends` and `services.toolProfiles` mirror the existing model backend/profile structure. A backend is a named `PlatformInstance` with adapterRef; a profile is a named `PlatformProfile` with backendRef. **One profile defines exactly one (logical operation, resource scope) pair** and its MCP tool/argument mapping. Multiple resources for the same operation require multiple profiles, not nested maps keyed by operations/resources.

Backend config contains flat string values for transport, endpoint, protocol version, timeout and request/response/observation/concurrency bounds. Profile config contains flat string values for logical operation, resource scope, MCP tool and the first slice's single string parameter name, byte limit and allowed values. A newline-delimited string carries that bounded parameter-value allowlist; arrays, nested objects, JSON-in-a-string maps and arbitrary dynamic field paths are rejected. Numeric limits remain quoted strings in public config and are validated/canonicalized by the adapter. Static lowercase/hyphenated field names satisfy the existing `validFieldPath` grammar, and every field has a corresponding ValueString schema entry and valid non-secret initialization default.

Reject unknown keys/non-string values, duplicate backend/profile names, duplicate (logical operation, resource scope) routes even across backends, absent backendRef, wrong adapter capability, invalid endpoints/durations/limits, empty/duplicate parameter values and out-of-scope argument values. The same logical operation may appear in different resource profiles only with a compatible public parameter shape; resource-specific value allowlists remain separate. No raw credential or endpoint secret may enter config/status.

Add tool capability to registry validation, profile requirements, AdapterLock normalization and the install/inspect/init lifecycle; init emits both toolBackends and toolProfiles with canonical defaults. Preserve the existing scalar ConfigSchema kinds, static-path rules, onlyKeys behavior and lock wire shape. The Platform resolver includes tool instances/profiles in canonical revisions and locks, and the installed control plane consumes them. Existing model/runtime/deployment adapters and locks must round-trip unchanged. Configuration validation and `adapters init` perform no network calls.

The cleartext HTTP exception is limited to the hardcoded fixture endpoint `http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp`, with exact host, port and path validation. Other HTTP endpoints, including other in-cluster Services, are rejected; no arbitrary redirect or worker-supplied URL is followed. HTTPS endpoints, if supported, retain normal certificate validation. No endpoint-derived authority is inferred.

### R2. Trusted tool catalog

Build each Work's advertised catalog from **configured logical operation descriptors intersected with effective tool grants and matching resource scopes**. Include bounded trusted descriptions, allowed arguments and authorized logical resource choices. Deep-copy/freeze that snapshot; changing Platform/template inputs cannot silently widen an existing claim.

Schema, ParseAction, prompt, worker payload, runtime validation and dispatch use this catalog. A grant to an uninstalled tool still produces an explicit installed-capability error; intersection must not silently convert an unsupported granted capability into successful execution. Empty tool access is valid for a model-only Work.

The first live fixture uses `repo.read` mapped to MCP `read_file`, so acceptance cannot pass solely through the old `git.read` special case. These are fixture choices, not reserved product names. Logical resource selection and parameter values must resolve to a configured resource; a permitted parameter name alone cannot authorize arbitrary paths, URLs or repositories.

### R3. Invocation and transport

For every attempt, reuse the existing **correlation-only** per-Work context and claim-ID equality guard in `internal/console/service.go:288`; do not add or claim caller authentication in E16. Require Running, check operation/resource/arguments against authoritative state, then record the decision and ProviderAttempt before any invocation-specific MCP request. Fail closed at every pre-call step. The Gateway remains the permission owner even if the model's schema omits unauthorized tools. A claim ID is correlation, not proof of the caller's identity. Worker-to-Gateway authentication belongs to [#121](https://github.com/wunderforge/agenova/issues/121).

Use Streamable HTTP initialization, negotiated protocol version, initialized notification and bounded `tools/call`. Handle JSON and SSE responses, request IDs, sessions and cancellation according to the pinned supported protocol subset. Reject unexpected/malformed responses and tool errors explicitly. Do not advertise sampling, roots, elicitation or other server-initiated capabilities that are not implemented. No fallback to stdio, legacy transport or mock content.

### R4. Results and evidence

For one invocation, ProviderAttempt and ProviderOutcome have identical `Target`, operation and correlation IDs. Choose a stable logical target before the call. A bounded safe external reference belongs in a **separate optional `resultRef` field**, never in Target. Validate and project this additive field consistently across the fact journal, evidence API, CLI decoder and Portal. Endpoint URLs, tokens and raw private content are not result references.

Record decision, attempted/succeeded/failed provider outcome and Work outcome separately. An Allow decision is not a successful provider call. Failed outcome recording after an actual call yields an explicit evidence failure; do not assert that the tool was not invoked or retry it silently.

MCP text is untrusted observation data, delivered in a delimited/labeled observation with `untrusted=true`. Bound the wire body before decoding, including SSE framing/accumulation. Exceeding `max-response-bytes` fails the operation with `tool-response-too-large`; never parse a truncated JSON/SSE message as success. For a valid response within that limit, truncate text to `max-observation-bytes` on a UTF-8 boundary and mark `truncated=true` with a visible notice. Escape it in the UI and never promote it to a system prompt, tool configuration, Policy or authority. Raw observation text is not added to evidence by default.

### R5. Real server and reproducibility

Deploy a fixed-version read-only MCP server as a kind Deployment+ClusterIP Service in a dedicated namespace. Pin the reviewed source/version and final image digest; prohibit mutable `latest` and unresolved image placeholders in accepted evidence. Mount the reference files read-only. The server must execute a real file read, not return a client-side canned observation.

Server logs record each received `tools/call` before handler dispatch, its correlation ID, tool name and result status, with no file body/token. Capture the complete log window and pod identity/restart count. Log gaps, restarts without retained logs or unmatched correlation fail the zero-call check. Initialization/health messages are logged separately and never counted as tool executions; rejected attempts must not initiate their own downstream requests. Positive control calls before/after the denial suite prove the log collector remained live.

### R6. Token integration

The first server may run without credentials. Full Epic completion requires an E14-agreed token-required server path: resolve credentials on the trusted provider side, test invalid/missing token failure, and prove Worker/request/evidence contain no external secret. Credentials are not copied into Platform config or the tool catalog. Do not independently redefine E14's principal or credential contract.

## Negative Cases

All cases below have deterministic `httptest`/composition coverage. The five zero-call categories must also be witnessed against the real kind server as part of the E2E campaign; merely reporting them from unit tests is insufficient.

| ID | Trigger | Required platform result | Server proof |
| --- | --- | --- | --- |
| N1 | Known configured operation requested without the claim's tool grant (ordinary Deny) | Explicit rejection at the trusted boundary; no ProviderAttempt | Zero correlated `tools/call` entries |
| N2 | Granted operation with another/ungranted resource, or argument selecting an out-of-scope file | Resource/argument rejection, no expansion of authority | Zero calls; name allowlist alone is not enough |
| N3 | Test-only driver binds reference context A, then submits an operation nominating independent Running claim B through the existing service/Gateway-side guard | Correlation mismatch; never attribute an invocation to B. Proves Gateway-side cross-claim rejection only, not authenticated caller isolation (#121) | Zero calls for the injected attempt; retain the driver receipt and complete server logs |
| N4 | Invocation after claim success, failure or expiry | Terminal authority rejection | Zero calls after the witnessed terminal boundary |
| N5 | Decision append or ProviderAttempt append fails before the provider | Explicit recording failure; no downstream invocation | Zero calls; test-driver receipt records failure when the journal cannot |
| N6 | Server stalls beyond the configured deadline | Bounded timeout, cancellation, no mock substitution | One received call may exist; no automatic replay |
| N7 | Wire response exceeds the hard byte cap, including SSE/chunked content | `tool-response-too-large`, no successful observation | Actual call recorded; no fake successful result |
| N8 | Valid wire reply has text beyond the smaller observation cap | Labeled untrusted, truncated observation; task result must remain honest about incomplete data | Real call, bounded text returned to worker |
| N9 | Invalid endpoint/config/mapping or unsupported installed tool | Configuration/capability error before execution | Zero invocation traffic |
| N10 | Connection failure, malformed protocol reply, `isError`, unsupported version or content | Distinct safe failure, no mock fallback or invented success | Retain the actual observed request count |
| N11 | Provider succeeded but outcome append fails | Evidence failure, no silent replay or zero-call claim | Real call remains visible independently |
| N12 | MCP text asks the model to change policy or invoke an unconfigured endpoint/tool | Text remains untrusted data; any forged action is independently rejected | No unauthorized follow-up call |

The adversarial driver must exercise the installed composition's existing correlation-only guard, not a second permissive endpoint. N3 is deliberately a test-only injected cross-claim operation; it does not claim a hostile worker was authenticated or that a caller could not forge the reference correlation value. Caller authentication is deferred to #121. SDK-independent test seams may inject a journal failure or a post-terminal invocation through a test-only composition of the same installed builder. Keep this separate from the successful run on the unmodified production binary, document the test artifact SHA and never expose fault-injection controls on the normal Work API.

## Compatibility

- Keep existing ClaimRequest/authority and RuntimeBackend meanings. MCP-specific configuration stays in the adapter.
- Existing explicit synthetic demo fixtures remain labeled synthetic. Absence of a real backend can preserve the clearly identified legacy reference fixture; a configured real backend that fails must never select that fixture.
- Existing recorded evidence without `resultRef` remains readable; unequal attempt/outcome Target remains invalid. Extend all shared decoders together, including their unknown-field rules.
- The installed API retains its current reference-identity limitations until E14 delivers its separate contract. E16 cannot claim production authentication from this fixture.
- D1 B keeps root `go.mod` at `go 1.22` with no MCP SDK dependency. A root Go upgrade is a separate maintainer-approved chore covering both Go builder Dockerfiles and CI. An independent fixture-server module may use the official SDK and its own pinned compiler without importing it into the root module.

## Open Decisions

- D1 is resolved: [Option B](design.md#d1-mcp-client-option-b-selected), minimal Go 1.22 standard-library client, selected by Tom on 2026-09-26.
- [D2](design.md#d2-fixture-server-selection-before-slice-2) remains open: independent-module SDK file server or vetted existing server plus a request-body logging reverse proxy. Select before Slice 2; source/version/image lock is required before real acceptance. No image digest or implementation is claimed yet.
