# E16 Slice 1 review packet

## Branch and source state

- Branch: `codex/e16-slice1`.
- Base: `c56ba3a21a43ba550185e991cecede796af4f054` (the approved main baseline).
- Review target: the signed Slice 1 Draft PR head against this base. The initial code review covered uncommitted implementation/tests and packet files; the publication commit includes them and the S1–S4 follow-up. The 21 reviewed Go files are identified by the source hash manifest below. Local diagnostic logs under ignored `.tmp/e16-slice1/` are excluded from the commit.
- This is Slice 1 contract/configuration/composition work for #178. It does not close the Epic or demonstrate live MCP execution.

## Changes mapped to Todo

| Todo | Files | Delivered behavior |
| --- | --- | --- |
| Pre-work N1/N2/N3 | `task.md`, `spec.md`, `design.md` in this packet | Exact external chore draft path; independent Claude F1–F5 review and Tom self-review on 2026-09-26; retain the approved flat ValueString format after considering optional ValueInteger |
| Slice 1: neutral provider/catalog/result | `internal/toolbackend/catalog.go`, `provider.go`, `catalog_test.go` | Immutable bounded logical catalog, per-resource argument allowlist, authority intersection, neutral provider/factory/result and safe typed failures; no console or worker import |
| Slice 1: Platform configuration/revision | `api/v1alpha1/platform.go`, `internal/platform/resolve.go` | Add toolBackends/toolProfiles and structural/reference checks; adapter callback projects logical descriptors; bind tool config and neutral toolRoutes into revision/PlatformLock, omit new route field for old documents |
| Slice 1: registry lifecycle | `internal/adapterregistry/registry.go`, `types.go`, `lifecycle.go` | Tool identity/capability/profile validation, AdapterLock support, install/inspect/init and generated-name limits; existing scalar schema grammar unchanged |
| Slice 1: bundled configuration | `internal/adapters/bundled/registry.go`, `mcp.go`, `mcp_test.go` | Strict flat strings, fixed Streamable HTTP version, bounded endpoint/time/byte/concurrency settings and one operation/resource mapping; canonical values, invalid/ambiguous routes rejected; no transport yet |
| Slice 1: installed builder and admission | `cmd/agenova-control-plane/main.go`, `main_test.go`, `tools.go`, `tools_test.go` | Construct via registry factory, recompute/verify applied catalog, validate installed grants, inject provider into service; configured tool-bearing Work explicitly unavailable before worker setup until Slice 2 |
| Slice 1: governed composition | `internal/console/service.go`, `tool_provider.go`, `tool_provider_test.go`, `internal/workerprotocol/protocol.go` | Inject neutral provider behind existing Gateway; decision/attempt precede call; stable Target; sanitized failure; bounded untrusted/truncated replies; no mock fallback. Existing correlation check retained |
| Review delivery | `work/0178-governed-mcp-tools/review-slice1.md`, `docs/evidence/178/*` | Branch/base/source state, exact commands, logs, source hashes and explicit remaining work |
| Slice 1: CLI delivery checks | `cmd/agenova/smoke_test.go`, `internal/cli/adapters_test.go` | New capability visible in catalog, idempotent persisted install, inspect and backend+profile init; update expected bundled catalog without weakening existing adapter checks |

## Verification environment and commands

Working directory: `/Users/tomtianys/Workspace/Projects/P3-Agenova`.

Verified compiler: `go version go1.22.12 darwin/arm64`. Node: `v24.21.0`.

Use this environment for the successful minimum-version checks (including child CLI builds):

```sh
export PATH="/Users/tomtianys/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.22.12.darwin-arm64/bin:/opt/homebrew/opt/node@24/bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=1
export GOFLAGS='-ldflags=-linkmode=external'
```

Default linking with the same Go 1.22.12 compiler failed on this host: dyld aborted several test executables with `missing LC_UUID load command`. External linking resolved that observed execution failure; no root Go baseline, SDK dependency, Dockerfile or CI workflow was changed. This is macOS-host evidence, not Linux-container or kind evidence.

Focused command:

```sh
go test ./api/v1alpha1 ./internal/platform ./internal/adapterregistry ./internal/adapters/bundled ./internal/cli ./internal/toolbackend/... ./cmd/agenova ./cmd/agenova-control-plane ./internal/toolgateway ./internal/console ./internal/workerprotocol ./internal/runtime/agentsandbox ./cmd/agenova-demo-worker ./internal/modelprovider ./internal/facts ./internal/connectedclient
```

Repository gate:

```sh
pwsh -NoProfile -File ./scripts/check.ps1 -All
```

Final results on 2026-09-26:

| Gate | Exit / result | Evidence / scope |
| --- | --- | --- |
| Go 1.22.12 focused command above | 0; all 16 packages pass | [Focused output](../../docs/evidence/178/slice1-focused.log) |
| `pwsh -NoProfile -File ./scripts/check.ps1 -All` | 1; sole remaining failure is the unchanged keyboard browser case | Local-only `.tmp/e16-slice1/slice1-full-gate.log`; excluded from Git |
| Full-gate Go portion | pass: formatting, module tidy, `go vet ./...`, `go test -count=1 ./...`, both integration compile gates | Same local-only full log |
| Full-gate frontend portion | pass: contract checks (22 checks), typecheck, 124 unit tests, production build; browser smoke 51/52 | Same local-only full log |
| `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` | 0 after final packet/log updates | Rechecked after S1–S4; local `.tmp/e16-slice1/slice1-docs.log` |
| Original `git diff --check` plus new-source whitespace/conflict scan | 0, incomplete coverage | Then-untracked evidence logs were not checked by the diff command; the separate scan covered only Go source |
| `git diff --cached --check` | 0 after S1 cleanup and staging | Entire staged delivery, including new evidence files |
| `shasum -a 256 -c docs/evidence/178/source-sha256.txt` | 0; all 21 files match | Approved Go source unchanged by S1–S4 |
| `git diff --exit-code c56ba3a21a43ba550185e991cecede796af4f054 -- ui` | 0; UI and its tests unchanged from base | No diff |
| `git diff --exit-code -- go.mod go.sum deploy/reference/Dockerfile harness/integration/agentsandbox/testworker/Dockerfile .github/workflows/ci.yml` | 0 | No baseline/dependency/build-image/CI change |

The browser failure is `ui/smoke/console.spec.ts:99`: the keyboard scenario times out waiting for the `Loading evidence` heading. It also occurred during the earlier #191 docs-only validation in this session. This slice did not change UI sources or tests. The full local gate is **not green**; no E16 CI result or merge readiness is claimed.

The first broad run found an expected catalog-count assertion in `internal/cli/adapters_test.go`. It was corrected to assert all four explicit adapter identities; the focused suite and subsequent full Go suite passed. Earlier diagnostic logs (`slice1-initial-catalog-assertion-failure.log`, `go122-default-link-failure.log`, `slice1-prior-full-gate.log`) are retained only in ignored `.tmp/e16-slice1/`. Neither full-gate log is committed. The focused/boundary logs and source hash manifest were moved to `docs/evidence/178/` with trailing whitespace removed; their results and source hashes are unchanged.

Source identity for the reviewed Go implementation is captured in [source-sha256.txt](../../docs/evidence/178/source-sha256.txt), covering all 21 changed Go implementation/test files. Packet prose and logs are not included in that source hash list.

The import-boundary probe uses the same Go 1.22 environment:

```sh
python3 - <<'CHECK'
import subprocess
for package, forbidden in [
    ('./internal/toolbackend', ('/internal/console', '/internal/workerprotocol')),
    ('./internal/adapters/bundled', ('/internal/console',)),
]:
    dependencies = subprocess.check_output(
        ['go', 'list', '-deps', '-f', '{{.ImportPath}}', package], text=True
    ).splitlines()
    assert not any(p.endswith(suffix) for p in dependencies for suffix in forbidden)
    print('PASS:', package, 'has no forbidden consumer dependency')
CHECK
```

Exit 0; [boundary output](../../docs/evidence/178/slice1-boundaries.log).

## Residual risks and explicit remaining work

- The MCP provider currently returns `ErrUnavailable`. Installed setup reports tool `notConnected`; configured tool-bearing Work returns `tool_transport_unavailable` before journal/runtime setup. This deliberate Slice 1 gate must be removed together with Slice 2 transport and worker catalog propagation, not separately.
- Production worker Schema/ParseAction/prompt/runtime payload validation and the remaining hard-coded dispatch consumers are not migrated by this slice. The console injection tests use deterministic providers/executors and do not prove the real worker path.
- `ResultRef` is separate from Target in the neutral result. Shared fact/CLI/Portal projection remains Slice 2, so the service does not publish it yet. Existing Target equality validation is unchanged.
- Pre-call Deny/scope/cross-claim/terminal/journal failures show zero double-provider calls, not server log evidence. Cross-claim probes are correlation-only; caller authentication remains #121. Real server zero-call proofs, timeout/oversize transport tests and kind E2E remain later slices.
- S2: after ToolDecision Allow, `tool_provider.go` can reject Catalog.Validate/context/Running before ProviderAttempt, leaving stage 1; the CLI then rejects the completed Work. Slice 2 must validate before Gateway or complete a Failed attempt/outcome pair, and prove the N2 Work passes CLI evidence validation with zero provider calls. This is not fixed by Slice 1.
- S3: `max-concurrent-calls` is validated but not enforced by `toolbackend.Set` yet; implementation and concurrent/cancellation tests are explicit Slice 2 Todos.
- PR delivery-contract blocker: the repository validator requires a closing reference to the packet issue. This partial Epic delivery intentionally uses `Refs #178` and fails that check. Resolve the workflow conflict with the maintainer before merge; do not add a closing reference to #178 or weaken the validator in E16.
- D1 remains B. D2 server selection is pending before Slice 2; E14 credential integration remains needed for full Epic acceptance.
- No changes were made to PR #191, its review requests or merge state. Its human review/merge is separate.
