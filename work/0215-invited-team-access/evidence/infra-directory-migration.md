# Infrastructure directory migration — 19 October 2026

Owner requested production paths, preserving Terraform state and RDS, with no
resource recreation. This change does not rename cloud resources or grant access.

## Executed evidence

- Operator identity verified: account 931228356546, agenova-demo-operator (non-root).
- Complete old operator roots were privately backed up before moving them.
- Live local state now resides in `/Users/neoliu/projects/agenova/infra/production/eks`
  and `/Users/neoliu/projects/agenova/infra/production/edge`. PR checkout has no copy
  of these live states. Private vars, lockfiles and provider cache were preserved.
- EKS state SHA256: `dff1a7449f707350ef28e69b1cc2ef09d294067203df5bedb0f2e2b4cf5160a9`;
  lineage `1845063a-7c56-473b-727a-8f5b9d57ac75`, serial 190, byte-identical after move.
- Edge state SHA256: `bc46fe5c31d78cdc0eb1e767d26bd5853a031a5b80b806542c9ef6768c5208f8`;
  lineage `3caf5ca8-df14-2731-a69e-781771f7c5cb`, serial 73, byte-identical after move.
- Saved `plan -refresh=false` before and after: parsed `resource_changes`,
  `output_changes`, `resource_drift` and `planned_values` were exactly equal.
  Edge has zero changes. EKS has 19 pre-existing in-place differences, including
  expired ExpiresAt input, public API CIDRs and launch-template-related updates.
  They were NOT applied. Identical plans prove no migration delta, not zero drift.
- Both moved operator roots passed `terraform validate`; both source roots passed
  `terraform fmt -check`. Existing local network.tf was preserved in source control.
- Database and member-edge state hashes are unchanged. RDS was available with
  resource ID `db-SE47U2GGUZZDSVSN7PZB2Z7KVI` before and after migration; app login returned HTTP 200 afterward.
- No Terraform apply, destroy, import, state rm/mv or cloud mutation was executed.
- Obsolete saved plans are retained only in the protected backup, not reused.
- Removed the retired shared-password proxy deployment code; archived its runbook.
  Existing local strict-CNI policy and verifier files moved to deploy/production/eks.
- Midnight automation paths were updated without changing its time or preservation
  rules. Whole-root destruction remains forbidden because RDS shares the EKS VPC.

Private raw plans/config backups and machine comparison are in the operator's
`.tmp/infra-migration-20261019` under the PR worktree, excluded from Git. They can
contain sensitive infrastructure configuration and must not be published.

## CI

Pre-migration head 939e141: GitHub run 39232189020 passed baseline, Linux CLI and
Windows CLI. The earlier failure was PR-body format validation, corrected without
disabling checks. Migration final gate and updated-head CI are recorded in the PR.

Final local migration gate: `pwsh -NoProfile -File scripts/check.ps1 -All`
passed (Go vet/tests, documentation/contracts, 125 frontend unit tests, production
build and 56 browser smokes). `git diff --check` and PR-body contract passed.
