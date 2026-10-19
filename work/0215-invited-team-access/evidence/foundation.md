# Authentication/HTTPS foundation evidence — 2026-10-12

This is source-level foundation evidence for #215, not completed production identity or a cloud cutover.

- `go test -race -count=1 ./internal/authentication/... ./internal/connectedclient`: pass; [output](foundation-focused.txt).
- `pwsh -NoProfile -File scripts/check.ps1 -All`: pass, including 52 browser smoke cases; [output](foundation-all.txt).
- `git diff --check`: pass.

A prior full-gate run stopped because the intentional tidied go.mod/go.sum edits had not been staged (the gate compares working tree to index); they were staged and rerun. A subsequent run stopped because the new worktree lacked frontend dependencies; `npm --prefix ui ci` and browser installation resolved that setup issue. Final full gate passed. No UI source or test behavior was changed.

The private Cognito adapter verifies RS256 signature, exact issuer, client_id, access token type, expiry, issued-at and required scope, with optional resource audience. It rejects missing revocation identifiers. Tests cover forged signature, wrong algorithm/type/client/issuer, expired/missing/future timestamps, missing subject/scope/session, caller-directed key headers, JWKS rotation/outage/size limits, unknown-key refresh suppression and cancellation. Team/group token fields are not projected into application authority.

The authentication boundary requires a membership/revocation resolver on every request. A test disables membership between requests using the same token and observes denial. This proves the boundary invocation, not an implemented durable membership store or real Cognito revocation.

The connected-client HTTPS tests use trusted local TLS servers and retain certificate checks. A member client with no Kubernetes target can list evidence with a supplied token; credential failures send no request. Redirects do not send the token to a second endpoint. HTTP/non-origin/mixed Kubernetes configurations fail closed. Existing connected-client tests continue to pass.

Not delivered: durable directory/membership/session storage; API request-local identity/own-evidence integration; CLI login/profile/keychain/refresh; Cognito pool/invitations; Portal SSO; full production isolation, backup/restore, EKS two-user evidence or budget-compliant production deployment. The live demo is unchanged and the feature is not ready for members.
