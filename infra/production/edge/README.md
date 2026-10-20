# Shared HTTPS ingress infrastructure

Owns the existing Sydney ALB, listener, default legacy target, node-ASG attachment,
security groups and legacy traffic DNS. Physical names and resource addresses are
unchanged to preserve the existing state. Domain registration, hosted zone and
certificates remain outside this root.

The active `app.agenova.app` host rule and port 31090 member target are owned by
[member-edge](../../member-edge/README.md). The retired shared-password proxy is
not deployed; its old default target is retained as infrastructure state, not a
supported user entry point. Do not run the removed demo proxy scripts.

```sh
aws sts get-caller-identity
terraform -chdir=infra/production/edge init
terraform -chdir=infra/production/edge validate
terraform -chdir=infra/production/edge plan -out=review.tfplan
```

No apply is needed for a directory migration. Review ingress dependencies before
any future change or authorized teardown; do not orphan the member host rule.
See [state preservation instructions](../README.md). Preserve permanent DNS
validation records and certificates during compute/traffic cleanup.
