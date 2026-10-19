# Design: Reuse the installed member boundary

Use `internal/authentication` and `internal/console.AuthenticatedHandler` as the single member entry point. Google verification occurs during login; the service issues revocable sessions and checks current membership for each operation. `internal/console` passes the resulting `Principal` to `app.PrepareAssignment`; M1 Policy remains the admission authority. Existing connected CLI and Portal use the member session transport, and the evidence service enforces ownership below the HTTP handler.

The focused test composes the production boundary with two deterministic identities and memberships. It asserts Policy input/issued principal, forged-team resistance, owner-only evidence, and zero backend/provider calls after invalid authentication. It does not claim a real Google login or installed-kind result. The old `internal/identity` verifier and separate OIDC installation mode stay off this branch to avoid competing authentication paths.

Risk: #203's direct JWT/JWKS and token-file wording differs from the approved Google/member delivery. Record the transport choice in the Ticket and run real two-member acceptance before M2 merge.
