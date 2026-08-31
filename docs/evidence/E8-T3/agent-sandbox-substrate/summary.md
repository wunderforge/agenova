# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: 2026-08-31T09:07:37Z
- Branch: neo/e8-t3-kind-agent-sandbox
- Commit: 0b602fed2248ba215100f9a41ec65472cf70e09e
- Command: reproduce.sh all
- Pinned Agent Sandbox version: v0.4.6
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
