# M5 candidate design

Reuse the current Google/member service for real user identity and owner-scoped evidence. Reuse M1–M4's Policy evaluator, per-user authority caps and claim-bound worker Gateway credentials. The candidate adds a controlled host-token provider test and an acceptance runbook; it does not create another identity provider or deployment mode.

The original clean-kind direct-JWT plan no longer matches the selected identity transport. A current-member-service clean-kind path and two real user accounts are required before #206 can be accepted. EKS and IAM remain separate backend/deployment validation.
