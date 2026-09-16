# Task: Add the bundled adapter registry and lifecycle CLI

- Ticket: [#151](https://github.com/wunderforge/agenova/issues/151)
- Mission: Provide one explicit bundled-adapter registry and lifecycle service that both operators and future Platform orchestration use without provider-name branches.
- Target: `internal/adapterregistry`, `internal/adapters/bundled`, `internal/app`, `internal/cli`, `cmd/agenova`, and focused tests.
- User value: An operator can inspect, activate and initialize supported adapters by qualified identity, while the same registry resolves Platform requirements programmatically.
- PRD outcome: [Reference installation and initial policy bootstrap](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Technical design](design.md)
- [Architecture contract](../../docs/product/architecture-contract.md#reference-installation-and-bootstrap)
- [Platform contract design](../0044-platform-contract/design.md)
- [Platform resolver](../../internal/platform/resolve.go)
- [CLI composition root](../0040-cli-composition-root/task.md)
- [Start a GitHub Ticket playbook](../../docs/harness/playbooks.md#start-a-github-ticket)

## Scope

In scope:

- Qualified adapter identity, protocol/capability manifest, adapter-owned configuration schema and capability-specific factory registration.
- An explicit registry with deterministic catalog/lookup behavior and fail-closed duplicate, protocol and capability validation.
- A lifecycle service with inspectable installation lock state, idempotent identical install, conflict rejection and Platform-compatible initialization fragments.
- Bundled Kubernetes deployment, Agent Sandbox runtime and OpenAI-compatible model registrations through the same mechanism.
- `agenova adapters catalog|list|inspect|install|init`, human output and stable JSON output.
- A selected local installation state directory; no hidden global registry and no embedded credentials.

Out of scope:

- Downloading or loading third-party code, arbitrary executable paths, signatures or marketplace behavior (#149).
- Applying infrastructure or services (#45), credential references (#155), LiteLLM (#154), and AgentTemplate registration (#147).
- A universal operational adapter interface or provider-specific root command branches.

## Acceptance Criteria

- Registry construction rejects invalid identities, duplicate ID/version, protocol mismatch, capability/schema mismatch and missing capability factories without partial registration.
- Catalog and lookup are deterministic; Platform resolution uses the registry's descriptor lookup directly.
- Install records a secret-free exact adapter lock, is idempotent for an identical adapter, and rejects an active conflicting version without modifying state.
- Init emits a reviewable Platform YAML fragment (or equivalent JSON) generated from the adapter-owned schema and containing explicit adapter identity, local reference and named instance/profile defaults.
- The compiled CLI exposes all five adapter subcommands; invalid identity/version/subcommand/config exits non-zero with actionable diagnostics.
- The three bundled reference implementations register and construct through the same registry/factory boundary.

## Negative Case

- Unknown identity/version, duplicate registration, unsupported protocol, wrong capability, invalid schema, conflicting active version and corrupt state return no catalog/lock/Platform mutation.
- Init never emits credential fields and does not claim that adapter activation or configuration grants authority.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, dependencies and existing Platform descriptor boundary.
- [x] Confirm this packet under the Owner's 16 September 2026 execution exception: CI plus clean Codex review may proceed without waiting for another approval.
- [x] Add registry, manifest, schema, factory and lifecycle/store contracts.
- [x] Register the three reference bundled adapters without CLI provider branches.
- [x] Add adapter lifecycle CLI commands and composition wiring.
- [x] Add focused registry, lifecycle, Platform-consumption, CLI JSON/golden and compiled smoke evidence.
- [x] Run focused gates and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions and source-of-truth updates.

## Quality Gates

- `go test -count=1 ./internal/adapterregistry ./internal/adapters/bundled ./internal/cli ./internal/app ./cmd/agenova`
- `go test -count=1 ./internal/platform -run Platform`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Exact deterministic JSON for catalog/list/inspect/install/init plus human init YAML.
- First install, idempotent second install, conflicting version, unknown adapter, protocol/capability/schema failures and corrupt-state evidence.
- A Platform resolution test composed from bundled init fragments through the same registry.
- Compiled CLI smoke using an isolated installation state directory.

## Constraints

- Preserve `docs/product/architecture-contract.md`; adapter availability never grants claim authority.
- Generic registry/lifecycle code may depend on the pure Platform descriptor contract, never on Kubernetes/provider SDKs.
- Factories are capability-specific constructors returning capability-owned implementations; there is no universal runtime API.
- State and outputs contain manifests, identities and schema/default configuration only; never resolved credentials.

## Decisions and Blockers

- Decision: one catalog registry owns available bundled implementations; one injected store owns what a selected installation has activated.
- Decision: a file-backed store is selected by an explicit state directory at the CLI composition edge; tests and Platform orchestration inject memory/file stores directly.
- Decision: catalog may contain multiple versions, while one selected installation may activate only one version per qualified ID.
- Decision: init is a pure producer of mergeable Platform fragments. It neither installs infrastructure nor grants authority.
- Owner exception: on 16 September 2026 the Owner authorized implementation and merge after passing CI and a clean Codex review without waiting for a separate approval.
- Blockers: none; #40 and #44 are merged. #45 intentionally follows this ticket.

## Verification Evidence

- `go test -count=1 ./internal/adapterregistry ./internal/adapters/bundled ./internal/cli ./internal/app ./cmd/agenova` passed.
- `go test -count=1 ./internal/platform -run Platform` passed.
- `.\scripts\check.ps1 -All` passed after installing the lockfile-defined UI dependencies in this new worktree; Go, 109 component tests, production build, and 48 Playwright smoke tests passed.
- Compiled CLI smoke proved catalog, first/idempotent install, list, init and unknown-adapter failure against an isolated `--state-dir`.
- Focused tests prove duplicate/protocol/capability/schema/factory rejection, conflicting active version without mutation, corrupt store rejection, safe error redaction, and strict Platform parse/resolution from the generated fragments.
