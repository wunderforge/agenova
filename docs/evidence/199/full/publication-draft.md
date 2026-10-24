# Proposed follow-up issue (not published)

Title: [E8] Reconcile retained upstream allocations after claim expiry during controller outage

Parent: #12. Related: #51. Source: #199 full fault campaign on main 22355d5. Suggested severity: P2 operational resource cleanup; no demonstrated credential disclosure or authority bypass.

When the dedicated Agent Sandbox controller is absent before the first allocation, a reference Work expires without observing a backend identity. Its upstream SandboxClaim remains. Restoring the controller later assigns a real sandbox/Pod to that retained claim while the Agenova Work is still Expired. The adapter avoids destructively deleting unknown workers, but release remains unconfirmed and requires operator cleanup.

Reproduce with harness/security/controller-timeout.py on the explicitly selected disposable kind cluster. Evidence: full/cold-expired.evidence.json, full/current/cold-late.stdout and full/raw-current.tar.gz (cold/). Provider attempts remain zero, and no work-start authority is demonstrated for the late worker.

Acceptance proposal: preserve fail-closed authority and identity-safe deletion; reconcile retained allocations to confirmed release or explicit actionable unknown cleanup evidence; prove real kind timeout/restore cleanup without creating new execution authority; retain backend-neutral contracts. E15 reports only and does not implement the repair.

# Proposed PR (not published)

Title: test: verify component failure invariants on reference kind

Linked ticket: Refs #199. Task packet: work/0199-provider-failure/task.md.

This runs the full reference-kind fault matrix: real Provider stop/pause, queue expiry/resume, Running worker deletion, controller outage, and application control-plane Pod restart. Fresh images from main 22355d5 reproduce no fabricated success or authority growth in measured windows. Three supplementary real-backend Service probes reject Model/Tool calls after Succeeded/Failed/Expired with zero Provider count delta.

The report explicitly retains a P2 cleanup gap after cold allocation expiry, reference-session history loss after restart, and limits around universal fallback instrumentation and unconfigured mutating tools. No product code changes. Changed boundary: None. Deferred non-goal: EKS and optional durable member-mode certification.

Validation: installed experiments, exact Provider/journal correlation, real terminal probes, retained-evidence verifier, focused command compile, full -All (56 browser smoke tests), final Docs and exact-ID cleanup. Full report: docs/evidence/199/full/summary.md. Insert approved follow-up issue URL before publication. Do not claim all invariants passed or close #199 before the evidence/follow-up acceptance is agreed. The linked-ticket CI rule currently requires automatic-closing keywords; request maintainer handling rather than changing it or making a false completion claim.

# Proposed #199 comment (not published)

The full reference-kind experiments have now run on rebuilt main 22355d5. Provider stop gives Failed; process freeze and queued deadline give Expired; queued Work resumes after recovery; deleting a Running worker gives Failed; controller restoration resumes waiting Work; restarting the application control-plane loses reference-session records (404/unknown), then new Work succeeds. Real-backend supplementary probes reject six terminal Model/Tool replays without a Provider count increase. Full gate passed with 56 browser smoke tests. One P2 retained-allocation cleanup gap is documented with a proposed E8 follow-up. EKS and optional member-mode durability were not tested, no product logic changed, and the report does not label unknown or unconfigured cells as passes. Evidence/PR/follow-up URLs will be inserted after approved publication.
