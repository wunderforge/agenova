# Feature Specification: Deny claim-escaping worker and model-originated operations

- Ticket: [#198](https://github.com/wunderforge/agenova/issues/198)
- PRD outcome: [4. Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [5. Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

The architecture contract says authority is bound to the active claim and that worker messages, caller-supplied claim IDs and model output cannot widen it. This spec defines the observable results that count as proof on the reference kind install, and how each result is classified, so the demo narration and the #194 matrix rest on the same evidence.

## In Scope

- Matrix rows AC-1, AC-4 (terminal part), AC-5, FL-1, SR-4 and ES-1 (parity for these runs); SR-3 and AC-3 re-run with the provider oracle; AC-2 re-tested only to record current behaviour.
- The reference kind install with the bundled Agent Sandbox runtime and local Ollama.

## Out of Scope

- Fixes, network and credential exposure, filesystem isolation, worker network authentication, caller identity, real MCP tools, EKS.
- Whether the agent's own reasoning can be manipulated; only whether manipulated output gains authority.

## Requirements

- Given claim A Running and claim B known to the same control plane, when A's worker sends an operation naming B, then the operation is refused, no fact is added under B, and the provider count does not change.
- Given claim A Running in one install and claim B Running in a second install, when A's worker names B, then the operation is refused and neither install records a provider call for it.
- Given A is granted `git.read` on `repo:acme/payments` and profile `coding-standard`, when the worker requests another tool, another resource scope or another profile, then the request is refused and the provider count does not change.
- Given A has reached `Succeeded`, `Failed` or `Expired`, when its worker attempts any further operation, then the operation is refused, the provider count does not change, and a safe claim-attributed rejection record is available. The current implementation does not satisfy the rejection-record clause.
- Given the worker exceeds the model-turn limit, then the run ends with `agent-turn-limit` and the provider count equals the turns actually allowed.
- Every refused operation is classified as exactly one of: Gateway Deny (decision fact with policy reference and reason), edge rejection (refused before the Gateway), or run failure. None appears as `Succeeded`.

## Negative Cases

- Positive control: an allowed in-scope model call increases the provider count by one per call. A zero count without this control is inconclusive.
- A refusal that leaves no decision fact is not counted as "denied with evidence"; it is reported as an edge rejection and, where the ticket expects a recorded Deny, as a gap.

## Compatibility

- No product behaviour, API, evidence schema or reference manifest changes. The reference platform is restored to the admitted worker image after the run.
- Existing unit evidence (`TestGatewayUnknownAndInactiveClaimsFailClosed`, `TestJournalRejectsCrossClaimAndInvocation`) remains the source for Gateway-internal behaviour; this spec adds runtime evidence only.

## Open Decisions

- None. The two concurrency limits are recorded in the task packet; any finding that needs a product change goes to a follow-up issue for the owning Epic.

## Acceptance qualification

The requirements above describe supplemental harness scenarios. They do not replace the Ticket requirement for two known Running claims, a model-originated Gateway Deny with policy reference/reason, or recorded terminal-operation refusal. The current campaign is partial evidence; see [coverage](../../docs/evidence/198/summary.md#acceptance-coverage-and-remaining-work).
