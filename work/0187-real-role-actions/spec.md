# Feature Specification: Real role-scoped PR and rollback proof

- Ticket: [#187](https://github.com/wunderforge/agenova/issues/187)
- PRD outcome: [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [facts/accountability](../../docs/product/prd.md#5-facts-and-accountability).

## Intent

Show that reusing one agent template does not imply one standing tool permission set. The trusted requester and active policy narrow each Work, and the Tool Gateway enforces that grant before any external provider operation.

## In Scope

- A dedicated payment demo GitHub repository and disposable kind Deployment, both observable outside Agenova.
- Two operator-selected reference identities, one template, team-specific tool ceilings, distinct English task objectives, and four live model-directed Work cases.
- Host-side bounded `git.read`, `github.pr.create`, and `kubernetes.rollback` providers behind the existing Tool Gateway.
- CLI/API/UI evidence for decisions, provider outcomes, real external result, and cleanup.

## Out of Scope

Production identity, user impersonation UI, arbitrary repository or cluster operations, broad adapter catalog support, token provisioning, and presenting simulated incident facts as a real production outage.

## Requirements

- Given the Developer principal and an admitted claim, when the agent invokes `github.pr.create` with a valid model-authored replacement, then tests pass, a branch and PR are created in the dedicated repo, and the resulting URL is reported with the invocation.
- Given the SRE principal and an admitted claim, when the agent invokes `kubernetes.rollback`, then only the dedicated Deployment rolls back from the bad revision to the known-good revision and reaches Ready.
- Given either principal requests the other role's action, when its worker invokes the tool, then the gateway records Deny and the provider is never called.
- Given a model elects not to call an operation, no corresponding operation fact or external action is fabricated.

## Negative Cases

Unknown principal, over-ceiling request, unsafe file path, invalid replacement, nonexistent revision, stale claim, and provider failure remain explicit. An allowed governance decision alone does not imply external success.

## Compatibility

Existing synthetic `git.read` reference paths remain labeled as synthetic. Canonical ClaimRequest, SandboxClaim, authority, and evidence contracts remain backend-neutral and unchanged except additive policy tool-ceiling provenance.

## Open Decisions

The local-only host provider is a demo seam, not a production credential architecture. A general installed Tool Backend remains separate E16/E14 work.
