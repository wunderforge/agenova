# Research MCP — local implementation, not a deployment

## Real-model release candidate

The owner authorized adapting and releasing this slice after the local
walkthrough. Follow [the release gates](../../../work/0218-final-demo-product/release.md);
this paragraph does not claim a deployed version or model acceptance.

`worker.Dockerfile` builds a separate Researcher application image with the
existing controlled-v1/stdio and real-model ReAct loop. Unlike the local preview,
it does not select tools by parsing task URLs. The model chooses each call and
receives actual MCP observations; a Researcher finish without any successful
observation fails rather than emitting an unsupported research answer. The
ordinary reference Worker image and existing templates keep their behavior.

Researcher owns its prompt separately from the reference investigation Agent:
it must not inherit a mandatory unrelated read or a success-only finish rule.
When relevant evidence/capability is unavailable it selects the explicit
`RESEARCH_TASK_BLOCKED` finish-answer control value. The Researcher exits without
a final result even if an unrelated earlier call succeeded. This preserves the
existing tool/finish and exact-answer execution contract; it is Agent outcome
handling, not semantic Policy enforcement or a fabricated Gateway refusal.
The original cloud false-success attempt remains evidence, not a passing test.
The design follows [Nova's structured-output guidance](https://docs.aws.amazon.com/nova/latest/userguide/prompting-structured-output.html)
and retains independent server-side tool validation; model choice remains fallible.

Only granted tools reach the model catalog. If a necessary tool is absent, the
Researcher must select the explicit blocked control value rather than an
explanatory successful finish; that is NOT a Gateway refusal. Out-of-catalog model actions are rejected by the existing agent/host
action boundary. Explicit Gateway probes and independent MCP zero-receipt checks
remain separate security acceptance, not fabricated model behavior.

Extends the existing Go MCP fixture; no new service, database, PVC, identity
provider or credential is required. The research tools are opt-in through
`FIXTURE_RESEARCH_ENABLED=true`. Without it, existing read_file behavior remains.

| Logical operation | MCP tool | Input | Source |
|---|---|---|---|
| web.read | web_read | url: bounded public HTTPS URL | Actual public page; no key, not keyword search |
| web.fetch | web_fetch | source_id: product-a or product-b | Fixed GitHub/GitLab Issues HTTPS documentation |
| crm.read | crm_read | dataset_id: customer-retention | Synthetic read-only crm/customers.json in the MCP data root |
| Existing git.read / repo.read | read_file | Existing allowlisted file | Preserved incident/legacy files |

Only the trusted MCP service fetches public content. The Agent receives a
bounded, untrusted observation through the Tool Gateway, never direct sockets,
SQL access, data files, credentials or ambient internet permission. HTTPS
requests have no redirects, proxy, caller headers or upload body; resolved
addresses are checked and dialed directly with normal hostname TLS validation.
NetworkPolicy is L3/L4 and does not itself enforce the fixed-source allowlist or
URL validation. `web.read` rejects private/reserved destinations and checks DNS
at dial time. The trusted service itself has public egress, not the Agent Pod.

## Local acceptance

Use [the owner walkthrough](../../../docs/development/local-product-acceptance.md).
The loopback composition on 5210 starts the real MCP child on 5216 and uses the
production Tool backend factory and Gateway. The only added cleartext allowance
is the exact credential-free http://127.0.0.1:5216/mcp endpoint; bearer credentials
cannot be sent to it. This is not an alternate production login mode.

Identity, runtime and model are synthetic; CRM is synthetic; public fetch and
MCP transport are real. No paid provider or cloud resources are used. The local
MCP child is rejected if its port is already occupied.

## Prepared artifacts — render only

- researcher.yaml: project-qualified registration candidate, standard worker
  control entrypoint. Build the existing testworker Dockerfile with the proposed
  agenova-researcher:local tag before any owned-cluster run; no image was deployed.
- researcher-v2.yaml: new immutable registration for keyless `web.read` and
  `crm.read`, scoped to research with empty direct-network authority. Keep the
  original registration/history; do not overwrite it in place.
- policy.yaml: explicit empty network authority, exact research/researcher rule.
- tool-profiles.yaml: merge fragment, not a complete Platform. Retain old routes.
- kustomization.yaml: extends the existing MCP resource set with read-only CRM
  data and research tools. Only the trusted MCP selector gains DNS/public HTTPS
  egress; Agent Pods are not selected. Existing deny policies can compose with it.

Render without contacting a cluster:

```powershell
kubectl kustomize harness/demo/research-mcp
```

Not approved or applied to kind/EKS. The existing kind installation does not
enforce NetworkPolicy. Before release, verify the actual CNI, namespace/labels,
Gateway route, compatible Worker image and real model; then compare allowed
web.fetch with a raw Pod HTTPS attempt (also when web.fetch is denied). Require
a known-reachable positive control and retained independent evidence. A timeout
alone is not attribution to infra. Do not globally remove other templates'
explicit public-web support while rolling out this Gateway-only scenario.
