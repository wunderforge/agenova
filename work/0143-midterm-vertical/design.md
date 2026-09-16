# Integration design

## Reuse rather than replace

- Extend application submission to separate admission/issuance from execution so the same RunService and gateway reader can serve asynchronous UI queries.
- Keep the canonical #136 controlled-kind runtime operations; worker transport belongs in its adapter/composition edge, not in shared RuntimeBackend types.
- Extend append-only fact storage without replacing the existing gateway fact APIs; assemble the evidence view above those producers.
- Provider adapter owns bounded HTTP inference, private configuration, timeout, sanitized errors and task-result lookup keyed by system-issued invocation identity. Governance Allow and provider success remain separate.
- Integrate the existing React preview rather than redesigning the accepted pages. Source adapters own transport; components consume one evidence representation.

## Worker transport and sequencing

Use Kubernetes exec/stdio for this local controlled checkpoint. The adapter validates the recorded backend/worker binding and pins context, namespace, container and allocated Pod. Kubernetes authentication authorizes the transport; the server binds each operation to the system-issued claim associated with that session. A nominated other claim is rejected before entering a gateway; SHA-256 control tokens are correlation only.

Sequence: Start acknowledges a waiting live worker -> RunService publishes Running -> WorkFunc dispatches the task through the pinned session -> worker emits tool/model requests -> host gateways enforce current authority -> adapter returns real model text -> worker produces the task result -> RunService publishes terminal outcome -> Terminate and Cleanup close/release the worker. Do not perform governed inference inside Start or treat the existing probe acknowledgement as task success. Bound/terminal invocation checks and wrong-binding/late-result tests are frozen gates.

This is an authenticated local transport, not a deployed HTTP proxy or production workload identity claim. #121 remains necessary for future network proxy calls with short-lived claim-bound credentials; do not claim its full scope implemented by this bridge.

## Security and lifecycle

The host server has a trusted principal and backend context; neither comes from ClaimRequest. Serial runtime execution is accepted because RuntimeBackend has no general concurrency guarantee. HTTP responses and evidence must not expose credentials or adapter configuration. Provider timeouts, worker failure, governance denial and cleanup failure remain separately inspectable. Long-lived credentials and unintended mounts are excluded from supported worker configuration; full hostile-agent filesystem/network proof remains #51 work unless its named gate is actually run.

## Release gates

First prove deterministic facts/API/provider and memory paths; then controlled kind; then separately authorized real inference; finally browser submission-to-kind-result and denied-before-allocation. Do not weaken or skip a failed gate to claim the checkpoint. Capture representative renderings without changing the accepted information hierarchy.

## Work names (owner-approved UI follow-up, 16 September)

Separate optional Work name from full Task instructions. The local demo uses the
existing extensible `spec.task.input.workName` string as display metadata; the
unchanged worker composition still reads `objective` only. No new top-level
contract field, identity, policy selector, backend behavior or LLM summarizer.
Request metadata.name remains the unique request reference, not a display title.

Both sources show compact names in lists/detail headings and disclose the complete
instructions on demand. Legacy requests use the first sentence/line, capped at
80 characters. Lists clamp to two lines. Search retains full objective and request
ID matching; Execution details offers Copy request ID with explicit failure feedback.
Demo stories remain independent from Connected evidence. Existing facts/requests are
not rewritten. Validate canonical round-trip and unchanged provider objective,
unit naming cases, submission/list/detail/browser-copy flows and read-only rendering
of the retained session; no live submission/inference is needed for this change.
