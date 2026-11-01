# Application credential recovery gate

Date: 1 November 2026, Australia/Sydney. Status: owner explicitly authorized the
exact OAuth/RDS reads, dedicated runtime login, single retained Secrets Manager
record and member-edge state for this recovery. Implementation/testing precedes
sensitive reads and creation. PR #263 administrator
merge is authorized after release readiness and all gates; it is not merged yet.

## Verified starting point

- EKS, nodes and ALB are absent; RDS `agenova-production` is available and deletion
  protection is enabled. No infrastructure apply has been performed this round.
- Secrets Manager in Sydney contains only the existing RDS managed administrator
  Secret. Parameter Store has no parameters. No application credential backup has
  been found; owner reports no known backup.
- The exact previously supplied OAuth download exists at
  `C:/Users/dapen/Downloads/client_secret_1046037255259-4do6nbtiers5j46naul3ktvjatbv4dla.apps.googleusercontent.com.json`.
  Only its existence was checked in this recovery step; contents were not read.
- The locked EKS plan creates only the expected nine runtime addresses. Sixteen
  retained state resource identities were captured as hashes; RDS is untouched.
  Binary plans were removed and must be freshly generated before apply.
- PR #263 source and all three actual image checks pass at `a442e94`.

## Proposed smallest repeatable recovery

1. Read only the named OAuth file in a controlled program. Validate Web-client
   shape and configure the existing authenticated callback; do not silently edit
   Google configuration, change caller identities or publish client secrets.
2. Read only RDS managed Secret
   `rds!db-16fd8654-0050-42b0-a6df-12ec85ea7c74` for a short-lived trusted initializer.
   It never enters the running control plane or an Agent image/Pod.
3. Create one dedicated database login `agenova_runtime_recovery`, inheriting
   only the inspected existing `agenova_runtime` permissions. Assert that the
   inherited role has no administrator/DDL/audit-update/fact-update powers before
   granting membership. Do not reset any existing account's password, alter Neo's
   role, rerun Owner bootstrap, migrate schema or modify business records.
   Assert the new login cannot create roles/databases, bypass RLS or become superuser.
4. Generate a fresh runtime password and 32-byte application session key. Persist
   the runtime URL, Google configuration and session key as one encrypted Secrets
   Manager record `agenova-demo/member-service`, independent of EKS lifecycle.
   Existing browser/CLI sessions may need normal Google sign-in because the old
   session-encryption key has no known backup. Existing application users/grants
   are retained; no replacement Owner is created.
5. Rehydrate only the trusted member service's Kubernetes Secret from that
   record using the authorized operator. Agent Pods have no access. No new CSI
   driver, runtime IAM permission, public Secret endpoint or additional node is
   needed for the minimal operator-driven recovery path.
6. Use the existing `member-edge/terraform.tfstate` backend alongside the already
   authorized EKS/edge roots to restore the existing `app.agenova.app` route.
   Plans must explicitly enumerate only owned runtime/entry changes; preserve
   database, hosted zone, certificates and Neo's access. Do not read the database
   or domain states or apply a broad destroy.
7. Verify role/credential provisioning, temporary-resource cleanup, application
   login, real cloud model and tool calls, all three permission modes and actual
   denied calls. Save sanitized evidence before closing runtime resources.
   Retain the one application Secret and recovery configuration for tomorrow.

## Bounds and failure handling

- New paid retained resource: one Secrets Manager record. Live AWS Price List
  query for `AWSSecretsManager`, `regionCode=ap-southeast-2` returned USD 0.40 per
  secret-month and USD 0.000005 per API request (USD 0.05/10,000), before tax.
  No automatic rotation Lambda, customer KMS key, PVC, database or extra node.
- Reuse the existing master credential; do not rotate it. Do not show/store
  plaintext credentials in chat, Git, Terraform state, images, CLI arguments,
  local files, Agent Pods or ordinary task logs.
- Use protected in-memory handoff and labelled temporary trusted objects. Check
  ownership before cleanup. Failed writes are not absence: inspect the exact
  intended object/version and stop rather than overwrite unknown credentials.
- If the new login/Secret already exists without matching ownership/provenance,
  stop. Keep a partially provisioned credential recoverable; do not regenerate a
  new password on ambiguous retries or delete another operator's objects.
- Failures before release readiness leave compute off. Startup deadlines/plans
  must be refreshed after any approval wait. No cleanup deadline is represented
  as an AWS spending cap.

## Practices and evidence boundaries

[AWS EKS external secrets](https://docs.aws.amazon.com/eks/latest/userguide/manage-secrets.html)
supports keeping credential authority outside the disposable cluster. This
minimal recovery uses the operator to restore a Kubernetes Secret rather than
installing a CSI driver for this demo.
[AWS Secrets Manager pricing](https://aws.amazon.com/secrets-manager/pricing/)
documents the storage/API model; the Sydney amounts above were also verified
through the live AWS Price List API.

This is a recovery plan, not a claim that credentials were read,
a database login created, a Secret written, infrastructure started or deployment
completed. Original file existence and Google authorization-entry validation are
confirmed; RDS credential access and database/cloud mutation have not occurred yet.
