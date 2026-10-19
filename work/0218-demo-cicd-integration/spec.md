# Integration acceptance contract

Ticket [#218](https://github.com/wunderforge/agenova/issues/218); elaborates existing PRD authority/evidence/member behavior.

An authenticated member submits a canonical request. The serving installation selects scoped Policy/template, resolves authority, derives a bounded catalog, dispatches through the existing Gateway and persists canonical evidence. No trusted identity or endpoint is taken from Work input.

Configured tools report configuration, not health. Broken configured tools never fall back to mocks. Provider/recording failures remain distinguishable; terminal/cross-context/unauthorized attempts dispatch nothing. Updated CLI/UI understand additive result fields; old evidence remains readable.

The candidate includes Control Plane, Worker and clients. External MCP deployment, credential provisioning and OTEL/Collector/backend belong to their Owners. No new hostile-worker/network-isolation or generic Secret Store claim is introduced.
