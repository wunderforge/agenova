# Live member-control-plane upgrade — 13 October 2026

Owner explicitly authorized granting Fisher the installation duty and performing
an actual CLI upgrade, then confirmed the browser grant action. The Owner session
assigned `platform-operator`, installation `demo-eks-bedrock`. No AWS permissions
were changed in this slice.

## Real identity and authorization

- Native CLI Google login completed successfully; the local OS credential store
  was used, including user-approved macOS Keychain access after the rebuilt binary.
- A read-only diagnostic used that same OS-stored application credential against
  `/auth/me` and `/api/members`. It confirmed `fisherinaus@gmail.com`, principal
  `user:request:1b14840ce04029b7c95a7cf28a319ffc`, and the exact installation duty.
  No bearer token, Google assertion or password was printed or committed.
- AWS STS independently returned that email as the assumed
  `AWSReservedSSO_AgenovaAdministrator_43738c55e42c3fae` session, account 931228356546.
- Real CLI validation of `unassigned-installation` returned HTTP 403, exit 1.
- The initial public Homebrew preview lacks this adapter mode. Live commands used
  the newly built `.tmp/operator-upgrade/agenova`, not a claim of published release.

## Executed upgrade

Every platform invocation passed the HTTPS server and exact installation name;
Kubernetes access used Fisher's `agenova-sso` profile/kubeconfig. The adapter
performed the mutation, not an out-of-band `kubectl set image` command.

```sh
agenova platform plan -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock --state-dir operator-state --json
agenova platform apply -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock --state-dir operator-state --yes --json
agenova platform status --server https://app.agenova.app --installation demo-eks-bedrock --state-dir operator-state --json
```

Target: `agenova-system/agenova-member-control-plane`, UID
`0de32e97-8eb1-4a6f-8472-b8fbdfa1ef39`. Registry repository:
`931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane`.

- Original image: `sha256:0651bc8f78e032e0e5c6750eaa69fbe69a7db36598c3c51b1ff0647a839bc68d`.
- First candidate: `sha256:2ee6b30fbf84af4e8a51360e54a56419f25d27d29bee291754fa0b350428dd9d`.
  Deployment converged, but the adapter returned an unsupported readiness word;
  CLI consequently did not save the local pointer. This was found in live testing
  and fixed to the existing `available` state, with full-gate rerun.
- Final image: `sha256:0eba786b1519aa1b208c75d1b2aceaa8fa72aca90c170160b2a854af2b911d2d`.
  Built from the corrected source, pushed to ECR, applied through the same CLI.
- Final apply: exit 0, `applied: true`, `ready: true`.
- Status: exit 0, no target changes, member Deployment `available`.
- Reapply: exit 0, `changes: []`, `applied: false`, `ready: true`.
- Pod changed from `agenova-member-control-plane-557cf68c54-kc7qk`
  (UID `93349490-a4d4-4319-b49c-b99ee82059a3`) to
  `agenova-member-control-plane-7cf5f4d799-kt6cg`
  (UID `3c6a5221-25bd-4b2e-877d-3416ee774350`). Final Deployment 1/1 available.
- Full Deployment `spec` comparison before/after was equal after excluding only
  the control-plane image: auth, database Secret mounts, proxy and SA retained.
- No SQL migration, identity bootstrap, credential rotation or permanent resource
  creation occurred. Existing ECR repository received the two new image manifests.
  These member-service artifacts must not be added to disposable demo destruction.

## Post-upgrade application evidence

- `/login`: HTTP 200; unauthenticated `/api/requests`: HTTP 401.
- The same Fisher CLI session retained access after both restarts.
- Three previously visible Succeeded Work records matched before/after restart.
- Submitted a new model-only Work after final upgrade. CLI `run` and subsequent
  `work show` both reported Succeeded, principal matching Fisher above.
- Request: `work-c1ec0ca3c9875518a0d8b96bc4a2fc8bc199114c40507d442ef22d8479572ec0`.
- Backend `agent-sandbox`; model `amazon.nova-micro-v1:0`, 630 input / 80 output
  tokens, response ID `74bc45f5-6f79-4600-a50d-a7cb1847ca87`. This is application
  provider-response evidence, not a separately queried CloudWatch metric.

## Checks and bounds

- `go test ./internal/adapters/bundled ./internal/platformapply`: pass.
- `pwsh -NoProfile -File scripts/check.ps1 -All`: pass after readiness fix;
  55 browser smoke tests passed.
- `pwsh -NoProfile -File scripts/check.ps1 -Docs`: pass for operator documentation.
- Python binding output matched the actual Go adapter's live-accepted template hash.
- Negative unit cases cover changed UID, auth/sidecar drift, wrong rollout strategy,
  denied RBAC, runtime mismatch, changed plan and concurrent patch rejection.
- This validates image-only upgrade of this existing member installation. It does
  not prove model configuration changes, schema migration/rollback, initial SSO
  installation, zero downtime, general role campaigns or all production gates.
- Single-writer Recreate entails brief downtime. Long-running Work during upgrades
  was not tested. Existing broad AWS/EKS admin permissions remain unchanged.
