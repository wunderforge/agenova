# Verification — 2026-10-30

## Real AWS migration

- STS: designated `agenova-demo-operator`, account `931228356546`.
- Five S3 objects compare byte-for-byte with private original state backups;
  [metadata/counts](migration.json). No infrastructure apply or refresh was run.
- `init -migrate-state` initially recreated lineage/serial while preserving
  resources and outputs. Under an exclusively created S3 lock, compared remote
  resources/outputs to the original backup, then restored the exact backup bytes
  using conditional `PutObject` with the read ETag. Readback was byte-identical.
  This avoided losing original metadata; S3 versioning retains migration history.
- Every original source checkout now has the same backend and was initialized
  with `-reconfigure`; state pull matches original lineage/serial/resources/outputs.
- Bucket [readback controls](backend-controls.json): private, versioned, AES256,
  BucketOwnerEnforced, deny insecure transport.
- Controlled lock contention: exclusively created an S3 `.tflock`; actual
  `terraform plan -input=false -lock-timeout=0s` failed with
  `Error acquiring the state lock`. Released only the test-owned lock.
- Existing SSO administrator role has AWS `AdministratorAccess`; no permission
  changes made. Local `agenova-sso` STS failed: expired token and refresh failed.
  Successful teammate SSO use remains unverified and requires their own login.
- Read-only final check: RDS `agenova-production` available; EKS clusters `[]`.

## Local gates

- Python unittest discovery: 6 tests pass (plan direction/scope and backend guard).
- Recursive Terraform formatting: pass.
- Terraform validate and backend/identity guard: pass for all five roots.
- Full repository gate initially stopped for missing frontend dependencies after
  Go tests/vet and integration compile checks passed. Final rerun recorded below.

## Limits

No infrastructure restoration, shutdown cycle, application deployment or UI smoke
was performed. Latest main is the future UI deployment baseline; it is not running
at the absent application endpoint. Historical state has not been refreshed for
cleanup drift. First restore needs reviewed networking/DB ingress reconciliation.
State, tfvars, saved plans and full private logs remain in `.tmp/shared-state`, not
in this evidence packet. No CI files changed. Automatic deletion remains cancelled.

## Final repository gate

`pwsh -NoProfile -File ./scripts/check.ps1 -All` rerun: exit 0.
Go tests/vet, contract/component/type/build gates and all 90 browser smoke tests
passed. The local run used Node 26.7.0 (npm warned that package engines specify
Node 24); use the documented Node 24 toolchain for supported team/CI execution.
These local UI tests do not establish an online deployment.
