# Cloud decoder and CLI compatibility follow-up

The first real Nova Micro / MCP run reached both providers. Actual model calls
took 466–620 ms and the successful public search took 275 ms. The stored provider
response/decision summary/tool arguments/results were retrieved through the
authenticated, Work-scoped content API. This does not establish answer acceptance.

Two source defects were reproduced rather than hidden by retrying unchanged work:

- Nova mixed a prior tool's resource into a finish action, then omitted `action`.
  Its structured tool response did not reliably follow the compact branch-only
  `anyOf` requirements. The reference Agent correctly rejected those actions;
  weakening that parser would hide the problem.
- The strict CLI evidence validator rejected the new canonical `AgentFeedback`
  events even though the running server and UI supported them. This interrupted
  CLI polling and is an actual new-protocol compatibility defect.

The repair uses one bounded, required-field object only for the opt-in
discovery-v3 provider grammar. Legacy grammars remain unchanged. It keeps action,
tool and scope enums, excludes native HTTPS from the Gateway-only strategy and
requires all nine fields. The existing Agent/parser and Gateway still enforce
action-specific empty fields, exact tool/scope pairing and route-specific input
limits. Provider formatting cannot grant authority.

The reference Agent supplies complete examples, repeats the mandatory action and
empty-field requirements after a format error, and records the specific parser
failure in its bounded Agent-reported feedback. It distinguishes an event's date
from a multi-day holiday period without hardcoding a question's answer. Provider
choice, external credentials and network grants are unchanged.

The CLI accepts only the shared canonical feedback vocabulary, correlated to the
latest validated model-action turn and original Work/claim. Foreign invocation
IDs, unknown codes, altered descriptions, future/missing turns, authority/decision
payloads and lifecycle violations remain rejected. Canonical public descriptions
now have one protocol-level source of truth used by both producer and consumer.

This follows [Nova's structured-output guidance](https://docs.aws.amazon.com/nova/latest/userguide/prompting-structured-output.html)
to provide an explicit output schema and exact JSON instructions. The reason to
choose a flat object here comes from the observed cloud responses, not a claim
that every provider rejects `anyOf`. Model conformance still needs real acceptance.

Focused tests passed for workerprotocol, connectedclient, console and the reference
worker, including meaningful feedback attribution/mutation checks, finish-only
grammar preservation, argument-size hints, legacy grammar preservation and
recovery from the observed cloud finish-field defect. The unchanged Windows
`TestOwnerBootstrapDeliveryGuards` remains a known local failure. Latest Linux
full/race/browser gates and exact rebuilt OCI image tests must pass before using
the new candidate. Cloud positive/negative/Advisor/legacy acceptance is pending.

An isolated real Nova Micro protocol probe then completed the public-source
answer and an honest incomplete answer after a simulated denial, using three
model calls each. The first output needed one format repair. A singular next-step
finish example after actual feedback prevents contradictory completion fields;
valid tool progress resets the consecutive-error window, while model-call and
consecutive-error bounds remain. This probe used the reference Agent and a
previously observed public search result; it is **not** a governed Work or proof
of a real Gateway denial. Those checks remain required against the rebuilt images.

Separately, initial restoration used the wrong Kubernetes API service IP in a
controller-only network rule. The operational composition now reads the actual
`default/kubernetes` Service IP and verifies it against the EKS service CIDR;
only the DNS/API endpoints are allowed. Worker default denial is unchanged. The
initial allocation failure and all subsequent attempts remain separate evidence.
