# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: 2026-08-31T08:02:51Z
- Branch: neo/e8-t3-kind-agent-sandbox
- Commit: 009e7e34dec65f52e27376ce0ab460071f8e8f41
- Command: reproduce.sh all
- Pinned Agent Sandbox version: v1.0.0
- kind: /opt/homebrew/bin/kind (existing)
- kubectl: /opt/homebrew/bin/kubectl (existing)
- kind cluster / context: agenova-k8s-lab / kind-agenova-k8s-lab
- Result: pass

## Notes

- Scope: the pinned upstream Agent Sandbox lifecycle only. No Agenova
  `ClaimRequest`/`SandboxClaim`, `RuntimeBackend` adapter, or contract is
  exercised here (that is E8-T4 / #51).
- Reruns are idempotent: cluster and namespace creation are safe to repeat,
  and teardown deletes only the `agenova-k8s-lab` kind cluster and the
  `agent-sandbox-smoke` namespace created by this script.
- Not isolated here: claim-only deletion vs warm-pool "recycle" behaviour
  (see #48). This harness deletes the claim, pool, and template together and
  asserts the sandbox pod count reaches zero.

Raw output: `output.txt`
