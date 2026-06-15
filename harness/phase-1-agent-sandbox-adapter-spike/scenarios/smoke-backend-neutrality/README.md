# Scenario: Backend Neutrality

This scenario protects the Phase 1 pluggability rule.

Expected evidence:

- `RuntimeBackend` is documented as the boundary between Agenova's stable contract and concrete sandbox substrates;
- the in-memory backend is the reference backend and contract test oracle;
- Agent Sandbox is one adapter target, not a product-level hard dependency;
- future backend adapters can satisfy the same contract without application-facing API changes.

