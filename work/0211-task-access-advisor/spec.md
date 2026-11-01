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

The original Advisor adds optional template/Platform/evidence fields. The 1 November follow-up adds optional runtime.maxModelCalls on ClaimRequest, optional maxModelCalls ceilings on Template/Policy, and issued runtime.maxModelCalls. Old documents remain valid; an omitted request defaults to six or a stricter ceiling. The legacy omitted-six issued representation remains unchanged. Lifecycle semantics remain unchanged. Registries keep immutable semantics. No SQL migration, generic Skill/Task lifecycle, cross-Work elevation or cloud rollout.

Owner-approved follow-up (31 October): ordinary `task.type` may be omitted/empty;
it has no routing or authority requirement. Nonempty legacy types retain their
serialization and meaning. Omitted/empty forms canonicalize to omission without
inventing a classifier; explicit requests remain independent of the Advisor.
Whitespace-only types are invalid. Rules planning remains an explicitly typed
workflow contract; model planning accepts a generic objective. Remove the primary
type selector while preserving repository inputs and legacy typed copies and registered Advisor profiles.
See followups.md for failure diagnosis and real local Researcher acceptance.

## Negative Cases

Disabled mode; missing/oversized/over-ceiling guide; unknown/ambiguous task/profile; extra model fields; timeout/error; call/concurrency cap; unauthorized project; session revoked; stale/swapped task/guide; cross-user ID; Policy shrink; duplicate retry/restart; no silent mock fallback.

## Open Decisions

None blocking local work. Persistent rejected-advice audit, monthly monetary caps and multiple-profile composition are later scope, not delivered claims.

## Authorized 1 November extension

- Image safety limit: twenty for the opt-in reference Researcher; trusted per-Work
  limit: six by default, user selectable within Template/Policy ceilings. Explicit
  over-ceiling requests fail before allocation. The platform counts every reserved
  model dispatch, including failed provider attempts; Advisor planning is separate.
- Persist each reservation before model dispatch. Shared accounting serializes
  concurrent Gateway attempts within the existing single-writer service. Storage
  failure prevents dispatch. Recovery never resumes an interrupted Work or resets
  its authority; this is not a distributed/HA counter or a monetary spending cap.
- A refused extra model call produces actual ModelDecision Deny. If earlier model
  calls exist, retain a platform-reported incomplete summary with no invented
  final Agent answer. Normal reference Agents can finish within their limit.
- Entrypoint protocol `research-discovery-v2` explicitly opts a compatible image
  into safe registered Template tool discovery independently of current Work
  grants. Omitted protocol retains legacy grant-filtered discovery. Existing
  arbitrary images are not required to inherit a base image or migrate together.
- No tool descriptors from unrelated templates or secrets/endpoints are exposed.
  Schema shape validates catalog choices; actual identity/Work/operation/resource
  authorization stays in the Gateway at invocation. Denied calls reach no backend.
- Evidence adds bounded selected-action fields, turn and optional actionRef, not
  raw prompts, private reasoning, credentials or unbounded tool arguments.
- New Work removes presets, adds model-call settings beside timeout. Work details
  adds recorded progress and open activity above secondary context; requested vs
  granted remains intact. Configured Advisor model identity is read-only.
- New local immutable Researcher identity uses approved-general-model. Existing
  coding profile names, registrations, request hashes and historical evidence stay
  valid. No automatic global rename or wildcard model alias.
