# Feature Specification: Minimal two-task incident MCP demo

- Ticket: [#242](https://github.com/wunderforge/agenova/issues/242)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority).

## Intent

One engineer uses one template and fixed policy. Work A investigates payment timeouts; Work B duplicate invoices. The template permits both, but each request asks for one scope. Task prose cannot grant authority.

## In Scope

- Two disjoint full-path profiles share `repo.read` and native `read_file(file)`; labelled synthetic files contain facts, not canned diagnoses.
- Existing admission, resolution, Gateway, real SDK HTTP and local Worker executable; reference allocation and scripted inference are explicitly labelled.
- Add safe activity evidence for argument rejection without logging arguments or credentials. MCP receipts distinguish initialize, tools/call and actual reads.

## Out of Scope

- Independent shared environment changes by this lane, credentials, migrations, new public contracts, arbitrary shell/URL/SQL, UI or general connector framework. Owner-approved deployment is integrated with #243 after joint acceptance.

## Requirements

- Given the same caller/template/policy, the two canonical Work inputs resolve different resource scopes without manually editing effective authority.
- Payments lists only `payments/INC-P1042/logs.txt` and `retry.txt`; Billing only `billing/INC-B2081/logs.txt` and `job-config.txt`.
- The Worker receives its scoped catalog, not data-root mounts, provider endpoints or credentials. Real SDK calls actually read the selected files.
- Probe A -> B while Running with A's issued identity; terminal replay is separate. Platform-issued invocation IDs correlate allowed calls with independent receipts.
- The opt-in connected campaign submits ten tasks per scenario, journals every start/result, stops on unknown remote state or caller/policy drift, and requires exact real-model metadata and matching downstream receipts. It reports a mechanical screen and explicitly requires human review; safety acceptance is independent.

## Negative Cases

- Cross-scope, disguised path, terminal replay and denied admission cannot add downstream tools/call. Token failures and path escape retain existing fail-closed handling.

## Compatibility

- No public schema, policy semantics, execution contract, endpoint rules or credential placement changes. Existing E16 fixtures remain intact.

## Open Decisions

- Deployment is approved only within the integrated #243 release scope and still requires reviewed images, namespace ownership, exact control-plane selectors and verified network boundaries. Local/static evidence does not establish AWS isolation or live-model reliability. Token initialization is outside this minimal synthetic-data service; its open route must not be mistaken for complete authentication.
