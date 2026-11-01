# Local implementation plan — 1 November 2026

Owner authorized implementation, local tests and backup to the current feature
branch. No PR, merge, cloud access, infrastructure recovery or deployment.
Preserve debug.log, historical records and the running 5224 acceptance service.

## Delivery sequence

1. Contracts and budget: optional requested maxModelCalls, server-owned
   Template/Policy ceilings, default six and reference Agent safety limit twenty.
   Validate before allocation; atomic governed model-request accounting before
   provider dispatch. Persist reservations in existing facts and fail closed on
   recording failures. Existing recovery never replays interrupted Work.
2. Discovery and authorization: opt-in versioned reference Researcher protocol
   advertises registered Template tools separately from Work grants. Preserve
   legacy controlled execution. Every actual tool request still traverses the
   identity-bound Gateway; refusal is a recoverable observation, never a grant.
3. Evidence and Agent: record bounded public selected-action details and actual
   Gateway decisions; correlate turns without hiding denied checks. Improve
   internal/public context and handle denied calls without inventing absence.
4. UI: remove Task preset, add model-call settings beside timeout, show usage,
   put lifecycle progress above unchanged requested/granted permissions, open
   Activity and link stages to actual records. Display Advisor model identity.
   Give new Researcher registration a generic model profile; retain legacy names.
5. Visual audit: all reachable application routes plus nested details/dialogs,
   quiet consistent buttons, typography, focus and mobile layouts. Screenshots
   must be inspected, not merely produced.
6. Joint gate: contract/authority/Gateway/protocol tests, scoped and full Go
   checks, UI unit/build/browser regression, deterministic real-Gateway negatives
   and actual existing local-model/MCP positive tasks. Inspect renderings and
   record limitations. Commit and push only the feature branch.

## Acceptance matrix

- Same Template catalog, different Work grants: allowed calls execute and denied
  calls record actual Gateway refusal with zero backend attempts.
- Tool and resource-scope denial, invalid/expired/wrong-Work identity, malformed
  action and unavailable backend remain distinct.
- Budget two allows at most two model dispatches, including concurrent attempts;
  approved budgets beyond six work. Retries cannot reset consumed quota and
  interrupted durable Work never regains invocation authority after recovery.
- Old requests/protocols, explicit upstream/Manual, Suggest/Automatic, copy/retry
  and existing coding tasks remain compatible. Advisor calls remain separate.
- Internal CRM produces relevant synthetic-data findings; Public reads actual
  source material; missing access yields actual denial or explicitly no attempt,
  never fabricated enforcement or private chain-of-thought.
- New Work has no preset, no mandatory task type; requested/granted rows omit
  unrequested tools. Status is visible while running, and evidence navigation
  preserves scroll and distinguishes task completion from lifecycle completion.
- Existing Windows Owner-bootstrap test failure remains a separately reported
  baseline unless this batch changes that code. No AWS/CNI proof from local mocks.

## Progress

- [x] Reconcile approved TODOs and inspect existing contracts/practices.
- [x] Contracts, budgets and independent discovery/authorization.
- [x] Public action facts and Agent handling.
- [ ] UI changes and whole-product visual audit.
- [ ] Local joint regression and real-model acceptance.
- [ ] Evidence, feature-branch commit/push, owner handoff.
