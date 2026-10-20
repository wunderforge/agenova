# Sydney EKS infrastructure

This root manages the existing EKS installation and its shared network, ECR, IAM,
logs and budget. Its physical names and Terraform labels retain `demo` for state
compatibility. The active application is the Google-authenticated member service.
The VPC/subnets are also dependencies of production RDS; never destroy this root
as a nightly shutdown procedure.

Use an authorized non-root AWS identity in account `931228356546`, Sydney.
Keep the existing private tfvars and state; do not copy example inputs over them.

```sh
aws sts get-caller-identity
terraform -chdir=infra/production/eks init
terraform -chdir=infra/production/eks validate
terraform -chdir=infra/production/eks plan -out=review.tfplan
```

Review every proposed change before an explicitly authorized apply. Saved plans
can contain secrets and belong only in the protected operator checkout. The
`expires_at` tag is not an automatic deletion mechanism. Public API CIDR drift and
manually managed EKS access must be reconciled deliberately, never during a move.

`network.tf` retains the already-installed strict network-policy CNI add-on. Apply
[network controls](../../../deploy/production/eks/network-policies.yaml) before
running workers; use the accompanying verifier with the selected kubeconfig.
Node IAM does not grant model access; Bedrock uses the gateway Pod Identity role.

See [state migration and lifecycle boundaries](../README.md) and
[member application configuration](../../../deploy/production/member-service.md).
