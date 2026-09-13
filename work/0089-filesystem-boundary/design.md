# Technical Design: Minimum agent filesystem boundary

- Ticket: [#89](https://github.com/wunderforge/agenova/issues/89)
- Feature spec: [spec.md](spec.md)

Implemented reference approach; real-backend enforcement remains owned by #48/#51.

## Current State and Constraints

The merged #30 RuntimeBackend exposes allocation, observation, explicit start, termination and cleanup while keeping application outcome and pool administration outside the shared interface. #89 adds filesystem description values to Allocation and Observation without adding a sixth operation.

The reference runtime stores a per-allocation simulated task filesystem and reports `/workspace` as the worker-visible directory. Its reusable cases prove the neutral contract and lifecycle behavior, not native isolation. The Agent Sandbox adapter reports filesystem evidence as `Unsupported` until #48 maps a substrate and #51 verifies it.

## Decision

Use one backend-selected working directory with the spec's fixed rule. Place scratch, synthetic HOME and tool caches below it, and leave ordinary local process/file APIs direct. The adapter implements the substrate boundary; shared Agenova code expresses the requirement, correlation and evidence only.

After #30 merges, inspect its actual types first. Add only the minimal neutral working-directory description and capability/evidence needed by its accepted launch/observation results. Reuse its allocation identity and errors. Do not add a general file API, a second lifecycle registry, a new public Workspace resource, or a host-path mapping. If no accepted extension point can carry these values, stop for a reviewed producer/consumer decision rather than editing #30 for convenience.

The minimum output demonstration is a file or bundle explicitly produced under W and exported while work is active. A test-only collector verifies bytes into a separate harness-owned destination. It simulates the external handoff, is never worker-mounted, and is not a proposed RuntimeBackend filesystem API. A real consumer must use its governed output/tool operation; this ticket does not invent one. Termination does not wait for a human or guarantee final output salvage.

## Ownership and Contract Boundaries

| Surface | Later #89 responsibility | Boundary retained |
| --- | --- | --- |
| internal/runtime | Minimal working-directory semantics attached to the final #30 seam | No provider types or application outcome ownership |
| internal/operator | Reference model correlated to claim/allocation | Simulated enforcement, no hostile isolation claim |
| internal/runtime/contracttest | Reusable success/fault cases from the spec | Factories hide implementation setup; no shared syscall proxy |
| Local compatibility fixture | Known repo, real git/compiler/test commands, output bytes | Test-only trusted workload; no execution of arbitrary user code |
| #48 / adapter note | Consume accepted capability requirements and list gaps | Provider layout/objects stay with adapter owner |
| #51 | Real working directory, mounts, negative access and cleanup | G5 evidence cannot be inferred from G2 |
| #52 / gateways | Preserve credential and bypass boundaries | Local file permission is not external-access authority |
| #53 | Consume cwd and export boundary during integration | Fixture/mock artifact can proceed independently |

## Alternatives Considered

- Send every file access through Tool Gateway: incompatible with ordinary git/compiler/process use and creates an unnecessary filesystem product.
- Expose configurable host mounts or a generic path allowlist: enlarges authority and requires a policy language, host semantics and additional credential risks.
- Enforce only cwd or string-prefix validation: compatible with tools but does not constrain process access, links or traversal; useful test setup cannot stand in for isolation.
- Persistent workspace with post-run retrieval: adds storage/access lifecycle and retention product scope. Prefer explicit pre-termination export and ephemeral cleanup.
- Force the current Agent Sandbox spike to define the rule: would reshape a shared contract for provider convenience. Keep unsupported gaps visible.

## Verification Strategy

Three evidence levels are deliberately separate:

1. **Reference model (simulated):** reusable cases exercise pre-Start denial, task access after Start, a readable-but-not-writable runtime fixture, direct outside/traversal denial, fresh replacement, the post-termination export cutoff, and cleanup refusal while termination remains incomplete. Synthetic sentinels prove model decisions only; they do not contain arbitrary native programs.
2. **Local process compatibility (real local commands, no isolation):** a harness-owned temporary root contains W, synthetic runtime/outside fixtures and a separate collector destination. Build a tiny deterministic repository with local git identity/config, set process cwd to W, use an allowlisted environment with HOME/temp/caches under W, edit a source file, run git diff and a compiler/test command, and record command/tool versions, exits, exported bytes and digest. The test-only collector accepts explicit relative regular files only, rejects symbolic and hard-link aliases and special/oversize files, and rejects collection after explicit closure. Collector closure is not worker termination or FS-N7 evidence. No arbitrary native outside-write attack runs on the host; model denial is recorded separately.
3. **Real backend (#51, not executed by this ticket):** repeat FS-P1, FS-P3, FS-N0, FS-N1, FS-N2, FS-N4, FS-N7 and FS-N8 as the actual worker identity. FS-N1 must attempt a direct absolute-path create/write outside W and compare its sentinel. FS-P3 must use a descendant heartbeat and show it stops before cleanup succeeds. Capture native errors, unchanged sentinels, backend layout/mounts, worker configuration and cleanup observations. Document supported OS/profile and residual gaps; do not replace real isolation checks with collector validation or model denials.

Reproduction starts with the focused commands in task.md. The reference and local compatibility fixtures publish exact case names, command/tool versions, exits and artifact identity. The existing `-All` gate checks repository regression and compiles the integration package; it does not run a cluster or prove native isolation.

## Risks and Compatibility

- Read-only runtime data requires tool cache/temp configuration; hard-coded writes outside W are unsupported, not a reason to loosen the rule. Runtime special facilities require explicit #48/#51 review.
- Link resolution alone has race hazards. Test collector validation for trusted local compatibility is not hostile isolation; the real backend must enforce the rule for worker native I/O independently.
- A source fixture may be prepared before start, but production acquisition must use already-authorized access. No global git helper, host home or provider secret is inherited to make clone/push work.
- Early or failed termination can lose unexported output. Failed cleanup must not silently retain a reusable workspace or change the work outcome; confirmed cleanup remains separate resource evidence.
- #30's five-operation boundary remains unchanged; filesystem checks stay test probes rather than a sixth backend operation.
- Update only evidence/status sources whose owned facts changed; do not broaden the PRD or architecture contract from this ticket.
