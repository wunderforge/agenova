# Agenova fixture storyboard

Minimal React/TypeScript evidence foundation for [#59](https://github.com/wunderforge/agenova/issues/59). It is a read-only executable storyboard, with no HTTP client or live service. The [task packet](../work/0059-react-contract-foundation/task.md) defines scope and evidence.

From the repository root, with Go (see `go.mod`), Node.js 24 and npm:

```powershell
npm --prefix ui ci
npm --prefix ui run browsers:install
npm --prefix ui run dev
```

Open the local address printed by Vite. Choose any of the 12 canonical request/issued-state cases or the two explicitly labeled, in-memory display corruptions. Request intent does not grant authority; Team B denial has no fabricated claim. No absent invocation list is presented as observed zero invocations.

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

- `contractgen` reflects only `ClaimRequest` and `IssuedState` plus their reachable `api/v1alpha1` types, including the five required contracts. It reads the canonical enum declarations and explicitly handles the Duration JSON string serializer. Unsupported shapes fail; it is not a general Go schema library.
- `contracts.generated.ts` contains wire bindings and shape metadata. Pointer/slice nullability and JSON `omitempty` follow Go serialization. These are wire shapes, not proof that a value passed semantic validation.
- The Vite virtual module invokes the canonical Go parsers against the original manifest-referenced inputs on every build. YAML is normalized by Go, and caller-invalid cases use the caller parser. Only normalized valid data or sanitized error category/path enters the bundle. No separately maintained UI fixture payload exists.
- `shape-check.ts` checks display integrity using generated metadata. Required source fields missing/null are visible diagnostics even where Go could normalize them. It does not implement or replace canonical policy/semantic validation.
- `EvidenceSource.load(key)` returns canonical domain values or source status/diagnostics. Components import only that interface and generated types. `main.tsx` is the composition edge. The fixture source accepts canonical-parser build output, not untrusted remote data; #68/#60 must establish the later HTTP validation boundary.
- Detailed invocation records, lineage, live polling and additional lifecycle snapshots are not claimed. ApprovalRequired is covered as an explicitly derived test, never as granted authority.

Vite's [virtual module API](https://vite.dev/guide/api-plugin) is used for build-time fixture loading; Playwright's [global setup](https://playwright.dev/docs/test-global-setup-teardown) owns the Vite preview server directly, including deterministic Windows teardown.
