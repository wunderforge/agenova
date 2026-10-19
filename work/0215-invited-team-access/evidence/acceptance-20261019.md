# Acceptance continuation — 19 October 2026

## Verified

- Main CI is unchanged. Head a9eeb26 baseline run 39234657466 passed; earlier native
  Linux/Windows runs remain historical package evidence, not required workflow additions.
- Actual EKS, namespace agenova-system, compatible worker digest
  `sha256:ec74e41c6348b4e4d10d4da3ab1ee0561d44c1e05964487adb2cc4a9c7b4ca9e`:
  `TestControlledRuntimeBackend_Recovery` passed in 40.24 seconds.
  A unique test worker ran a real child task. A new adapter with empty in-memory
  bookkeeping reattached solely for cleanup. Start was rejected, premature cleanup
  was rejected, and the running worker's status/result remained unchanged. Terminate
  acknowledged stopped; independent Kubernetes reads confirmed claim, Sandbox and
  Pod absence. Unique test pool/template were removed. No production Pod was restarted.
- Disposable PostgreSQL 16: uncached race-enabled tests passed for console,
  workstore, authentication/postgres, invitations and control-plane packages.
  Includes own-evidence isolation, concurrent admission, writer fencing, session
  logout cancellation, grant/membership revocation, durable restart and uncertain
  provider-outcome non-replay. No production database or member grants were modified.

## Reproduce EKS adapter recovery

Use an explicitly authorized cluster context and compatible immutable worker image:

```sh
go test -v -tags 'integration controlled' -run '^TestControlledRuntimeBackend_Recovery$' \
  -timeout 6m ./harness/integration/agentsandbox -args \
  -kube-context <context> -namespace <namespace> -controlled-worker-image <image-digest>
```

The test creates only uniquely named resources and stops/releases its own worker.
It does not create clusters or modify existing member Work. Kubernetes access and
Agent Sandbox v0.4.6-compatible CRDs/controller are prerequisites.

## Explicit remaining gaps

This proves real worker reattachment/stop/release, not abrupt control-plane process
termination with PostgreSQL and Bedrock in flight. SQL crash boundaries use a
synthetic provider; they cannot prove zero real Bedrock replay on their own.
Owner declined the live Fisher revocation test. No teammate account was disabled,
no session fabricated, and no fresh two-Google-user campaign is claimed.
Schema-compatible live rollback, independent reviewer approval, real Windows/WSL
Google login and budget/availability acceptance remain open. Ten-day continuous
operation is now authorized until at least Sydney October 29 20:57 AEDT; this does
not prove the AUD100/month long-term topology is feasible.

Private transcripts: `.tmp/acceptance-20261019/eks-recovery.log` and `postgres.log`.

## Portal cleanup validation

Full repository gate passed: Go checks/tests, 125 frontend unit tests, build and
56 browser smokes. The global Project/Template controls are removed; explicit
scoped URLs still return the scoped configuration. Existing Work submission fields
and server permissions are unchanged. The live member Deployment was updated only
after confirming there were no active SandboxClaims, using an optimistic image patch.
No schema or configuration migration. New immutable image: `931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane@sha256:282d9e32e73a87f562088c28a35490e7b4ddca2ae3a60192b3f3075091d6b10e`.
Rollout reached ready.
