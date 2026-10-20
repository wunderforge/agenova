# Design: Reuse the installed member boundary

Use `internal/authentication` and `internal/console.AuthenticatedHandler` as the single member entry point. Google verification occurs during login; the service issues revocable sessions and checks current membership for each operation. `internal/console` passes the resulting `Principal` to `app.PrepareAssignment`; M1 Policy remains the admission authority. Existing connected CLI and Portal use the member session transport, and the evidence service enforces ownership below the HTTP handler.

The focused test composes the production boundary with two deterministic identities and memberships. It asserts Policy input/issued principal, forged-team resistance, owner-only evidence, and zero backend/provider calls after invalid authentication. It does not claim a real Google login or installed-kind result. The old `internal/identity` verifier and separate OIDC installation mode stay off this branch to avoid competing authentication paths.

Risk: synthetic tests cannot establish installed two-member Google login or CLI/API/Portal parity. Run and record that acceptance before M2 merge.
