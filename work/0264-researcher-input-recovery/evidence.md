# Local verification

All reproduction data below is synthetic public contract data. Operational Work records remain outside Git.

- `go test ./internal/workerprotocol ./cmd/agenova-demo-worker ./internal/console ./internal/connectedclient`: PASS in the normal host test environment. Restricted-process loopback calls initially failed; the same tests pass with normal loopback access.
- New actual pipe/Exchange regression: a fixed-choice `dataset_id=customer-retention` wrapper remains invalid; explicit repair produces a new valid action and exactly one backend call. An ungranted corrected action produces zero backend calls; repeated invalid actions stop after two model calls.
- Fixed-only schema enums, mixed free-text catalogs, unknown routes and large-catalog schema fallback: PASS.
- Console classification ignores recovered validation failures, later provider failures, denials and observations: PASS.
- `npm run typecheck`, `npm test -- --run`, `npm run build`: PASS; 17 files / 192 unit tests.
- `npm run test:smoke`: PASS; 96 browser tests, including the evidence-linked invalid-input notice at widths 1440 and 390.
- Rendered proof inspected locally: `.tmp/ui-smoke/connected-terminal-invalid-09013-s-exact-validation-evidence/invalid-crm-1440.png`, matching mobile and evidence pages. Existing June notice styling and requested/granted table are preserved.
- `./scripts/check.ps1 -All`: documentation, boundaries, formatting, module checks and vet pass. Full Go test run stops at the existing Windows `TestOwnerBootstrapDeliveryGuards` invitation-file delivery failure. This is not an all-green local baseline; Linux CI and actual image smoke remain release gates.

## Release gates

Independent PR review, latest Linux baseline, actual release-image checks, and cloud task acceptance remain pending. No new infrastructure, credential read, database migration, or authority expansion is required.
