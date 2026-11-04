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

## Initial migration limits (before the separately authorized cycle)

At migration completion, no infrastructure restoration, shutdown cycle, application
deployment or UI smoke had been performed; the later authorized cycle is below. Latest main is the future UI deployment baseline; it is not running
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

## Authorized real lifecycle verification

The user explicitly authorized startup, main UI verification, then shutdown.
Latest fetched main: `32b59bfac6783c2a5c3ad35c699d3dc45117f04f`.
The app code uses that main revision with the release packaging fix described below.

- Created EKS 1.35, managed node group, CNI and Pod Identity addons, operator
  access and model-gateway association after reviewing the saved plan. Restored
  existing team SSO role access to EKS; actual SSO login remains unverified.
- Reviewed and applied edge/member-edge creation plans. Retained cluster security
  group meant the targeted RDS ingress plan was a no-op; no DB migration or
  owner bootstrap was executed.
- Restored private application configuration and compatible Agent Sandbox v0.4.6.
  Strict CNI policy plus the real worker-egress verification passed, including
  trusted-canary positive connectivity and denied worker paths.
- Built/pushed main app image through trusted build-only corporate CA injection;
  no TLS bypass. Runtime image contains no injected enterprise CA.
- Found homepage 503: restrictive archive permissions propagated into Vite public
  assets, unreadable by the non-root runtime. Release Dockerfile now normalizes
  built asset files to 0644 and directories to 0755. Rebuilt and redeployed.
- Verified running digest:
  `sha256:b36425ca5eb7a5d7e74a3442648cb98dbae99ae76a6d9af1a23c100dc07e3570`.
  Corrected this cycle's newly created main-SHA image tag; retained the initial
  broken image under the diagnostic `-failed-permissions` tag. No ECR repository
  or pre-existing image history was deleted.
- [HTTP readback](http-check-fixed.json): homepage/static assets/login 200,
  unauthenticated API 401 and members redirect 303. Desktop/mobile browser
  checks found no page errors, failed resources or mobile overflow.
- Rendered evidence: [home desktop](home-desktop.png), [home mobile](home-mobile.png),
  [login desktop](login-desktop.png), [login mobile](login-mobile.png).
- Google account login and authenticated business journey remain unverified.
  User explicitly deferred that manual check until evening and requested shutdown.
- Targeted EKS destroy preview attempted to include persistent model-gateway IAM
  role/policy and was incomplete. Guard rejected it; never applied. Runbook now
  uses exact reviewed AWS IDs followed by locked reconciliation of absent state
  bindings. No force drain, finalizer removal or whole-root destroy.
- Expanded guard includes the two cluster-scoped Pod Identity resources while
  keeping IAM excluded. All 7 focused tests pass. Full repository check with
  Node 24 passed, including all 90 browser smoke tests after the packaging fix.

Private runtime/config/state backups and detailed cycle logs are in
`.tmp/terraform-plans`; none of the secret-bearing files are committed.

### Final shutdown readback

[Real AWS readback](shutdown-verification.json) confirms EKS/node group/ASG,
recorded EC2 node/root EBS, ALB, both target groups and app/demo traffic aliases
are absent. Additional active cluster-tagged EC2 and owned-volume queries returned
empty lists. RDS remains available with the same resource ID/creation time and
deletion protection enabled. Cluster, node and model-gateway IAM roles remain.

[Locked state reconciliation](state-reconciliation.json) removed only 7 EKS,
5 edge and 4 member-edge absent bindings; lineages preserved and all remaining
persistent resource records compared unchanged. Shared state is ready for a new
reviewed startup plan. Outputs/data snapshots may remain historical until that
plan refreshes them; do not use them as evidence that deleted resources exist.
The task-only Docker ECR login token was logged out. No automatic restart exists.
