# Task: Minimal two-task incident MCP demo

- Ticket: [#242](https://github.com/wunderforge/agenova/issues/242)
- Mission: Demonstrate task-scoped file access using one existing MCP tool and two incident datasets.
- Target: `harness/demo/incident-mcp`, fixture/composition tests and bounded rejection diagnostics.
- User value: The same person and Agent investigate two incidents without giving every task both datasets.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [demo](../../docs/product/prd.md#8-demonstrable-contributor-path).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Spec](spec.md), [architecture contract](../../docs/product/architecture-contract.md), [AIDLC](../../docs/development/AIDLC.md), [playbooks](../../docs/harness/playbooks.md), [fixture](../../harness/integration/mcpfixture/main.go), [adapter](../../internal/adapters/bundled/mcp.go), [installed composition](../../cmd/agenova-control-plane/tools.go), [service](../../internal/console/service.go).

## Scope

In scope:

- One existing `read_file`, two synthetic datasets, disjoint full-path profiles, one template/policy and two canonical Work inputs.
- Real local MCP SDK/Worker processes, reference in-memory allocation, explicitly scripted model tests, deterministic negative probes and independent receipts.
- Minimal deployment manifests/instructions, connected real-model rehearsal driver, and safe existing-kind activity evidence for pre-Gateway argument rejection. The Owner approved integrated release with #243 on 20 October; deployment remains the integrator's responsibility after joint gates.

Out of scope:

- Independent cloud changes by this lane, existing credential reads, new authority contracts, write tools, arbitrary URL/shell/SQL and extra governance stacks. No half-integrated release before #243 joint acceptance.

## Acceptance Criteria

- Same principal/template/policy, different requested/effective scope; each task reads only its own two files through the same tool.
- Fixed profiles bind non-overlapping full paths; Worker cannot select endpoints, roots or credentials.
- Actual MCP tools/call receipts match trusted invocation IDs; negative probes have zero new downstream calls.
- Valid/missing/wrong synthetic token cases are safe; no real credential or shared Secret access.
- Reproduction, exact runtime/model limitations and minimal deployment proposal are ready before separate approval.

## Negative Case

- Running A -> B scope and A scope + B file, terminal replay and admission deny must not reach MCP. Positive controls prove the observer works.

## Execution Todo

- [x] Scout existing implementation and self-review against PRD/contracts; no new authority engine needed.
- [x] Add data, canonical configuration and minimal future deployment manifests.
- [x] Add real local SDK/Worker composition tests and rejection evidence.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`; Windows baseline failure verified separately on Linux.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./cmd/agenova-control-plane ./internal/console ./internal/adapters/bundled ./internal/toolbackend ./internal/toolgateway`
- `go test -tags integration -run '^TestIncidentDemo' -count=1 -timeout 3m ./cmd/agenova-control-plane`
- `go test ./...` from `harness/integration/mcpfixture`
- `go test ./harness/demo/incident-mcp/acceptance` and local render of `harness/demo/incident-mcp-aws`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Canonical input tests, actual SDK/file/Worker results, decision/receipt correlation, negative call deltas, exact local runtime/model labels and no-deployment statement.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Owner subsequently authorized parallel local completion with #243, joint acceptance, integrated merge/deployment and AWS verification/repair on 20 October. This lane performs local changes/tests only; the integrator owns live operations. Never broaden cleartext HTTP/TLS rules or access real keys.

## Decisions and Blockers

- The existing exact fixture URL is reused. No local Ollama endpoint was available at initial inspection; scripted responses prove wiring only. Real-model reliability and AWS network/credential isolation remain separate acceptance.
- [Local acceptance, exact commands and remaining deployment gates](acceptance.md). No shared deployment or merge performed.
- The connected campaign driver retains all 20 attempts, requires exact real-provider metadata and correlated SDK receipts, stops on unknown submission/caller-policy drift, and keeps mechanical business screening separate from human review and safety probes. The AWS overlay has a deliberately invalid digest placeholder and must be finalized only by the reviewed release process.
