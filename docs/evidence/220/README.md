# Issue 220 evidence

Date: 2026-10-14

Base revision: `ae24a67261dff289a7063f7435c7c3616fd36dc0`

Branch: `codex/sonia-e21-bau-sre-pilot`

## Delivered evidence

- The sanitized fixture `internal/gateway/testdata/e21-resource-scopes.json` covers every accepted E21 resource kind and representative malformed or unsupported inputs.
- Parser tests prove canonical round-trip and fail-closed validation.
- Tool Gateway tests prove structural scope rejection occurs before claim attribution, policy evaluation, and adapter dispatch.
- Existing exact-authority tests plus the new parameter-widening case prove untrusted parameters cannot replace the issued resource scope.
- The existing positive control proves one allowed request records one correlated Allow and reaches the deterministic adapter exactly once.
- Existing operation/resource denial and inactive-claim tests prove zero adapter calls on denial.

## Commands and results

Focused contract and boundary packages:

```text
env GOCACHE=/private/tmp/agenova-e21-go-cache \
  GOMODCACHE=/private/tmp/agenova-e21-go-mod \
  go test ./internal/gateway ./internal/toolgateway ./internal/authority \
  ./api/v1alpha1 ./internal/app ./internal/facts ./internal/workerprotocol

PASS
```

All Go packages:

```text
env GOCACHE=/private/tmp/agenova-e21-go-cache \
  GOMODCACHE=/private/tmp/agenova-e21-go-mod \
  go test ./...
```

All packages unrelated to loopback listeners passed in the sandbox. `internal/connectedclient` and `internal/modelprovider` initially failed because the sandbox denied `httptest` binding `[::1]:0`; both passed unchanged when rerun with local ephemeral-port permission:

```text
ok github.com/wunderforge/agenova/internal/connectedclient
ok github.com/wunderforge/agenova/internal/modelprovider
```

Formatting and whitespace:

```text
gofmt -w <changed Go files>
git diff --check

PASS
```

## Sanitization

The fixture uses only synthetic identifiers (`example`, `lab`, `tracker`, `INC-123`). No company name, internal endpoint, repository, cluster, ticket body, kubeconfig, credential, token value, or secret value is included.

## Unrun gate

`pwsh` is not installed on this host. Therefore `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` and `-All`, including any frontend/browser checks performed only by that wrapper, remain to be run in CI or a host with PowerShell before M1 is accepted.

## Residual boundaries

- This is contract and deterministic adapter-spy evidence, not a real enterprise provider integration.
- E14 still owns authenticated worker-to-Gateway claim binding.
- E16 still owns real governed provider routing/transport and provider result behavior.
- E18 still owns durable evidence and observability across process restart.
- Network isolation, Agent Sandbox execution, private integration configuration, and non-production deployment remain later E21 milestones.
