# #59 frontend evidence

- Date: 2026-09-08
- Branch: `codex/0059-react-contract-foundation`
- Implementation: `93ebbeb` (the subsequent evidence commit changes documentation/artifacts only).
- Gate: `./scripts/check.ps1 -All`
- Result: PASS, exit 0. Full output: [baseline.txt](baseline.txt).
- Environment: Windows, Go 1.26.4, Node 24.11.1, npm 11.6.2, Playwright 1.63.0 / Chromium 153.0.8010.12.

## Reproduction and results

| Exact command | Result |
| --- | --- |
| `npm --prefix ui ci --no-audit --no-fund` | PASS; clean lockfile install, 103 packages |
| `npm --prefix ui run browsers:install` | PASS; isolated Chromium test browser installed |
| `npm --prefix ui run contracts:check` | PASS; 12 canonical cases, YAML/JSON equivalence, enum-source drift, deliberate stale-binding rejection and 22 fixture/shape tests |
| `npm --prefix ui run typecheck` | PASS |
| `npm --prefix ui test -- --run` | PASS; 40 tests in 2 files |
| `npm --prefix ui run build` | PASS; 20 modules, JS 208.46 kB / 63.89 kB gzip; includes drift check |
| `npm --prefix ui run test:smoke` | PASS; 7 tests, six rendered states and mobile overflow assertion |
| `./scripts/check.ps1 -All` | PASS; 13 Go test packages, vet/module/gofmt/docs/boundaries, integration-package compile, and all frontend checks above |
| `git diff --exit-code 3504988 -- api/v1alpha1 harness/fixtures/contract/v0` | PASS; canonical sources and fixture files unchanged |
| `git diff --cached --check` | PASS |

## Canonical case matrix

The build invokes canonical Go parsers on the original manifest inputs. Invalid inputs produce sanitized diagnostics only; no raw invalid payload is bundled. Each row is checked against its manifest expectation and rendered in a component test.

| Full canonical case ID | State / outcome |
| --- | --- |
| `claim-request.valid.team-a-engineer-json` | Request intent only; no issued authority |
| `claim-request.valid.team-a-engineer-yaml` | Equivalent normalized request intent |
| `claim-request.invalid.missing-template` | required-field |
| `claim-request.invalid.missing-task` | required-field |
| `claim-request.invalid.missing-runtime` | required-field |
| `claim-request.invalid.self-asserted-principal` | self-asserted-principal |
| `claim-request.invalid.secret-value` | secret-value; raw value excluded |
| `issued-state.valid.team-a-engineer` | Allow / Running; issued authority, backend identity and ClaimRunning event |
| `issued-state.valid.team-b-denial` | Deny; no claim, authority, backend or evidence claimId |
| `issued-state.invalid.caller-effective-authority` | system-managed-field via caller parser |
| `issued-state.invalid.caller-claim-phase` | system-managed-field via caller parser |
| `issued-state.invalid.caller-backend-identity` | system-managed-field via caller parser |

## Rendered proof

- [Request intent](request-only.png): `claim-request.valid.team-a-engineer-json`.
- [Allow / Running](allow-running.png): `issued-state.valid.team-a-engineer`.
- [Pre-claim denial](deny-no-claim.png): `issued-state.valid.team-b-denial`.
- [Invalid caller state](invalid-caller-state.png): `issued-state.invalid.caller-effective-authority`.
- [Missing field](missing-field.png): `derived.missing-field`, from Team A issued state with `evidence` deleted in memory.
- [Unknown field](unknown-field.png): `derived.unknown-field`, from Team A issued state with `futureField` added in memory.

All screenshots come from browser smoke against the production build. The Allow, Deny and unknown-field images were visually inspected; all six have executable assertions. Derived cases are visibly labeled, never represented as canonical fixtures.

Additional focused negatives mutate `decision.result`, `claim.phase`, each evidence array (missing/null), and `spec.task.unknownField`. Arbitrary nested JSON under task input remains permitted. A component test changes only the denial decision vocabulary to ApprovalRequired and proves no grant; this is a derived vocabulary test, not a new canonical fixture. Source replacement, not-found, rejection, consumer mutation isolation and late responses are also tested.

## Decisions and limitations

- Bindings and shape metadata derive only from the two v0 roots and reachable types, with explicit Duration serialization; no general-purpose generator or policy engine was added.
- Go semantic validation happens at fixture build time. Browser validation checks generated wire shape and source completeness only. The future HTTP source must establish its trusted semantic-validation boundary under #68/#60.
- No HTTP/live polling, operational controls, full console, detailed invocation history, or real-backend proof is claimed. Canonical positive issued states currently cover Allow/Running and pre-claim Deny only.
- Windows shell-based preview teardown initially hung after assertions; direct Vite preview ownership now completes cleanly. The full gate also caught and resolved the helper's HTTP/HTTP2 TypeScript union issue before this accepted run.
- Race testing is left to the repository's Linux PR profile; local `-All` compiles the integration package but does not run a real backend.
