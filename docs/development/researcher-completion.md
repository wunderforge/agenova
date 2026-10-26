# Researcher completion and native HTTPS diagnostics

The optional `research-v1` reference-agent execution protocol does not change
controlled-v1 lifecycle or authority. Existing images need no update. Deploy a
supporting control plane before registering an immutable new Researcher image;
do not replace existing templates in place.

## Three different outcomes

- `outcome.status` remains the execution status and existing claim phase.
- Optional `outcome.completion` is model-bound, Agent-reported task completion:
  completed, partial or incomplete, with a bounded reason/target. Old records
  omit it; absence is not proof of task quality.
- Tool decisions and runtime/network evidence remain independent facts. A model
  cannot declare a trusted firewall verdict or grant itself permission.

The Portal labels successful execution with incomplete/partial goal output as
Incomplete/Partial, not green Succeeded. The answer remains visible. A concise
notice links to completion details or the specific real event. Ordinary tool
failures reach an opted-in Agent as observations after the attempt/outcome
commit; audit, identity, cancellation and protocol failures remain fatal.

## Reference native diagnostic

The reference Researcher image can select `native_https` to read exactly
`https://jitsi.org/security/` inside its original Pod. This is application code,
not an MCP tool, proxy or networking grant. It uses no external credential or
HTTP proxy, follows no redirects, validates TLS, limits reads to 16 KiB and the
entire attempt to 10 seconds. A public DNS-verified build-time IP can pin the
same destination for controlled network comparisons; it does not disable SNI or
certificate checks. Never put a private/metadata address in this build argument.

The original execution session records a selected action and bounded worker
observations. Those reports are not independent CNI proof. A standalone
`/agenova-demo-worker --native-probe` supports controlled diagnostics; label it
as a probe, not a model-generated action.

## Acceptance

Use the same caller, Agent image, Policy and task twice, toggling only web.read.
Both Works grant no direct public egress. A must show actual successful MCP
reading and a useful answer. B must show the real model choosing native access,
an actual original-Pod connection, no page content and an incomplete explanation.
Inspect Work/Pod UID, image digest, network policy union and CNI or same-path
allow/deny controls. No attempt, DNS failure alone, remote error or timeout alone
cannot establish infrastructure enforcement. Do not present unrelated probes as
the model's attempt.

Local enforcing-network procedure and explicit fixture limits:
[packet](../../work/0256-demo-security-completion/task.md).
Cloud acceptance requires the exact released images and separate evidence.
