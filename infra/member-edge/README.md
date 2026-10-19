# Member validation endpoint

This Terraform root adds `app.agenova.app` to the existing Sydney ALB and EKS
installation. It creates a dedicated host listener rule, target group, ASG target
attachment, NodePort 31090 SG rules and Route 53 alias. It does not create another
ALB, replace the legacy listener action, or modify permanent certificates.

The matching Deployment/Service example is
`deploy/production/eks-member.json.example`. Use the member-service runbook and
review actual cluster names, compatible worker binding, image digest and mounted
configuration before applying. No secrets belong in this root or the manifest.

This root depends on the existing `agenova-demo` ALB, cluster, node group and VPC.
Do not destroy `infra/production/edge` or `infra/production/eks` while member validation uses
those resources. The managed RDS database also currently uses that VPC. The old
cleanup automation is paused. This dependency must be migrated explicitly before
retiring the old stack; it is not the final AUD100/month hosting topology.

For rollback, remove only this root's route/target resources and member workloads;
retain RDS and its data, domain registration, hosted zone and permanent certificate.
Do not destroy the database to roll back an application image. Schema compatibility
must be checked before running an older member-service binary.
