# Payment retry incident — isolated Agenova exercise

This is a synthetic but runnable payment-service example. No production
payments, customer data, or real financial operations are involved.

`v2.7` introduced a retry regression at 09:00. The request has one 5-second
total deadline. A retry must not begin when its backoff would cross that
deadline, and all attempts for one payment must reuse one idempotency key.
The current `src/retry.go` violates both rules. Run `go test ./...` to see the
regression and verify a proposed fix.

The prior `v2.6` Deployment revision passed the same checks and remains
deployable. There was no schema or data migration. The on-call runbook permits
a controlled rollback after checking the error rate, current rollout and
in-flight requests. The developer path is a reviewed code fix; the SRE path
is an immediate, reversible service-restoration action. Those are different
tasks over the same incident, not role-dependent scripted answers.
