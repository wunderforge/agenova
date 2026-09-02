# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: 2026-08-31T11:43:01Z
- Branch: neo/e8-t3-kind-agent-sandbox
- Commit: adf4d76148226c1d69d6affa7805cbbf4410b462
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
  `teardown` uses `--ignore-not-found`, and `down` deletes only the
  `agenova-k8s-lab` kind cluster. `smoke` leaves its fixtures in the
  `agent-sandbox-smoke` namespace; `teardown` removes them.
- Not isolated here: claim-only deletion vs warm-pool "recycle" behaviour
  (see #48). The `teardown` phase deletes the claim, pool, and template
  together and asserts the sandbox pod count reaches zero.

Raw output: `output.txt`
