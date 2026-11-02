# Technical Design: Deny claim-escaping worker and model-originated operations

- Ticket: [#198](https://github.com/wunderforge/agenova/issues/198)
- Feature spec: `spec.md`

## Current State and Constraints

- The control plane drives the worker over `kubectl exec` and validates each worker message in `exchangeWorker` (`internal/runtime/agentsandbox/execution.go`) before calling the console handler, which then calls the Model or Tool Gateway (`internal/console/service.go`).
- The reference worker (`cmd/agenova-demo-worker`) only sends well-formed operations for its own claim, so it cannot exercise the refusal paths.
- The installed service admits one worker image, configured as `compatible-worker-image` in the Platform.
- One claim executes at a time per control plane.
- Ollama logs every request it receives; it is the provider-side oracle (`harness/security/ollama-count.sh`).

## Decision

Use a test-only scripted worker. It is built from the reference worker Dockerfile with the worker entrypoint replaced by a small program that reads its scenario name from the task objective and sends a fixed, predefined sequence of operations. The scenarios map one-to-one to the spec requirements. Two isolated test Platform files select separate namespaces and platform identities, a digest-pinned test image and `compatible-worker-protocol: controlled-v1`. Each has its own state directory and matching engineer template. The original reference installation remains configured with its original image. The test namespaces are deleted by UID after evidence capture.

Each scenario run records: the Ollama count before and after, `agenova work show --json` for the affected requests, and a classification derived from the facts (decision fact present → Gateway Deny; none and the run failed at the worker step → edge rejection; otherwise run failure).

The second-install data point uses a second Platform file with its own namespace and deployment name on the same kind cluster. If the two installs cannot coexist, that is recorded as an environment blocker for that data point only.

## Ownership and Contract Boundaries

- New, test-only: `harness/security/scriptedworker/` (program, Dockerfile, unit test), a run script and a test platform file under `harness/security/`.
- Evidence: `docs/evidence/198/`.
- No change to `internal/`, `cmd/`, `api/` or `deploy/`.

## Alternatives Considered

- Go tests only, against the real handler and gateways: fast, but the Ticket asks for evidence on the installed kind environment; kept as supporting unit evidence where it already exists.
- Prompt injection through the real worker and real model: the model can only choose a tool name and file, and its output is not reproducible; recorded as out of reach for deterministic evidence.
- Changing the serial execution to get two Running claims in one control plane: a product change, outside report-only scope.

## Verification Strategy

- Unit: scenario parsing and the exact operation sequence per scenario, using in-memory input and output.
- Runtime: positive control first, then each scenario, on `kind-agenova-k8s-lab`; provider counts and CLI JSON committed.
- Cleanup: delete only test-owned namespaces by UID; the untouched reference platform status must show no drift.

## Risks and Compatibility

- Other traffic to Ollama during a run would inflate counts; runs are sequential and each has its own before/after reading.
- A changed worker image could be left configured; restoration is a required step with recorded output.
- Evidence is lost when the control plane restarts (known limit, #180); JSON is captured before any restart.

## Observed limits (19 October)

The campaign provides A1 worker-operation evidence only. It does not satisfy A2 injection reaching the Gateway. A terminal target and a Running target in a second installation are supplementary cases, not replacements for the accepted two-known-Running-claim criterion. See [evidence](../../docs/evidence/198/summary.md). The delayed expiry scenario proves cancellation, not an observed post-terminal invocation.

## Public-seam feasibility tests

`harness/security/boundaryprobe/` uses the public Service constructor, reference in-memory backend and an injected Executor. It verifies queueing, downstream Gateway Deny and fresh-context late operation refusal without changing product internals. Because this seam skips installed exchangeWorker, its output is explicitly supplementary. Analysis and remaining maintainer decisions are in [blocker-analysis.md](../../docs/evidence/198/blocker-analysis.md).
