# #187 live role-action evidence (22 September 2026)

The scenario and requester identities are local reference fixtures. Ollama model calls, Agent Sandbox workers in kind, Tool Gateway decisions, GitHub PR creation and the isolated Kubernetes rollback were observed live. The role is fixed by the operator who starts each loopback service; this is **not** a production login or a single multi-user API. The [recording guide](../../../deploy/reference/demo/role-rehearsal/README.md) names each Portal route and its interpretation.

| Work (loopback API) | Principal team | Effective tools | Real model outcomes | Tool provider attempts | Result |
| --- | --- | --- | ---: | ---: | --- |
| `incident-dev-pr-allowed-r9` (8091) | `payments-development` | `git.read`, `github.pr.create` | 6 | 4 | Initial model-authored source failed tests; model revised it; provider opened [demo PR #1](https://github.com/wunderforge/agenova-payment-incident-demo/pull/1); Work `Succeeded`. |
| `incident-dev-rollback-denied-r2` (8091) | `payments-development` | `git.read`, `github.pr.create` | 4 | 1 read only | `kubernetes.rollback` ToolDecision `Deny`, reason `tool-not-granted`; no rollback provider attempt for that invocation. |
| `incident-sre-rollback-allowed-r2` (8092) | `payments-reliability` | `git.read`, `kubernetes.rollback` | 6 | 4 | `kubernetes.rollback` provider outcome `Succeeded`, target `agenova-payment-demo/payment-api revision 3 (v2.6)`. |
| `incident-sre-pr-denied-r2` (8093) | `payments-reliability` | `git.read`, `kubernetes.rollback` | 3 | 1 read only | `github.pr.create` ToolDecision `Deny`, reason `tool-not-granted`; no PR provider attempt for that invocation. |

For both denials, the overall Work is `Succeeded` because the bounded task was to investigate and report the denied attempt. That status does **not** assert that the forbidden operation happened. The denied invocation IDs have no matching `ProviderAttempt` facts. Four opt-in live Playwright tests assert this directly through the API and rendered Connected Portal.

Independent target checks:

```text
gh pr view 1 --repo wunderforge/agenova-payment-incident-demo --json url,state,headRefOid,files
=> OPEN; only src/retry.go changed; d514b17c2d0e04fa3f0da668b894e272e9dbd370

kubectl --context kind-agenova-k8s-lab -n agenova-payment-demo get deployment payment-api -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision} {.spec.template.spec.containers[0].env[?(@.name=="PAYMENT_VERSION")].value} {.status.readyReplicas}'
=> 3 v2.6 1
```

The model-authored PR source independently passed `go test -p 1 ./...` in the final pinned, no-network, read-only, unprivileged Go container. The intentionally buggy main-branch fixture failed in that same container. The post-hardening live retests used a separate isolated service and showed two safe failures: malformed model source was rejected before a clone, and later syntactically valid but test-failing source was rejected before push. Those retests did **not** create another PR. Thus the original real PR/rollback/denial proof is valid, while a fresh successful PR run with the final hardened code remains model-dependent and is not claimed as repeatably demonstrated.

Quality gates: `scripts/check.ps1 -All` passed with `GOFLAGS=-buildvcs=false` and a workspace-local `GOCACHE` (the default sandbox cannot read Git VCS metadata); UI unit suite 127 passed; 52 standard browser smoke tests passed; 4 live role-demo Playwright tests passed. The local `api` evidence is in memory and will disappear when its corresponding service stops. PR #1 remains open, not merged.
