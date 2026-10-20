# Work-scoped public networking

Feature contract and implementation provenance: [#243 packet](../../work/0243-work-network-authority/task.md). Related MCP acceptance: [#242](../../work/0242-mcp-incident-demo/acceptance.md).

## Permission is separate from the image

An Agent may implement its own HTTP client or framework tools. That does not grant network access. Omitted `requestedAccess.networkProfiles` grants none. A submitter can explicitly ask for:

```yaml
requestedAccess:
  networkProfiles: [public-web]
```

The same profile must be present in the immutable AgentTemplate ceiling and in an explicit Policy network constraint:

```yaml
constraints:
  networkProfiles:
    values: [public-web]
```

These are additions to complete canonical documents, not standalone apply files. The request, template and Policy must all agree. A Policy ceiling that includes public-web never adds it to a Work that did not ask. Unknown/duplicate profiles and unapproved requests fail closed.

The initial profile permits public IPv4 TCP 443 and necessary cluster DNS. It is broad Internet permission, not an HTTP-read-only, domain allowlist or TLS-inspection guarantee. IPv6 and direct public DNS are not added by this profile. Tool/model permissions and credentials remain separately governed.

## Runtime prerequisites

The Agent Sandbox adapter requires an operator-verified startup-isolating substrate, selected in its existing backend configuration:

```yaml
config:
  connection: {mode: in-cluster, namespace: agenova-system}
  network-policy-mode: strict
```

This is a capability declaration, not proof. Verify the actual CNI and fresh-Pod first-packet behavior with reachable independent controls. A worker self-check, a sleep, an idle warm pool or a NetworkPolicy YAML object is not sufficient. Unsupported environments must not silently run a governed Work permissively.

The opt-in governed Agent Sandbox integration tests likewise require
`-verified-strict-cni` before contacting a cluster or model. This is an operator
attestation, not independent packet evidence; do not use it with ordinary kind
networking merely to bypass the prerequisite.

Managed templates explicitly deny network access instead of inheriting upstream public-by-default rules. Warm workers have no standing Internet grant. A running Work receives only its own temporary network rule, selected by trusted runtime identity rather than reusable template name. Native egress authority carries an absolute backend deadline. On release, a trusted substrate destroys the worker and verifies absence instead of trusting an arbitrary image's stop acknowledgment.

## Evidence and limits

Requested/effective network profiles are visible in the shared CLI/API/Portal evidence. Runtime facts distinguish activation/revocation attempts, success and failures; their reason identifies Configured or Simulated evidence. Configured means a rule was installed through the declared backend, not that a packet probe passed. In-memory behavior never proves OS/CNI isolation. Raw dropped packets are not fabricated tool calls.

Historical records without network enforcement evidence remain historical/unknown. They do not prove that old workers were offline. Policy/controller changes are asynchronous; release latency and existing-connection termination require actual backend measurement. Backend expiry bounds application-control-plane outage, subject to the substrate controller's reconciliation and availability; this is not instantaneous revocation during all infrastructure outages.

## Upgrade and recovery

- Preserve the existing worker protocol, images, authentication, model and tool configuration.
- Drain active Work before shared rollout. Replace/drain legacy warm pools and remove only their verified obsolete allows; a new deny does not override an old additive allow.
- Adopt the versioned isolation-generation runtime resources. Do not repoint old registered template contents or silently widen active claims.
- Grant namespace-scoped network-rule permissions and guarded Pod patch to the trusted control plane. Pod patch binds the grant to the original Pod UID and claim label; replacement Pods do not inherit it. Workers receive no Kubernetes token or management permission.
- Verify login, old-compatible worker execution, governed model/MCP calls, evidence/history and observability after deployment.
- Do not roll back to open egress to make a failed task pass. Preserve failed cleanup evidence and require explicit reconciliation when resource identity cannot be verified.

The implementation/deployment packet owns actual validation results. This guide describes the feature contract, not a claim that a particular cluster has passed its security campaign.
