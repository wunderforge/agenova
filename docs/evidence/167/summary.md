# #167 installed API / CLI / local Portal verification

Date: 2026-09-17. Dedicated target: `kind-agenova-k8s-lab/agenova-system`, Agent Sandbox v0.4.6, local Ollama `llama3.1:latest`.

## Observed CLI and API flow

1. Built the updated `agenova-control-plane:0.1.0` image, loaded it into kind, and reconciled the installed reference Platform. `platform status --json` reported the same revision with zero changes, `installationReady: true`, provider health `not-checked`, and the `agenova api connect` instruction.
2. `agenova run -f deploy/reference/demo/work.yaml` returned `Allow`, a real Agent Sandbox claim, `Succeeded`, a task-specific Ollama answer, and model tokens. After the CLI process exited, `agenova work list` and `agenova work show investigate-payment-retries` returned the same request reference, claim ID, outcome and answer through the installed API. `work show missing-work` returned nonzero HTTP 404.
3. `agenova run -f deploy/reference/demo/denied-work.yaml` returned `Deny` before a Claim/worker/provider call. The same denied reference appeared in `work list/show` without a Claim.
4. `agenova api connect` printed `Forwarding from 127.0.0.1:8088 -> 8081`; a separate `npm --prefix ui run dev -- --port 5177 --strictPort` served the Portal through a server-side `/api` proxy. The opt-in `npm --prefix ui run test:installed` checked `/api/setup` installed identity/revision, allowed/denied HTTP evidence, list/detail parity, and visible React details; **2 real installed Playwright tests passed**. Screenshots: [allowed Work](installed-allowed-work.png), [denied Work](installed-denied-work.png).

The first real browser run found two integration defects rather than passing on fixtures: the in-cluster Control Plane ServiceAccount lacked `list` on namespaced ConfigMaps for registered-template discovery, and the UI expected uppercase Policy rule fields while the actual Go JSON contract emits camel-case rule fields. Both were corrected, then the same real tests passed. A later failure was only the test expecting the request ID in a collapsed details panel; the test now opens that panel and verifies the ID and claim, matching the intended UI behavior.

## Gates and boundaries

The complete repository gate (`scripts/check.ps1 -All`) passed, including Go, TypeScript, build and 49 fixture browser smoke tests. The real installed Playwright checks are intentionally opt-in because they require a prepared kind/Ollama installation and live request references.

The reference API and UI connection are loopback-only transport, not production user authentication. The service uses a fixed Team A identity; Work/evidence exist in one Pod's memory and disappear on restart. The tool artifact reads are marked mock; Agent Sandbox execution and Ollama inference are real. The UI is a separately started local client, not a `platform apply` installation result.
