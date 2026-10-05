# Fresh restore — 2026-10-05

- Restored the existing Sydney stack: 27 EKS infrastructure resources and 9 ALB/edge resources. Preserved separately managed domain and certificates.
- Platform status is ready; identical apply reports `changed: false`.
- Allowed Work `investigate-payment-retries` succeeded using `amazon.nova-micro-v1:0`; four successful model turns. Final response ID `14d449f9-6b3d-4feb-9a27-65fd62f314e4`, 828 input / 88 output tokens. See `allowed.json`.
- Denied Work `investigate-unapproved-project` has no claim or model invocation; provider attempt counter remains 4 before/after. See `denied.json` and provider observation files. Counter scope is this service process, not an AWS account-wide metric.
- All 21 HTTPS session checks passed. Root redirects to `/login`, valid login reaches connected Portal, logout revokes the session. Rendered Portal result is saved in `portal.png`.
- Reused validated control-plane, worker and session images; exact current ECR digests are in the cleanup inventory.
- E14 #197 branch contains only task/spec/design; no implementation PR was available at inspection. Shared demo credentials and reference Team A remain unchanged; this does not complete parent E13 multi-user acceptance.
- Cleanup no earlier than 2026-10-05T17:35:32Z (2026-10-06 04:35 AEDT), using operator AK profile. Automatic cleanup requires the local automation host to be online.
- Final `pwsh -NoProfile -File scripts/check.ps1 -All` passed, including 52 UI smoke tests; output in `final-all.txt`.
