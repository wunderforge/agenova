# Feature Specification: React contract bindings and fixture evidence source

- Ticket: [#59](https://github.com/wunderforge/agenova/issues/59)
- PRD outcome: [Read-only claim console](../../docs/product/prd.md#7-read-only-claim-console), fixture-backed foundation only.

## Intent

Make the existing canonical contract cases executable in a small React storyboard. Define a replaceable evidence-source boundary and honest missing-data behavior while leaving governance semantics with the canonical contracts.

## In Scope

- Mechanically checked TypeScript JSON bindings, one fixture adapter, read-only rendering, and reproducible contract/component/browser evidence.
- Source diagnostics identify field paths and categories separately from canonical domain objects.

## Out of Scope

- Live transport/polling (#68/#60), final dashboard, policy resolution, mutation, new canonical fields, and detailed invocation/lineage schemas.

## Requirements

- Given canonical Go JSON types, when bindings are generated or checked, then all reachable fields, enum literals, optionality and custom wire encodings are accounted for. Unsupported shapes and stale output fail the check. Duration is its canonical string wire format, not a Go integer duration in the UI.
- Given a manifest case ID, when the fixture source loads it, then it reads the original manifest input directly. It may parse YAML at the adapter boundary; it may not maintain duplicate payload files.
- Given a source injected into a React component, when it returns a request or issued snapshot, then domain data uses canonical bindings. A narrow asynchronous lookup interface may return data, not-found, or invalid-data with diagnostics; these are transport/display states, not new governance outcomes. Fixture enumeration belongs in the storyboard adapter/controller, not the future HTTP contract.
- Given `issued-state.valid.team-a-engineer`, then render Allow, Running, issued authority, backend/worker identity, ClaimRunning, and the explicitly empty invocation lists.
- Given `issued-state.valid.team-b-denial`, then render Deny and its reason, with no claim, grant, backend or lifecycle invented. Absence permitted by denial is different from missing required data.
- Given the canonical ClaimRequest YAML/JSON pair, then render the same requested intent with no assertion of a corresponding issued claim. Do not join Team B evidence to Team A's request by assumption.
- Given a missing required field, unknown contract field, unsupported enum, or invalid caller-issued state, then render a diagnostic with category/path and withhold any validated-snapshot presentation. Known safe fields may be shown only clearly marked incomplete/invalid. Never default to Allow, Running, or granted requested access.
- Given omitted/null evidence arrays, then preserve a visible source diagnostic even where the canonical parser normalizes to an empty slice; do not describe missing observations as observed zero invocations.
- Given arbitrary keys inside the canonical free-form task input map, then preserve their permitted JSON semantics; they are not unknown contract fields.
- Given an unknown case ID or a source failure, then show unavailable/error explicitly. A test substitute must exercise this without fixture imports in components.
- Given shared valid and invalid inputs, then check their outcomes against canonical Go parsing and manifest expectations. Caller-invalid issued-state inputs must use ParseCallerIssuedState; trusted positives use ParseSystemIssuedState. Origin cannot be selected by a payload field.
- Given a deliberate binding drift, then the repeatable contract check fails before a build can be accepted; restore the binding and prove it passes.

## Negative Cases

- All five invalid ClaimRequest cases and three caller-invalid IssuedState cases from the manifest.
- In-memory derived missing required field, unknown field and enum cases with recorded base IDs and mutation paths. Derivatives are test scenarios, not newly maintained fixtures.
- Deny/ApprovalRequired must never gain a claim or authority from request data, field defaults or display fallbacks.
- Raw invalid payloads, including credential-like values, must not be dumped into rendered diagnostics.

## Compatibility

- No change to canonical Go semantics, fixtures, field names, enum vocabularies, normalization rules or request/authority separation.
- The five requested bindings include the existing IssuedState envelope and transitive types rather than introducing a UI evidence aggregate with different meaning.
- No invented completed/failed/expired fixture snapshots: the currently available issued positives are Allow/Running and pre-claim Deny. Synthetic enum/negative tests must be labeled as such.
- The current evidence contract has only minimal invocation views; do not infer unsupported outcome, lineage, timestamps or detailed calls.
- This source interface enables later HTTP replacement without claiming transport compatibility with an endpoint that #68 has not yet delivered.

## Open Decisions

- Task + Spec at 3504988 were explicitly approved by the user on 2026-09-08; approval was recorded on #59. No open implementation decision. Canonical Go validation occurs at fixture build time; runtime validation checks display shape only, and later HTTP integration must supply its own trusted validation boundary.
