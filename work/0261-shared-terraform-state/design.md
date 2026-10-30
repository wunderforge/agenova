# Shared Terraform backend design

Ticket: [#261](https://github.com/wunderforge/agenova/issues/261)

Use an account-owned S3 bucket with AES256 default encryption, versioning, BucketOwnerEnforced ownership, public access block and explicit TLS-only policy. Use native lockfiles instead of introducing DynamoDB. AWS IAM Identity Center AgenovaAdministrator already provides administrator authority; verify its policy rather than granting a new broad role. No static keys in backend configuration.

Store five roots separately: production/eks, production/edge, member-edge, database, domain. Keep existing resource addresses and lineages. Back up original private files, copy into the latest-main checkout, initialize with migrate-state and compare pulled state. Add backend configuration to old operational roots after migration so subsequent init cannot silently use stale local state.

Do not refactor persistent resources between states in this task: that is a separate, riskier migration. A plan-inspection tool validates exact ephemeral addresses and refuses deletion/replacement elsewhere; it does not claim IAM enforcement or automatically execute apply. Whole-stack destroy remains prohibited.

Evidence includes real S3 controls, original/pulled equality and conditional lock contention, isolated plan-guard tests and repository gate. Team interactive SSO and full rebuild/shutdown remain explicitly unverified until run. Terraform state can include secrets: private backups and plans use restrictive permissions and only redacted summaries enter Git.
