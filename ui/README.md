# Agenova React console

The default **Demo** mode is an illustrative Work/Platform story, not live
evidence. **Connected** mode (`/?mode=connected#/work`) reads and submits through
the Agenova `/api` contract. Its source is selected by the Vite development
server, not by a browser-held credential or a Kubernetes-specific UI adapter.
The [portal harness](portal-harness.md) describes the user-facing boundaries.

From the repository root, with Go (see `go.mod`), Node.js 24 and npm:

```powershell
npm --prefix ui ci
npm --prefix ui run dev
```

Open the local address printed by Vite. Demo work is browser-local and clears
on reload. Connected work comes only from the configured API; a failed tunnel
shows a connection error, never demo data or an invented empty result. The
header preserves the selected mode and route across reloads. `/fixtures`
remains the original canonical contract reader.

After applying the reference Platform, open a **separate** PowerShell terminal
at the repository root and keep this loopback-only connection running before
opening Connected:

```powershell
agenova api connect
```

The Vite server forwards `/api` to `http://127.0.0.1:8088` by default. If the
local connection uses another port (`agenova api connect --port 18081`), set a
**server-side** variable before `npm dev`:

```powershell
$env:AGENOVA_API_URL = 'http://127.0.0.1:18081'
npm --prefix ui run dev
```

`AGENOVA_API_URL` accepts only a literal HTTP loopback origin (`127.0.0.1` or
`[::1]`) with an explicit port; Vite refuses remote hosts, credentials, paths,
queries and fragments. Do not use a `VITE_` variable or put tokens in the
browser bundle. The tunnel is authorized by the operator's Kubernetes access,
but it does **not** authenticate each local browser user. Use it only on a
trusted machine for this reference demo; it is not a production identity
boundary. Connected mode requires the installed Platform identity/revision
from `/api/setup`; if a legacy local demo happens to occupy port 8088, it
shows an error instead of treating those records as installed Work. The
installed service currently retains Work/evidence in one Pod's
memory, so restarting it loses that history. Durable authorized history is a
separate product capability.

The opt-in installed browser test compares the real API, a separate CLI
process, and the Portal against the same allowed and denied Work records. Run
it only after preparing kind/Ollama and those request references:

```powershell
$env:AGENOVA_CLI_PATH = (Resolve-Path .tmp/agenova.exe).Path
$env:AGENOVA_LIVE_REQUEST_REF = 'investigate-payment-retries'
$env:AGENOVA_LIVE_DENIED_REF = 'investigate-unapproved-project'
npm --prefix ui run test:installed
```

The test asserts the Agent Sandbox worker identity, real model/provider facts,
cleanup, denial without a Claim, and exact CLI/API record parity. It is not
part of fixture-only smoke tests because it requires an installed live service.

| Surface | Current source | What remains |
| --- | --- | --- |
| `/fixtures` | Canonical Go-parsed and shape-checked fixtures | No live API |
| `/console/requests/...` | Validated, correlated fixture snapshots; a derived narrowed case is labeled | This legacy storyboard does not query installed Work |
| `/#/work`, `/#/platform` in Demo | Typed illustrative stories in `portal-data.ts` | No mock record is authority |
| `/?mode=connected#/work`, `/?mode=connected#/platform` | Installed API through Vite's local `/api` proxy | No fixture fallback; source errors stay visible |
| New work in Connected | Canonical ClaimRequest POST to the configured API | The server supplies trusted identity; browser input cannot grant authority |

The portal does not add a second governance contract or run a frontend policy evaluator. Work-specific activity and cross-work Platform activity remain separate. The 45-to-30-minute narrowing story is illustrative, not proof that an Agenova worker ran or that the current resolver emitted those exact records.

## Staged single-claim console (#60)

Append one of these client routes to the printed local address (the original contract storyboard is now at `/fixtures`):

```text
/console/requests/fix-payment-timeout
/console/claims/claim%3Afix-payment-timeout%3A1
/console/requests/fix-payment-timeout?scenario=narrowed
/console/requests/fix-payment-timeout-team-b
```

These are presentation routes, not API endpoints. Request and claim references
resolve through the fixture EvidenceSource. The page compares request/issued
values only after reference and template correlation; the Team B snapshot has
no matching request document and says so explicitly.

The composition root also injects `ConsoleReferenceKeys`, which maps references
to the source's opaque keys for issued state and request documents. Fixture
prefixes live only in the fixture adapter; a replacement source supplies its
own mapping. A component test replaces both keys with unrelated opaque values.
This resolver is local presentation configuration, not an HTTP or evidence DTO.

The `narrowed` scenario removes `github.pull-request` from Team A's effective
tools in memory and revalidates with the canonical Go system parser. It is
visibly labeled as derived, not executed policy evidence. Source-state scenarios
are `loading`, `not-found`, `malformed` and `unavailable`; select them in the
labeled fixture control or use `?scenario=...`. Loading deliberately remains
pending until navigation, with no polling or timer. Malformed deletes `evidence`
from a cloned snapshot so generated shape checks expose the missing field.

```powershell
npm --prefix ui test -- --run src/console.test.tsx
npm --prefix ui run build
npm --prefix ui run test:smoke -- console.spec.ts
```

Console screenshots cover seven states at 1100x1000 and 390x844, with route
reload/history, keyboard, focus, landmarks and live-region smoke checks. The
full commands below include both console and original storyboard regressions.
No new dependency or canonical fixture payload is needed.

The `/console` route retains the earlier fixture-driven #60 console. The root
portal uses the approved [#143 connected checkpoint](../work/0143-midterm-vertical/task.md)
and can point at the installed reference API through the local proxy described above.

## Product portal: Demo and Connected

The root portal preserves the accepted Work, Agents, Policy and Platform pages.
Demo uses illustrative records. `/?mode=connected#/work` uses only the local
console API: empty, pending, denied, unavailable and completed states are not
filled with demo data. New work submits canonical ClaimRequest JSON; identity
is supplied by the server, not the browser. A completed task result and cleanup
failure remain independently visible.

```powershell
npm --prefix ui run dev
```

Run the installed-API tunnel described above. Vite forwards `/api` while
preserving the browser Host for the same-origin submission boundary. Work
detail refreshes at one-second intervals for up to
two minutes, then offers a manual refresh. Each HTTP read has an eight-second
timeout. A terminal claim continues refreshing until the final outcome arrives.
Platform configuration is not a gateway health check; recorded use is shown
separately, and unconnected Tool/Memory interfaces stay explicit.

`smoke/connected.spec.ts` intercepts canonical API responses to prove submission,
narrowed authority, denial, pending/failure, final-result polling and cleanup
failure presentation without contacting a cluster or model provider. These
intercepted checks do not replace a real installed-API/kind/Ollama E2E run.

## Shared checks

```powershell
npm --prefix ui run contracts:generate
npm --prefix ui run contracts:check
npm --prefix ui run typecheck
npm --prefix ui test -- --run
npm --prefix ui run build
npm --prefix ui run test:smoke
./scripts/check.ps1 -All
```

Generation is necessary only after an intentional canonical shape change. The repeatable check compares generated output, runs canonical fixture/parity tests, and demonstrates a stale-binding failure using a temporary candidate (tracked files remain unchanged). Build also checks drift. Browser smoke uses the production build; rerun build after edits. Screenshots and failure traces go to `.tmp/ui-smoke/`; CI uploads them as `frontend-smoke-evidence`. On Linux, install Chromium system dependencies with `cd ui` then `npx playwright install --with-deps chromium`.

## Ownership and validation

- `contractgen` reflects `ClaimRequest`, `AgentTemplate`, `IssuedState` and the shared Go evidence `View`, with reachable canonical/fact/provenance types. It reads canonical enum declarations and explicitly handles Duration and timestamp strings. Unsupported shapes fail; it is not a general Go schema library.
- `contracts.generated.ts` contains wire bindings and shape metadata. Pointer/slice nullability and JSON `omitempty` follow Go serialization. These are wire shapes, not proof that a value passed semantic validation.
- The Vite virtual module invokes the canonical Go parsers against the original manifest-referenced inputs on every build. YAML is normalized by Go, and caller-invalid cases use the caller parser. Only normalized valid data or sanitized error category/path enters the bundle. No separately maintained UI fixture payload exists.
- `shape-check.ts` checks display integrity using generated metadata. Required source fields missing/null are visible diagnostics even where Go could normalize them. It does not implement or replace canonical policy/semantic validation.
- `EvidenceSource.load(key)` returns canonical domain values or source status/diagnostics. Components import only that interface and generated types. `main.tsx` is the composition edge. The fixture source accepts canonical-parser build output, not untrusted remote data; #68/#60 must establish the later HTTP validation boundary.
- The connected portal consumes ordered invocation/lifecycle facts and bounded polling from the shared evidence API. Lineage and durable cross-process history remain outside this checkpoint. ApprovalRequired never appears as granted authority.

Vite's [virtual module API](https://vite.dev/guide/api-plugin) is used for build-time fixture loading; Playwright's [global setup](https://playwright.dev/docs/test-global-setup-teardown) owns the Vite preview server directly, including deterministic Windows teardown.
