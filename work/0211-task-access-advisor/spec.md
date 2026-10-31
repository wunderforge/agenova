# Feature Specification: optional advice and automatic submission

- Ticket: [#211](https://github.com/wunderforge/agenova/issues/211)
- PRD outcome: owner-approved local preflight extension, 30 Oct 2026.

## Intent and Requirements

1. Optional AgentTemplate accessGuide has bounded version, Markdown and structured profiles (id/type/description/canonical requestedAccess). Digest covers all fields plus template identity/image. No code execution or external runtime fetch.
2. Optional Platform services.accessAdvisor selects enabled/allowed/default modes, rules/model engine, planner model profile, timeout and call/concurrency/record bounds. Omission means disabled/manual. Bad configuration is rejected.
3. Manual remains unchanged. Suggest returns a draft without allocation. Automatic plans and submits in one operation. Per-request choice cannot bypass allowed modes. Explicit existing requestedAccess is never overwritten by platform defaults.
4. Authenticate and check current project submission eligibility before provider calls. Guide/catalog/template come from server context, not request-supplied authorities. Model sees bounded task, guide, ceiling and visible capability descriptions.
5. Selector outputs only registered profile ID or clarification plus bounded explanation. Unknown JSON/IDs, oversized text, injection, provider failure or ambiguity never silently widen access or fall back to a mock.
6. Profile must fit template and installed capabilities. Current Policy/Template/runtime resolution is reused at actual submission. All profile permissions are initially required: narrowing prevents automatic allocation. Deny/ApprovalRequired remain existing outcomes.
7. Advice binds owner, original request name/task/runtime/project/template digest, guide digest and TTL. Edited requestedAccess after suggest still follows ordinary Policy. Task changes invalidate provenance.
8. Same owner/name/content reuses plan/Work; conflicting content conflicts. Concurrent/retried calls cannot multiply model calls. Successful automatic Work recovery after restart uses existing stored intent digest before planning.
9. Receipts separate planner source/guide/profile/hash/model tokens from worker model facts. No raw provider payload/secrets. Accepted receipt persists in Work; bounded draft/failure records are private process state.
10. UI supports three modes, suggestion review, automatic start and reset on changed intent. Truthful provider label and normal copy/uncertain-submission recovery remain.

## Compatibility and Non-goals

Only optional fields on template/Platform/evidence. ClaimRequest/EffectiveAuthority/lifecycle unchanged; old documents remain valid. Registries keep immutable semantics. No SQL migration, generic Skill/Task lifecycle, cross-Work elevation or cloud rollout.

Owner-approved follow-up (31 October): ordinary `task.type` may be omitted/empty;
it has no routing or authority requirement. Nonempty legacy types retain their
serialization and meaning. Omitted/empty forms canonicalize to omission without
inventing a classifier; explicit requests remain independent of the Advisor.
Whitespace-only types are invalid. Rules planning remains an explicitly typed
workflow contract; model planning accepts a generic objective. Remove the primary
type selector while preserving repository inputs and legacy typed copies/presets.
See followups.md for failure diagnosis and real local Researcher acceptance.

## Negative Cases

Disabled mode; missing/oversized/over-ceiling guide; unknown/ambiguous task/profile; extra model fields; timeout/error; call/concurrency cap; unauthorized project; session revoked; stale/swapped task/guide; cross-user ID; Policy shrink; duplicate retry/restart; no silent mock fallback.

## Open Decisions

None blocking local work. Persistent rejected-advice audit, monthly monetary caps and multiple-profile composition are later scope, not delivered claims.
