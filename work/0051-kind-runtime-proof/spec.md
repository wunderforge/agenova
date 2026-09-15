# Feature Specification: Real kind runtime proof

- Ticket: [#51](https://github.com/wunderforge/agenova/issues/51)
- PRD outcome: `docs/product/prd.md` §3 and final demo runtime step.

Given one admitted resolved claim, allocate exactly one observed Agent Sandbox worker and report its neutral identity. Ready is infrastructure evidence only. Start triggers and acknowledges real work on that worker, and the deterministic task yields an inspectable result correlated to claim/worker. Terminate establishes stop/cancellation separately from Cleanup; Cleanup confirms resources absent. The run service—not this adapter—owns work outcome. Filesystem is BackendVerified only with a real inside/outside probe, else Unsupported.

Negative cases: wrong worker, duplicate/premature Start, Start after Terminate/Cleanup, failed stop acknowledgement and failed release confirmation never silently pass. Do not change public authority contracts, RuntimeBackend, or gateways. Any worker-control channel is adapter-owned, not falsely attributed to upstream native semantics.
