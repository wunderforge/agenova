# Permanent invite-only identity

This Terraform root is independent of the disposable E13 stack and must never be
included in its cleanup. It creates a Cognito User Pool, separate native and web authorization-code clients (applications use PKCE), an API scope and managed login. Public signup is off;
software-token MFA is required. It creates no IAM users or member EKS permissions.
Both clients are secretless authorization-code clients; web tokens stay in the
server-side encrypted session store. Native credentials use the OS credential store.

```sh
AWS_PROFILE=<designated-operator> terraform init
AWS_PROFILE=<designated-operator> terraform plan -var='account_id=<account-id>'
```

Apply only as part of the reviewed #215 installation. The service must be reachable
at `public_origin` with the verified API boundary, persistent data and tested recovery
before invitations are sent. The native callback must exactly match
`http://127.0.0.1:38123/callback`. Validate that callback against the real Cognito
configuration before release. No user, password, invitation or application role is
managed by this root. The User Pool has both service-side deletion protection and
Terraform `prevent_destroy`; use an encrypted, access-controlled remote state for
production, with independent backup and recovery ownership.

Cognito groups are not application authority. Current PostgreSQL membership and
project grants control access. Logout revokes the refresh-token family's application
access through durable tombstones; short JWT expiry is not treated as immediate
revocation. Native/web login uses separate clients and five-minute access tokens.

The default Cognito email sender has managed quotas. Verify invitation delivery and
sender suitability for the actual team before launch; moving to SES requires verified
sender configuration. User-pool authentication does not imply Google federation:
Gmail addresses can be invited as personal Cognito accounts.

Status: configuration validated and planned only. This root has not been applied;
there is no live production identity service or invitation from this change yet.
