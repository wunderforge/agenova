# E14 team acceptance runbook (member-service path)

This runbook tracks [#206](https://github.com/wunderforge/agenova/issues/206) after the Owner's 20 October 2026 decision to reuse main's Google/member service. The old direct-JWT token files, JWKS fixture, `oidc-config-secret` reference install and token-file Portal proxy were removed from this candidate because they cannot exercise the chosen identity boundary.

## Prerequisites

Use one deployed service running the current member authentication path, with an active versioned Policy and AgentTemplate, a real runtime and model provider. Obtain two distinct **approved** Google member accounts with project access and a second teammate able to repeat the procedure from a clean environment. Record the exact service revision, Policy/template versions, runtime/backend versions and the target environment. The [member-service deployment guide](../../deploy/production/member-service.md) describes the current installation; a clean kind installation remains unproven and must not be inferred from a working EKS deployment.

## Two-user acceptance

1. Each member signs in using the current browser/native CLI flow. Confirm the service resolves their own subject, team and active membership. Do not export session credentials into Work YAML, URLs, screenshots or logs.
2. Submit two [Work fixture](work.fixture.yaml) copies whose `spec` bytes are identical; only their names differ. Confirm one installed service and one Policy/template version were used without restarting or changing service identity.
3. Compare the two issued authority snapshots. The Policy must admit both users, narrow them differently, and record stable reasons. Verify requested access cannot grant itself and no unrequested authority appears.
4. For each member, compare CLI, authenticated API and Portal evidence: principal projection, Policy ID/version, requested/effective authority, worker identity, invocation facts, outcome and cleanup. Verify a guessed other-user request or claim reference has the same not-found behavior as an absent one.
5. Exercise invalid/disabled membership, over-scope Work, cross-claim worker target and post-terminal Gateway calls. Record claim/backend/provider call counts; a denial must not cause an external call or B-attributed invocation fact. Run the installed worker check again on this integrated branch.
6. Run the focused gates and `./scripts/check.ps1 -All`. A second teammate repeats the install/reapply, two-user Work and cleanup from a clean checkout, recording an independent redacted transcript.

The focused host-token fixture is `go test -count=1 ./internal/console -run TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow`. It proves only that an allowed Work reaches a host-held credentialed provider and a denied Work causes zero provider calls. It is not installed Google, EKS/Bedrock or kind evidence.

Store screenshots, transcripts and temporary credentials outside Git. The [release check](RELEASE.md) must cite exact evidence or mark the gate pending. Clean up only resources created for this acceptance run; preserve the shared member service and cluster.
