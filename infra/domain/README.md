# Permanent demo-domain resources

This independent Terraform root manages the ACM certificate for
`agenova.app` and `*.agenova.app` in `us-east-1` (CloudFront's required certificate region) and
its DNS-validation CNAME. It reads the owner's existing public hosted zone
`Z00685831HNHNRLEZ9VYK` as a data source. It does not create/manage the zone or
register the domain, and must not be included in `infra/demo/aws` teardown.
Certificate and validation record have `prevent_destroy` protection.

Use `AWS_PROFILE=agenova-demo` and an authenticated non-root operator:

```sh
terraform -chdir=infra/domain init
terraform -chdir=infra/domain plan -out=domain.tfplan
terraform -chdir=infra/domain apply domain.tfplan
```

The current console-created apex/wildcard certificate and CNAME were imported:

```sh
terraform -chdir=infra/domain import aws_acm_certificate.demo arn:aws:acm:us-east-1:931228356546:certificate/146c07de-d961-43f3-a291-d27bd552c410
terraform -chdir=infra/domain import 'aws_route53_record.validation["agenova.app"]' 'Z00685831HNHNRLEZ9VYK__c2bcda06481a5cdcdce022a8459d61c1.agenova.app_CNAME'
```

Do not rerun imports when state already contains these resources. State is local
and ignored by Git; preserve/back it up securely. Only this independent root owns
these resources. The lock file is committed.

By default apply establishes the validation records without blocking on domain
registration/public DNS propagation. Once public delegation is live, apply with
`-var=wait_for_certificate=true` to wait for issuance (five-minute timeout).
`PENDING_VALIDATION` is not HTTPS readiness. No CloudFront distribution or traffic
alias is defined yet; the custom demo URL is not deployed by this configuration.

The apex and wildcard share one ACM validation CNAME; Terraform manages it once.
The Owner deleted the old demo-only certificate; its historical CNAME remains
managed as `aws_route53_record.legacy_validation`. The current certificate is ISSUED.

The direct-ALB deployment additionally uses `aws_acm_certificate.sydney` in
ap-southeast-2 with the same apex/wildcard names. Its validation uses the shared
account/domain DNS CNAME; issuance was verified. Keep both regional certificates
when destroying the disposable `infra/demo/edge` ALB stack.
