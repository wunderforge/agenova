# Feature Specification: Researcher completion and native attempt evidence

- Ticket: [#256](https://github.com/wunderforge/agenova/issues/256).
- Intent: separate execution, agent-declared goal completion and enforcement evidence.

## Requirements

An opt-in reference worker can request a bounded action schema supporting completion metadata and a local native_https action. These are execution-edge features, never authority. Ordinary controlled-v1 operations and final-answer binding stay valid.

Completed, partial and incomplete are optional agent-reported completion metadata. The host verifies metadata against the last governed model response, not natural-language task matching. Incomplete output is retained; execution can finish normally. No model-output classification can create a Gateway Deny or independently verified packet verdict.

Native HTTPS executes inside the image process in the original Pod. Only the approved public demo target, TLS verification, no proxy/redirect/upload and bounded connection/read time are allowed in this reference diagnostic. The platform does not execute HTTP on the worker's behalf. Attempt reports must match the last model-selected action and session claim; reports are explicitly worker observations, not trusted CNI evidence. Missing execution or attribution fails the demo acceptance.

Recoverable provider tool failure returns a bounded observation; lost audit writes, identity faults and protocol errors remain fatal. Existing callers may finish without tools. The UI must not show incomplete output as a green completed business task.

## Compatibility and Non-goals

No lifecycle enum, Policy semantics, network grants or long-lived credentials change. Old images omit opt-in metadata and keep their existing path. History is immutable. No generic shell, search API, model-dependent platform task parser or silently verified infrastructure claims.

## UI

Keep requested-only permissions. Group status with title and actions together. Admin inherited all-project access is informational, not editable redundant per-project grants. Scoped grants remain editable for ordinary members. Tool cards distinguish actual adapter type from instance names without guessing an MCP type from a label.

## Negative Cases

Reject report without matching model action, wrong claim, duplicate report, private IP/target, excessive records and mismatched final metadata. A model assertion of blocking is not an infra verdict. No attempt is not denial; timeout is not automatically packet-block proof.

