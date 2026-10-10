# Task: Resolve external credentials behind trusted adapters

- Ticket: [#155](https://github.com/wunderforge/agenova/issues/155)
- Mission: Resolve opaque external credential references only inside trusted service adapters, with fail-closed admission and no secret-bearing public or worker outputs.
- Target: host-only credential contracts/resolvers, Platform reference validation and resolver activation, adapter composition, credential-boundary tests and sanitized evidence.
- User value: Tool, Model and Memory adapters can consume one reviewed credential boundary instead of inventing provider-specific secret configuration.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap); this elaborates the accepted E14 extension without adding a secret-management product.

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Specification](spec.md) and [design](design.md)
- [Architecture](../../docs/product/architecture-contract.md): Authority and Credentials, Backend Neutrality, Evidence Surfaces, Reference Installation and Scope Discipline
- [AIDLC](../../docs/development/AIDLC.md): Sources of Truth, Adaptive Planning Depth, PR Review Context and Source Ownership
- [Playbooks](../../docs/harness/playbooks.md): Start a GitHub Ticket, Change a Core Contract and Elaborate Parallel Work
- [Accepted credential boundary](../0036-credential-boundary/task.md), [field validation](../../api/v1alpha1/credential_fields.go), [gateway validation](../../internal/gateway/validate.go), and [worker launch](../../internal/app/run_service.go)
- [Platform contract](../../api/v1alpha1/platform.go), [resolution](../../internal/platform/resolve.go), [registry types](../../internal/adapterregistry/types.go), and [bundled adapters](../../internal/adapters/bundled/)
- [Installation readiness](../../internal/platformapply/service.go) and [readiness checks](../../internal/adapters/bundled/credentials.go)
- [Model gateway](../../internal/modelgateway/), [Tool gateway](../../internal/toolgateway/), [installed composition](../../cmd/agenova-control-plane/main.go), and [Agent Sandbox tests](../../internal/runtime/agentsandbox/)
- [Host credentials](../../internal/credentials/credentials.go), [contract tests](../../internal/credentials/credentials_test.go), [Gateway composition tests](../../internal/credentials/gateway_test.go), [Secret adapter](../../internal/credentials/kubernetes/secret.go), [bounded transport](../../internal/credentials/kubernetes/kubectl.go), [adapter tests](../../internal/credentials/kubernetes/secret_test.go) and [foundation evidence](../../docs/evidence/155/host-boundary.md)
- [Boundary review corrections and evidence](../../docs/evidence/155/review-boundary.md): resolver/consumer deadline separation and owned encoded Secret-buffer cleanup
- Consumers: [#175](https://github.com/wunderforge/agenova/issues/175), [#178](https://github.com/wunderforge/agenova/issues/178), [#179](https://github.com/wunderforge/agenova/issues/179); parent [#176](https://github.com/wunderforge/agenova/issues/176)

## Scope

In scope:

- One typed opaque reference, explicit host resolver identity and operator-captured binding; deterministic test resolver plus a bounded Kubernetes Secret adapter.
- Fail-closed malformed/missing/unauthorized/wrong-kind/unavailable behavior before consumer external calls; sanitized errors and lifecycle-limited value use.
- Platform shape-only validation/plan, narrowly authorized resolver readiness, explicit restart/reconcile reload semantics, and worker/public secret exclusion.
- Focused contract/spy/privacy tests, full baseline and sanitized real Secret integration evidence when an explicit existing cluster is supplied.

Out of scope:

- Vault/cloud secret-manager implementations, provider SDK integration, authentication/SSO, worker-side resolution, automatic environment/file lookup, general rotation management and cluster creation.
- Claim, authority or Policy expansion; merging the unaccepted E17 branch into this producer task.

## Acceptance Criteria

1. Tool/Model/Memory host adapters share a typed reference and resolver boundary; values never enter stored public contracts or worker configuration.
2. A deterministic test resolver and one Kubernetes Secret reference implementation provide bounded behavior behind explicit host binding.
3. Malformed, unknown, unauthorized, missing, wrong-kind and unavailable references fail before external provider calls. Unauthorized names do not trigger secret discovery; public errors do not reveal unrelated existence or raw provider text.
4. Resolved bytes have invocation/lifecycle-limited exposure; host handles redact formatting/serialization. Adapter ownership remains responsible for not copying callback material into public outputs or diagnostics.
5. Platform validate/plan performs no secret retrieval. Apply/readiness uses only resolver-owned authorized checks; unsupported credentialed adapters remain explicitly unsupported.
6. Reload requires supported restart/reconcile in the first scope and does not change active claim authority. Mechanical scans and #36 worker-environment assertions remain passing.
7. Focused/race/full gates and sanitized real Kubernetes Secret evidence are captured separately; absent live environment is an explicit blocker, not a passing backend result.

## Negative Case

- A host handle bound to one allowed reference cannot be retargeted by a worker/request. Malformed or unauthorized references have zero resolver/backend calls and zero consumer external calls.
- Missing/forbidden/wrong-kind/unavailable Secret results never echo secret values, raw command output or existence details. No environment scanning, task-supplied paths, credential defaults or fallback resolver is allowed.

## Execution Todo

- [x] Read #155, routing, PRD, accepted #36 foundation, Platform and registry boundaries; isolate this producer from E17.
- [x] Assignee self-review: Task/Spec/Design matches the accepted Ticket, PRD and architecture; no authority/identity product expansion or separate packet approval is required.
- [x] Slice 1: host-only typed reference, immutable explicit registry/binding, scoped material and deterministic spy tests.
- [x] Slice 1: bounded Kubernetes Secret resolver and named synthetic negative/rotation/privacy tests; no live backend claim.
- [x] Review corrections: separate resolver timeout from caller-owned provider invocation; replace immutable Secret value strings with clearable encoded storage and decode only the selected value; focused/race/repeated/full gates passed. Independent re-review remains required.
- [x] Slice 2: accepted Platform reference shape, resolver registry activation/readiness and consumer composition, preserving #36 and unsupported behavior.
- [x] Slice 3: add opt-in Secret/RBAC/update and installed startup harnesses with synthetic-only fixtures, guarded cleanup and public/host-Pod scans. Execution records remain local.
- [ ] Independent human reproduction and acceptance of the credential producer.
- [ ] Independent reproduction of focused/race/full and real backend gates; continuation execution records are local by the Owner's publication choice.
- [ ] Obtain independent review; do not mark #155 complete or unblock live E17 merely because unit tests pass.

## Quality Gates

- `go test -count=1 ./internal/credentials/... ./api/v1alpha1 ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/app ./internal/runtime/agentsandbox` once the new package exists; new credential packages also run with `-race`.
- Docs preparation: `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` and `git diff --check`.
- `.\scripts\check.ps1 -All`

## Evidence Required

- Source revision, exact focused/race/repeated/full commands and sanitized raw logs; zero resolver/backend/external-call spies with positive controls.
- Reference/material/error formatting and serialization scans; credential-free worker/manifests and unchanged public authority evidence.
- Real Secret/RBAC, reload and zero-call controls only on an explicitly named existing context/namespace; record no secret values. Missing environment remains outstanding.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- E4 remains the Gateway owner. This producer supplies credentials only after trusted host admission; references cannot grant claim authority.
- Backend/namespace/API objects remain inside the Kubernetes resolver adapter. Public contracts contain opaque references, never values or task-controlled paths.
- No actual credentials may be read or copied during packet preparation or synthetic testing; no cluster mutation without an explicit existing context.
- Go 1.22 compatibility and existing dependencies remain unchanged unless a reviewed need is recorded.

## Decisions and Blockers

- User explicitly authorized taking #155 on 2026-10-09 and reported no existing kind context. Keep real Kubernetes integration blocked; do not create a cluster.
- The Owner requested a code-only public update. Continuation verification records and operational metadata remain local; public backend acceptance requires independent reproduction using an explicitly selected environment.
- Independent producer worktree and `codex/0155-host-credentials` start at accepted main `ebb4c04f7b8cc76d63036c820e04c62af68fad3d`; E17 remains on its separate draft branch.
- #155 assigned to the current GitHub account (`yanyang15037755`) after authorization; independent reviewer/reproduction is not yet named.
- First implementation slice is the host-only boundary plus bounded Secret adapter. Platform/public acceptance and installed consumers are separate Todo slices, not implied by a test resolver.
- Packet self-review completed on 2026-10-09. The first Secret adapter accepts only explicitly selected Opaque Secret keys in one captured namespace; backend names/key rules follow the official Kubernetes contract linked in the design. Public reference acceptance remains unchanged until coordinated Platform work.

## Foundation Verification - 2026-10-09

- Created the packet using `new-task.ps1 -Issue 155 -Slug host-credentials -WithSpec -WithDesign`, self-reviewed it, and implemented only the first internal producer slice. Public Platform acceptance, generated contracts, installation, worker protocol, Gateway production code and authoritative product documents remain unchanged.
- Native focused Go tests passed after a Tool fixture correction (`Tool: git`, `Action: read`, not a duplicated dotted operation). The new output-cap regression first reproduced an `io.ReaderFrom` bypass from embedding `bytes.Buffer`; private composition now exposes only the bounded Write method, and the unchanged regression passes.
- Tests prove exact immutable host binding, qualified identity/version validation, zero resolver/external calls for rejected references and existing Tool/Model admission denials, unavailable Secret blocking, scoped buffer clearing, reload observations, concurrent uses and redacted handles/errors. Kubernetes command selection is explicit and deadline/output bounded; scripted getters are not RBAC or live Secret proof.
- The first WSL full gate stopped because the Windows managed-worktree gitdir was not understood by Linux Git. The final ignored runner maps `GIT_DIR`/`GIT_WORK_TREE` and uses a Windows/WSL-compatible dependency junction; package-lock hashes match. No harness checks or source/dependency requirements were weakened.
- The [final source campaign](slice1-host-boundary-linux.log) passed focused tests, race checks, 20 repeated credential-package runs and `check.ps1 -All`: all Go, generated contracts, 124 frontend tests, build and 52 browser cases. Only documentation/publication edits followed. [Evidence](../../docs/evidence/155/host-boundary.md) records exact gates and residual gaps. Keep #155 open; independent review, Platform wiring and real Secret integration remain outstanding.
- Publication staging found three packet files with an extra blank line at EOF. The initial commit was pushed before that failure was handled; a follow-up removes the blank lines and reruns the documentation, PR-body, staged and whole-branch whitespace gates. No implementation changed after the passing source campaign.

## Boundary Review Corrections - 2026-10-09

- Draft #213 at `a9eabf4` passed baseline CI. Its two P2 findings are addressed before expanding Platform work: the resolver child deadline is released before the consumer receives its original caller-owned invocation context, and encoded Secret values now use owned clearable RawMessage projections instead of immutable value strings. Only the selected JSON byte field is decoded; all owned encoded projections are cleared even on identity/selection failure.
- The context regression failed before the production change for background/short/long caller contexts. Cleanup regressions cover valid/escaped selected data and missing/empty/null/non-string/invalid/oversized data, with unrelated data never decoded. Buffer cleanup is best-effort and does not claim erasure of all runtime/library temporaries or trusted consumer copies.
- The [final review campaign](review-boundary-linux.log) passed focused/race checks, 20 repeated credential-package runs and `check.ps1 -All`: all Go, generated contracts, 124 frontend tests, build and 52 browser cases. Only documentation/publication edits followed; [review evidence](../../docs/evidence/155/review-boundary.md) owns exact results and limits. Platform activation/installed wiring, no-context live Secret/RBAC/reload and independent acceptance remain outstanding. Keep #155 open and #213 draft.


## Platform Implementation and Publication Scope

- The dedicated model reference and explicit credential category activate immutable host bindings without Secret reads in metadata paths. Every actual apply checks selected material before local activation or target mutation, including unchanged and activation-only plans. Installed startup checks precede readiness; admitted model invocations resolve independently and reject exact-value response echoes.
- The reference resolver uses namespace-only config and captures exact name/key selections from typed model references. Secret permissions are exact named get; unsupported Tool/Memory and additional reference combinations remain separate consumer work.
- Opt-in source and installed startup harnesses are executable code, require an explicit existing context and a new task namespace, use synthetic material, and guard deletion with resource UIDs. They do not establish external provider health or an installed credentialed worker run.
- Only code, tests and implementation instructions are published in this continuation. Raw verification records, execution targets and image/resource identities remain local at the Owner's request. Independent human review and public reproduction remain required; keep #155 open and #213 draft.
