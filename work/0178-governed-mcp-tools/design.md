# Technical Design: Installed governed MCP tools

- Ticket: [#178](https://github.com/wunderforge/agenova/issues/178)
- Feature spec: [spec.md](spec.md)
- Baseline: main `c56ba3a21a43ba550185e991cecede796af4f054`, inspected 2026-09-26. Independent packet review (2026-09-26, findings F1–F5) incorporated; Tom authorized Slice 1 and selected D1 B. Slice 1 contracts/composition are implemented; live transport and worker propagation remain pending. Tom selected D2 A on 2026-09-30.

## Current State and Constraints

The formal entry point is `cmd/agenova-control-plane`. It reads `/etc/agenova/effective-platform.json`, builds runtime/model providers and composes `internal/console.Service`. Its `validateInstalledAuthority` currently rejects every tool other than `git.read`. Platform services currently contain only model backends/profiles, and the installed setup response identifies tool capability as `mock`.

The existing Tool Gateway already validates Running authority, issues invocation IDs, records decisions and invokes a backend-neutral Adapter. The shared service always installs `mockReadAdapter`; the worker/runtime also hard-code the tool and file-shaped input. E16 adds the real backend and removes these special cases while retaining the governance path.

PR #189's `ToolProvider` is a useful example of a host-side call/result seam, but is console-owned, returns `workerprotocol.Reply`, is unmerged, and does not wire the installed control plane. Its attempt/outcome Target mismatch has an unresolved review finding. E16 must neither depend on that PR nor copy its current contract unchanged.

## Decision

Use the installed service and a neutral `internal/toolbackend` provider/catalog/result package. Implement one bounded Streamable HTTP MCP adapter behind it, configured through Platform and used only after Tool Gateway authorization and reliable pre-call recording.

```text
Platform toolBackends + toolProfiles -> resolved revision -> installed control plane
                                                     |
effective claim authority + configured tool catalog --+-> bounded worker catalog
                                                     |
worker operation -> bound claim -> Tool Gateway -> decision + ProviderAttempt
                                               -> toolbackend MCP adapter
                                               -> kind MCP Service -> real file read
                                               -> ProviderOutcome + resultRef
                                               -> untrusted bounded observation -> model
                                               -> shared CLI/API/Portal evidence
```

### Provider and catalog ownership

- `internal/toolbackend` owns logical operation descriptors, validated arguments/resource bindings, invocation inputs and bounded result metadata. It imports neither `console` nor worker-protocol types; the service converts neutral results to worker replies.
- `internal/toolgateway` owns the permission checks and existing invocation identity. Adapt the neutral provider through its current Adapter seam without creating a second policy engine.
- The MCP implementation owns MCP messages, session state, HTTP client, protocol negotiation, tool-name mapping and limits. SDK types cannot leak into shared contracts.
- `internal/adapters/bundled` registers and validates the tool capability. It must not import `console`. The installed command assembles the neutral catalog/provider and injects them into the shared service.
- Service/worker protocol changes carry only the allowed logical descriptors and resource/argument options. Never send endpoint, transport, MCP-native method names or credentials to the worker.

### Platform shape and validation

F1 selects **toolBackends + toolProfiles**, mirroring modelBackends/modelProfiles. Each profile defines one logical-operation/resource pair. Adapter config is entirely flat strings, with static lowercase/hyphenated schema paths; it does not require new ConfigSchema value kinds, wildcard paths or nested arbitrary operation/resource objects.

Slice 1 shape (implemented on the E16 branch; not accepted by baseline main):

```yaml
services:
  toolBackends:
    - name: reference-docs
      adapterRef: mcp-http
      config:
        transport: 'streamable-http'
        protocol-version: '2025-06-18'
        endpoint: 'http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp'
        timeout: '5s'
        max-request-bytes: '8192'
        max-response-bytes: '65536'
        max-observation-bytes: '4096'
        max-concurrent-calls: '4'
  toolProfiles:
    - name: reference-docs-read
      backendRef: reference-docs
      config:
        logical-operation: 'repo.read'
        resource-scope: 'repo:agenova/e16-fixture'
        mcp-tool: 'read_file'
        parameter-name: 'file'
        parameter-max-bytes: '128'
        parameter-allowed-values: |-
          README.md
          logs/timeout.log
```

`mcp-http` references a declared versioned adapter in `spec.adapters`; unchanged Platform sections are omitted. The first mapping supports one required string parameter. Its whitelist has the single configured parameter name and newline-delimited permitted values. Strip surrounding line whitespace, reject empty/duplicate entries and invalid paths, sort the values and emit one canonical newline-joined string. Do not encode nested JSON, arrays or dynamic field paths inside strings. Add another profile for another resource; the tuple (logical-operation, resource-scope) must route to exactly one backend/MCP mapping across the Platform. Profiles for the same operation must have a compatible public argument name/type; their allowed values are checked against the selected resource independently.

The backend onlyKeys whitelist is `transport`, `protocol-version`, `endpoint`, `timeout`, `max-request-bytes`, `max-response-bytes`, `max-observation-bytes`, `max-concurrent-calls`. The profile whitelist is `logical-operation`, `resource-scope`, `mcp-tool`, `parameter-name`, `parameter-max-bytes`, `parameter-allowed-values`. The optional N3 ValueInteger alternative was considered; Slice 1 retains the approved all-string public format. Every field is `ValueString` with an explicit non-secret default that passes its canonicalizer, so `adapters inspect` and `adapters init` can express the complete supported shape. Do not use camelCase adapter field paths: the current namePattern accepts lowercase names. Shared `services.toolBackends`/`toolProfiles` follow existing Platform field naming separately.

Require string-valued config even for numbers. Parse bounded decimal strings strictly (reject signs, fractions, exponents and overflow), then emit canonical decimal strings. Normalize timeout strings through the adapter. Reference ceilings remain timeout 1ms..30s, request 1..16 KiB, response 1..1 MiB, observation 1..16 KiB and no more than the response cap, concurrency 1..16. Parameter byte limits must fit the request/worker budgets (Slice 1 caps each value at 4096 bytes). The adapter supplies each logical tool's bounded trusted description (at most 256 bytes); it never comes from server discovery or a worker. The neutral catalog permits at most 32 routes, 64 allowed values per route and 16 KiB serialized descriptor data. Slice 2 must separately enforce the existing 8192-byte model-schema budget; a valid catalog does not imply every derived schema will fit. Defaults are configuration aids, never authority or permission to make a network call.

Validate an explicit HTTP(S) endpoint with a nonempty host and MCP path, no userinfo/query/fragment and no redirects. Cleartext HTTP is allowed only for the hardcoded fixture endpoint `http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp`: the bundled adapter requires `fixtureMCPHost`, port `8080` and path `/mcp` exactly. Other HTTP hosts, ports and paths are rejected, including other in-cluster Services. This exception does not permit arbitrary configured HTTP endpoints. Scope selects the fixed server dataset; allowlisted argument values cannot escape it. No runtime tools/list metadata adds permission.

### Registry and lifecycle migration

Flat strings preserve `validateSchema` and `validFieldPath`; **adding tool capability still needs an explicit migration**. Cover these changes in Slice 1, without broadening the registry's generic schema language:

| Surface | Required change and evidence |
| --- | --- |
| `api/v1alpha1/platform.go` | Add toolBackends/toolProfiles, shape/ref validation and duplicate checks; validate flat string config and backward-compatible old documents |
| `internal/platform/resolve.go` | Add tool capability and instance/profile resolution; include canonical tool config in revision and PlatformLock; changes to a mapping alter the revision, list order does not |
| `internal/adapterregistry/registry.go` | Permit tool in identityCapability/capability validation and require its profile canonicalizer; retain static scalar schemas and valid initialization defaults |
| `internal/adapterregistry/types.go` | Accept tool in AdapterLock capability validation and add tool arrays to FragmentServices; retain AdapterLock wire shape, identity/version semantics and old lock compatibility |
| `internal/adapterregistry/lifecycle.go` | Extend generated profile-name validation and Init's capability switch to emit a tool backend plus profile; run both canonicalizers, validate defaults, make no network calls |
| `internal/adapters/bundled/registry.go` | Register the tool manifest/factory with static ValueString schemas and strict onlyKeys canonicalizers; no console import |
| `cmd/agenova` / lifecycle tests | Exercise install idempotency, lock reload/round-trip, inspect schemas and init fragments; merge the fragment into a valid Platform and resolve it; preserve model/runtime/deployment behavior |

Slice 1 gates include registry/lifecycle/CLI tests for missing or wrong backendRef, duplicate operation/resource routes, incompatible same-operation argument shapes, non-string config, unknown keys, invalid defaults and canonical lock/revision stability. Tool adapters are a new supported capability; do not describe the new lock as readable by an older binary that explicitly rejects that capability. Existing locks remain readable by the new binary.

### Hard-coded tool migration map

All anchors are baseline locations, not immutable line numbers. The six requested areas include three checks inside the first file.

| Area | Baseline restriction | Planned replacement |
| --- | --- | --- |
| 1. `internal/workerprotocol/react.go` | ActionSchema line 26, ParseAction line 65 and prompt line 82 hard-code `git.read`/file names | Generate bounded schema/prompt and validate model actions from the same trusted filtered catalog; update every ParseAction call |
| 2. `internal/runtime/agentsandbox/execution.go:223` | Runtime accepts only `git.read`, one scope and one fixed input shape | Validate against the Work catalog, model-selected action and bound claim; preserve transport/message limits |
| 3. `internal/console/service.go:277` | Picks a scope only for `git.read`, using the first resource | Derive authorized operation/resource combinations; no first-scope shortcut |
| 4. `internal/console/service.go:295` | Rejects all other tool names; dispatch below it constructs `git/read` and `file` | Resolve logical operation and validated parameters through the trusted catalog, then invoke the existing Gateway |
| 5. `cmd/agenova-control-plane/main.go:252` | Installed authority validation admits only `git.read` | Validate installed tool capability/catalog and inject the selected real provider from effective Platform |
| 6. Composition and consumer propagation | Static ActionSchema at control-plane line 131, mock adapter at service line 265, worker Task/Operation/Reply and worker ParseAction call | Pass per-Work catalog and per-call model schema; map neutral provider results to bounded replies; retain explicitly labeled fixture mode |

Tom's six-location inventory is covered above; the preceding registry/lifecycle migration table is also mandatory Slice 1 work. Additional call-site scouting remains required. In particular, update `cmd/agenova-demo-worker/main.go`, service/runtime ParseAction callers, `internal/modelprovider`'s existing 8192-byte schema limit, the fixed three-file finish condition, setup capability labels, related fixtures and tests. A dynamic schema with an unchanged parser/dispatch is not complete.

### Invocation, results and correlation

Freeze the claim/catalog snapshot and reuse the reference service's correlation-only guard (`Service` per-Work operation handler, `internal/console/service.go:298` at `94a8481`: the operation claim ID must equal the per-Work claim ID). Check Running before Tool Gateway dispatch. This is a service/Gateway-side correlation check, not authentication of the caller or proof against hostile worker impersonation; #121 owns Worker-to-Gateway authentication. N3 injects context A/target B using a test-only driver and proves this check rejects before any MCP call. Then invoke Tool Gateway with the logical operation/resource for a matching context. After a successful decision append, record ProviderAttempt before entering the real adapter. Use one stable Target (for example the logical operation) for attempt and outcome. Scope remains independently validated; the target string is not authority.

Add an optional bounded public `resultRef` to ProviderOutcome and the shared projection/decoder/UI contract. For the fixture it identifies the known logical artifact, not the private endpoint or response body. Preserve the equality check in `internal/connectedclient/run.go` (baseline line 515); add a regression that a differing external resultRef is accepted while a differing Target is rejected. Inspect all fact/TypeScript schema consumers rather than weakening the CLI validator.

Keep wire and observation limits distinct. Use a byte-limited HTTP body/SSE reader before JSON decoding; cap cumulative frames and decoded output, not only Content-Length. Hard overflow is failure. Valid text above the smaller observation limit is UTF-8-safe truncated and carries untrusted/truncated metadata and a visible notice. MCP-returned instructions cannot mutate the catalog, grant, endpoint or system prompt. Finish conditions must use actual accepted tool evidence and task needs, not a fixed count of three files.

Cancel on deadline/claim termination and reject new terminal attempts. Recheck authority before handing a request to the provider; an already dispatched call can remain visible after cancellation and must not be reclassified as zero calls. Do not automatically replay tools/call on an ambiguous disconnect or session error.

### Transport and real fixture

The first interoperability target is MCP protocol `2025-06-18` over Streamable HTTP, an explicit supported subset rather than a claim of latest-spec coverage. Initialization negotiates that version; subsequent requests include the negotiated version and session header when issued. Support JSON and bounded SSE call responses. Test initialized notification/202 handling, request-ID matching, cancellation, unsupported version, session expiry and error responses. No legacy-transport fallback or unimplemented server-initiated feature is advertised. A client-library choice does not relax these behaviors.

The real fixture is an independently running, read-only MCP Deployment+ClusterIP Service in namespace `agenova-e16`, with mounted reference files and JSON request logs. D2 selected Option A (see [D2](#d2-fixture-server-option-a-selected)); lock its reviewed source/version and image digest for acceptance; record its separate server toolchain and keep it out of the Agenova root module. A dedicated reference helper is acceptable only if it implements actual MCP transport and reads real mounted files; no handler that merely replays expected agent answers. This packet does not invent an available upstream image/version.

Capture server `tools/call` receipts before handler dispatch. Propagate a host-issued non-secret invocation correlation value through an adapter-controlled request header and bind it to the MCP request ID in server logs; workers cannot supply it. Run sequential positive controls around the negative suite and collect all server pod logs and identities. Absence of an application call is proven from continuous server logs plus the driver's rejected-attempt receipt, not from a client spy alone. Initialization/health traffic is separate; denied invocations never initiate their own provider connection.

### Slice 1 delivery boundary

`platform.Descriptor.DescribeTool` projects adapter-owned configuration into a neutral logical route; shared resolution does not inspect MCP keys. The resolved Platform and PlatformLock bind an additive `toolRoutes` projection into their revision, while the AdapterLock wire structure stays unchanged. `toolRoutes` is omitted when empty, preserving legacy revision payloads. The installed builder recomputes the projection and rejects configuration/catalog mismatches.

The registered MCP factory validates configuration and returns explicit `ErrUnavailable` until Slice 2 implements the transport. Installed capability is `notConnected`; tool-bearing Work with configured tool backends fails with `tool_transport_unavailable` before journaling or runtime allocation. This gate remains until the transport and dynamic worker consumers are ready together. Model-only Work and the explicitly selected legacy synthetic fixture retain their existing behavior.

The shared service accepts a neutral `toolbackend.Set` for deterministic composition tests. Its consumer adapter records decision -> attempt -> call -> outcome, keeps Target stable, sanitizes errors and labels/truncates observations. `ResultRef` is a separate neutral result field; journal/CLI/Portal projection is still Slice 2. Tests using injected providers are not live MCP evidence. Slice 2 S2 must move argument/session prechecks before Gateway dispatch and complete any post-Allow rejection with an attempt/Failed outcome pair when recording is available; the current early-return path can leave stage 1 and invalidate the Work in the CLI. Slice 2 S3 must enforce the configured concurrency limit in `toolbackend.Set`; Slice 1 only validates that setting. See [the Slice 1 review packet](review-slice1.md).

### Slice 2 delivery boundary

The bundled factory now returns the bounded client, and the Slice 1 `tool_transport_unavailable` admission gate is removed. Installed setup reports tool capability as `configured`, which describes installed configuration, not a successful call. The worker `Task` carries the catalog intersected with the claim's effective tools and resource scopes; the output schema, parser, prompt, runtime check and service all use it, and `git.read` remains only as the explicitly labelled synthetic fixture when no backend is configured. The service validates the installed route and argument before the Gateway; authority stays with the Gateway. `ResultRef` is projected as an optional `resultRef` on successful tool outcomes and validated by the journal, CLI and Portal. `toolbackend.Set` enforces `max-concurrent-calls` per backend.

## D1: MCP client Option B selected

Tom selected **Option B** on 2026-09-26: a bounded Go 1.22 standard-library Streamable HTTP client inside E16. The repository declares `go 1.22`; CI uses `go-version-file: go.mod`. The local Go 1.27.1 executable is not evidence of Go 1.22 compatibility. No dependency or toolchain file is changed by this packet.

Primary manifests checked on 2026-09-26:

| Candidate | Declared minimum Go | Implication |
| --- | --- | --- |
| Official `modelcontextprotocol/go-sdk` v1.8.0 (latest release observed) | `go 1.25.0` | Cannot be adopted while honestly retaining a Go 1.22 build baseline |
| Official SDK v1.0.0 | `go 1.23.0` | Pinning this older stable version still does not meet Go 1.22 |
| Official SDK v0.1.0 | `go 1.23.0` | The inspected early tag is not a Go 1.22 workaround either |
| Bounded client using Go 1.22 standard library | Designed to support Go 1.22; unimplemented/unverified | Retains baseline, but Agenova must own protocol/session/SSE/error handling and interoperability tests |

Sources: [v1.8.0 go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/go.mod), [v1.0.0 go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.0.0/go.mod), [v0.1.0 go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v0.1.0/go.mod), [release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0), [Streamable HTTP specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports), [lifecycle](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle). These sampled manifests establish incompatibility for the named tags, not a claim that every historical commit was audited.

**Option A: official SDK, not selected.** A root-module SDK adoption that raises the minimum Go baseline first requires **wunderforge's approval of an independent chore Ticket and PR**. That chore covers `go.mod`, `deploy/reference/Dockerfile`, `harness/integration/agentsandbox/testworker/Dockerfile` (both currently `golang:1.22-alpine`), CI and matching contributor prerequisites. CI currently follows go.mod; verify the approved minimum explicitly rather than relying on automatic toolchain download. Inspect transitive requirements and pin versions after approval. This repository-wide chore is not part of E16, and its later approval would not automatically reverse D1 B. A separate unpublished maintainer ticket draft is kept outside this repository; it is not part of E16.

**Option B: minimal client, selected and within E16.** Retain root `go 1.22` and implement only the documented Streamable HTTP subset with standard-library HTTP/JSON/SSE parsing. Own lifecycle, limits, sessions, cancellation and malformed-response tests against httptest and the independent real server. This is more than one HTTP POST, but adds no root SDK dependency and does not require the baseline chore. Run the actual minimum compiler matrix with `GOTOOLCHAIN=local` to expose incompatibility.

The fixture server is a separate process and may use the official SDK in an **independent Go module** and its own pinned builder image. Do not add it to a root go.work, use a root replace directive, import its SDK into Agenova, or let its build upgrade the root module. Its version/toolchain and tests are separate evidence. D1 is resolved; D2 below selects this fixture shape.

## D2: Fixture server Option A selected

Tom selected **Option A** on 2026-09-30: an independent Go module that uses the official SDK to serve one read-only file tool. Both candidates had to expose Streamable HTTP on kind, read real mounted files, use pinned source/image versions and retain complete per-call logs. The comparison below is kept as the decision record.

| Candidate | Work and likely effort | Benefits and unresolved work |
| --- | --- | --- |
| A. Independent Go module + official SDK read-only file server | **Medium, bounded:** one isolated module/go.sum and pinned compiler/SDK; one read_file handler with path/value limits; request/handler logging middleware; a server Dockerfile, Deployment/Service/read-only data mount; protocol/file/timeout/oversize fixtures and tests | Direct control of behavior and correlation logs, without a custom server-side MCP stack. Must pin SDK/image, keep the module isolated, and implement actual file reads plus fail cases; not merely an httptest fake |
| B. Existing Streamable HTTP read-only server + body-logging reverse proxy | **Medium to high / less predictable:** vet and pin server/image/license and read-only scope; configure/mount data; add a proxy container/config, request-body capture/redaction/correlation, Service routing and log collection; verify SSE/session/body forwarding and failure injection | Less tool-handler code only if a suitable server exists. No compliant server/image is yet vetted in this packet; selection/transport/logging gaps can exceed option A's work |

These are relative engineering estimates, not delivery dates.

**Why A.** The Agenova client is our own standard-library implementation (D1 B); the fixture's protocol stack is the official SDK. Interoperability is therefore tested between two independent implementations, not between two halves written together. A also puts the pre-dispatch receipt and correlation logging in ordinary HTTP middleware we control, and lets the fixture own its timeout and oversize cases.

**Why not B.** The obvious candidate, the official `@modelcontextprotocol/server-filesystem`, connects only `StdioServerTransport` (`src/filesystem/index.ts` on the `modelcontextprotocol/servers` main branch, checked 2026-09-30). Using it would need a stdio-to-HTTP bridge plus the body-logging proxy, so three components to pin and vet. That is the extra transport work this section already excluded from B, and proxy receipts would still only prove that a request reached the proxy.

**Risk.** A reviewer may ask whether a fixture we built counts as a real MCP server. The answer rests on two rules below: the tool handler performs real reads of mounted files and never returns scripted answers, and its logs come from a separate pod collected with `kubectl logs`, not through Agenova.

### D2 closure record

| Item | Decision |
| --- | --- |
| Source and module | `harness/integration/mcpfixture/` with its own `go.mod`/`go.sum`, following the `testworker` precedent of a separately built integration image. Not in a root `go.work`, no root `replace`, and nothing in the root module imports it. |
| SDK and toolchain | `github.com/modelcontextprotocol/go-sdk` v1.8.0 (latest release on 2026-09-30, published 2026-09-14; its `go.mod` declares `go 1.25.0`). The fixture module declares the Go version the SDK requires. Before building on it, Slice 2 tests that this SDK version accepts a `2025-06-18` initialize from our client; if it does not, pin a version that does and record the change here. |
| Builder and image | Multi-stage Dockerfile: `golang:1.27-alpine` (Go 1.27.1; Go 1.25 is out of upstream support, and the module's minimum stays `go 1.25.0`) and `gcr.io/distroless/static-debian12:nonroot`, both pinned by digest in the Dockerfile. Load the built image into kind with `kind load docker-image`. Record the built image digest in the acceptance evidence; do not write it into planning documents beforehand. |
| Tool surface | One tool, `read_file`, with one required string argument `file`, matching the Slice 1 profile (`mcp-tool: 'read_file'`, `parameter-name: 'file'`). Open files through `os.Root` (Go 1.24+) on the data mount, so `..` and symlinks that escape the root are refused while the ConfigMap's internal `..data` symlinks still resolve. Enforce a request body limit and a file size limit; return MCP tool errors for missing, oversized or refused paths. No other tools, prompts or resources. |
| Deployment | Namespace `agenova-e16`; Deployment `e16-mcp` with **exactly one replica**, so one pod's log is the complete call record; Service `e16-mcp` (ClusterIP, port 8080), path `/mcp`. This matches the Slice 1 hardcoded fixture endpoint `http://e16-mcp.agenova-e16.svc.cluster.local:8080/mcp`. Non-root, read-only root filesystem, no service account token, data mounted read-only. Manifests (`deploy.yaml` plus a `kustomization.yaml` that generates the data ConfigMap) live with the fixture module, not in the reference `platform apply` install. |
| Dataset | Real text files under the fixture's `data/`, mounted from a ConfigMap as resource scope `repo:agenova/e16-fixture`. It includes the Slice 1 example values (`README.md`, `logs/timeout.log`), one file larger than `max-observation-bytes` (4096) but under `max-response-bytes` (65536) for truncation evidence, and one file over 65536 bytes for oversize evidence. Keep the ConfigMap under its 1 MiB limit. |
| Call log | JSON lines on stdout. HTTP middleware reads the bounded body and writes a `receipt` line **before** handing the request to the SDK handler: timestamp, pod name (from the Downward API), correlation header value, a hash prefix of the session ID (not the raw ID), JSON-RPC method, id, tool name and the allowlisted `file` argument. It also logs unparseable or oversized bodies. After the handler runs it writes a `result` line with the same correlation and id, byte count, error flag and duration. Never log file contents, authorization headers or raw session IDs. `initialize`, notifications and health traffic are logged under their own method, so they are separate from `tools/call`. |
| Correlation | The client sends the Gateway-issued invocation ID in `X-Agenova-Correlation` on every request of the call (initialize, initialized, tools/call, DELETE). The fixture logs the same header. Workers never set it. |
| Timeout control | A fixture-only startup setting marks one dataset path as slow and sets its delay above the backend `timeout`. Agenova gets no public fault-injection flag. |
| Build and test | `GOTOOLCHAIN=local go -C harness/integration/mcpfixture test ./...` with a local Go ≥ 1.25, plus `docker build harness/integration/mcpfixture`, recorded as separate evidence. Root `go vet ./...` and `go test ./...` do not enter a nested module, and the root CI job uses Go 1.22, so the fixture is not built in CI. Adding a CI job for it is infrastructure work and needs its own approval. The root fast gate still runs `gofmt -l` and the SPDX header check on every `.go` file, so fixture sources must carry the Apache-2.0 SPDX header and parse under the CI `gofmt`. |

## Ownership and Contract Boundaries

- E16 owns Tool Backend configuration, logical mapping, actual MCP transport/results, service wiring, catalog propagation and tool evidence.
- #121 owns Worker-to-Gateway caller authentication. E16 reuses correlation-only reference binding and limits N3 to a test-only Gateway-side rejection probe.
- E14 owns trusted identities and shared credential resolution. E16 consumes the agreed server-side token boundary for its final acceptance slice; it does not require complete SSO before the no-token slice.
- The existing shared service remains an assembly consumer; provider/bundled code cannot import it. Add an import-boundary check to the implementation gate, including transitive provider dependencies.
- PR #189 remains optional reference material. Coordinate future overlapping service/worker changes, but do not cherry-pick its role policy or local console entry point into E16 by default.

## Alternatives Considered

- Reuse #189's console-owned ToolProvider verbatim: rejected because it couples the reusable provider contract to the demo consumer and does not wire the installed service.
- Public anonymous MCP endpoint: not selected for acceptance because fixed version, dataset, logs and failure injection must be under test control.
- Stdio or legacy SSE transport: outside Tom's selected transport and deployment shape.
- Generate tools from grant strings alone: insufficient without configured descriptors, argument/resource mappings and independent Gateway checks.
- Allow arbitrary MCP metadata to add tools: rejected; remote discovery cannot grant authority.

## Verification Strategy

1. Deterministic `httptest` servers cover real transport serialization/parsing and count every request; separate governance tests prove each pre-call rejection and decision/attempt recording failure. Include malformed/SSE/oversize/timeout/session cases without internet or a model.
2. Composition tests construct the installed builder with a fake MCP endpoint and registered policy/template. A non-`git.read` operation must work; a console-only injection cannot satisfy this test. Verify catalog copying, all six migration areas, configured-failure behavior and old explicit fixture labeling.
3. One opt-in kind acceptance campaign runs the actual installed control plane, real controlled worker, real model and independently deployed MCP server. Run one successful task and the five separate rejection categories, then timeout/oversize/truncation evidence. Collect CLI/API/Portal parity and screenshots.
4. Use a bounded test-only invocation driver around the same installed builder for cross-claim/terminal/failing-journal probes that ordinary submission cannot express. N3 uses the existing correlation guard only; it does not certify caller authentication or hostile-worker isolation (#121). Do not add public fault-injection flags. Keep this probe evidence distinct from the production-binary positive E2E; record exact artifacts and demonstrate real-server zero calls for each probe.
5. Run the focused commands in task.md and the full repository gate, including the registry/lifecycle/CLI matrix above. Under selected Option B, execute an actual Go 1.22 build/test matrix; compile/test any D2 SDK module separately. Never infer baseline compatibility from a newer local compiler.

## Risks and Compatibility

- Catalog propagation crosses protocol, runtime, service and installed validation. Land descriptor fixtures and contract tests before consumer changes are called complete.
- MCP data is adversarial input; labeling is not a guarantee against model persuasion. Continued server-side enforcement must block any attempted expansion.
- Facts can fail after a real call. Distinguish pre-call zero-call guarantees from post-call evidence loss and preserve independent server records.
- The reference fixture has no production identity/network-isolation claim; E14 token acceptance remains required for Epic closure.
- Server/client versions and image digest are delivery outputs to pin and record, not fabricated planning facts. D1 B and D2 A are selected; the fixture image digest and real evidence remain pending. The Go-baseline chore is separate and requires maintainer approval.
