# Task: Resolve effective authority by intersection

- Ticket: [#28](https://github.com/wunderforge/agenova/issues/28)
- Mission: Turn an admitted assignment's requested access into one smaller, immutable EffectiveAuthority without granting anything the request did not ask for.
- Target: `internal/authority/`, the shared EffectiveAuthority type from #25, focused tests, and this task packet.
- User value: A permitted Agent assignment receives only the tools, resources, model, memory, runtime profile, and timeout allowed by its reviewed boundaries.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Authority and credentials architecture](../../docs/product/architecture-contract.md#authority-and-credentials)
- [Canonical Team A request and issued-state fixtures](../../harness/fixtures/contract/v0/manifest.json)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [AgentTemplate producer #23](https://github.com/wunderforge/agenova/issues/23)
- [ClaimRequest producer #24](https://github.com/wunderforge/agenova/issues/24)
- [EffectiveAuthority producer #25](https://github.com/wunderforge/agenova/issues/25)
- [PolicyBundle producer #26](https://github.com/wunderforge/agenova/issues/26)
- [Assignment admission Task + Spec PR #96](https://github.com/wunderforge/agenova/pull/96)

## Scope

In scope:

- Require an `Allow` assignment-admission result from #27 before resolution.
- Intersect requested tools, concrete resource scopes, model profile, memory scopes, runtime profile, and timeout with the resolved AgentTemplate ceiling and MVP runtime limits.
- Preserve requested access and effective authority as separate immutable values.
- Narrow partially over-broad list requests to their allowed subset and cap timeout at the lower allowed maximum.
- Fail clearly when an explicitly requested non-empty dimension resolves to nothing, or a requested scalar profile has no allowed match.
- Prove the canonical Team A authority plus table-driven subset, over-broad, empty-intersection, scope-containment, runtime, and immutability cases.

Out of scope:

- Deciding whether the principal may use the project/template (#27), issuing a claim (#29), allocating a backend, or issuing credentials.
- Policy CRUD, a general scope language, wildcard tool/model names, runtime placement, or backend capability negotiation.
- Treating AgentTemplate defaults as authority that was not requested.
- Adding a second caller/project capability policy for the MVP unless the Owner explicitly expands #26; the #27 admission result is the caller/project/platform gate.

## Acceptance Criteria

- Resolution is impossible unless #27 returned exactly `Allow`; `Deny` and `ApprovalRequired` produce no EffectiveAuthority.
- The canonical Team A request resolves to the concrete authority dimensions shown by the shared issued-state fixture; system-managed authority identity remains part of later issuance.
- Tools and memory scopes use deterministic exact intersection; output follows request order and contains no duplicates.
- A requested resource scope must be concrete and survives only when it exactly matches or is contained by an allowed template scope; the concrete requested scope, never a wildcard ceiling, is emitted.
- A requested model profile and runtime profile must be explicitly allowed by the AgentTemplate ceiling; no different profile is selected silently.
- Effective timeout is the lower of the positive requested timeout and the positive template maximum; a missing usable runtime ceiling fails closed.
- Partially over-broad list requests are narrowed. If any explicitly non-empty requested list resolves to empty, resolution fails with a stable category/path and produces no authority.
- Omitted access remains omitted even when AgentTemplate defaults exist; defaults cannot enlarge the request.
- Mutating the source request, template, or policy objects after resolution cannot change the EffectiveAuthority snapshot.
- Shared types and resolution behavior remain backend-neutral.

## Negative Case

- Given an admitted request whose only requested tool or resource scope lies outside the template ceiling, resolution fails explicitly and produces no EffectiveAuthority or claim-ready result.

## Execution Todo

- [x] Scout the relevant producer contracts, fixtures, risks, and dependencies.
- [ ] Confirm this packet and its resolution rules with the Owner and Reviewer before implementation.
- [ ] Rebase on merged #23 through #27 contracts; do not create provisional duplicate public types.
- [ ] Add the smallest pure backend-neutral EffectiveAuthority resolver.
- [ ] Add fixture-driven and table-driven intersection, narrowing, empty-result, scope, runtime, and immutability tests.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -race ./internal/authority/...`
- `go test ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused test output naming the canonical Team A issued-authority case.
- A table of requested versus effective values for subset, partially over-broad, fully disallowed, empty/default-deny, scope-containment, model, memory, runtime-profile, and timeout cases.
- Mutation tests proving the returned authority is an independent snapshot.
- Passing repository baseline after rebasing on the merged dependency contracts.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Requested access is intent, not authority; resolution may preserve or reduce it but never add a tool, resource, model, memory scope, runtime profile, or timeout.
- Consume the public request/template/authority types owned by #23 through #25 and the admission result/policy reference from #27.
- Keep this resolver pure: no claim creation, backend allocation, credential retrieval, or evidence persistence.

## Decisions and Blockers

- Planning depth: Task + Spec because the intersection behavior is a shared authority contract consumed by claim issuance, gateways, evidence, CLI, and UI; no separate Design is needed for the pure resolver.
- Decision: #27 and #28 are separate logical checks inside one assignment-resolution pipeline, not separate services or external API calls.
- Proposed MVP rule: the matching #27 policy decision is the caller/project/platform gate. #28 performs capability-level intersection against requested access, AgentTemplate ceiling, and runtime limits without introducing another policy type.
- Proposed narrowing rule: preserve the allowed subset of a partially over-broad list, but fail if an explicitly non-empty requested dimension resolves entirely empty. Disallowed scalar model/runtime profiles fail rather than being replaced.
- Proposed default rule: AgentTemplate defaults configure reusable behavior but do not create claim authority absent matching requested access.
- Proposed scope rule: v0 supports exact scope equality or one terminal `*` ceiling such as `repo:acme/*`; any other wildcard form fails closed. EffectiveAuthority records only concrete requested scopes.
- Blocker: #23 must merge the AgentTemplate ceiling contract; #24 must merge ClaimRequest requested-access/runtime fields; #25 must freeze EffectiveAuthority; #26 and #27 must produce the approved policy/admission result.
- Shared decision with #27: authorization still needs an explicit requested project source. The current recommendation remains `ClaimRequest.spec.projectRef`; #28 must not infer project from task input.
- Implementation remains paused until the packet is approved and the producer contracts merge.
