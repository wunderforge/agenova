# Verification: bounded Researcher recovery and Agent directory

## Local results (2 November 2026)

- `go test ./cmd/agenova-demo-worker ./internal/workerprotocol`: PASS. Includes consecutive malformed finishes, mixed format/duplicate-read errors, denial preservation, Work/image call bounds and actual host/Worker exchange.
- Frontend unit suite: 197 tests / 19 files PASS.
- TypeScript typecheck and production Vite build: PASS.
- Focused rendered catalog smoke: PASS, desktop and 390px mobile. Dark browser preference stays light. Older versions remain explicitly discoverable; exact registration identity and project links are retained.
- `./scripts/check.ps1 -All`: FAIL only at the existing Windows `TestOwnerBootstrapDeliveryGuards` invitation-delivery test. Other Go packages pass; formatter, module metadata, vet, architecture and documentation gates passed. Frontend was run separately after this stop.
- Whole browser smoke suite: 99 tests PASS (1.7 minutes), including the new directory behavior and existing task/authority/activity flows.

## Exact image gate

The existing PR CI runs `harness/demo/research-mcp/build-worker.py`, including four new recovery/budget scenarios in the built image, without AWS credentials. Local Docker cannot start: its inference runtime socket reports "The file cannot be accessed by the system." A narrowly attempted rename failed; socket and Docker data disk are unchanged. Do not present the local image gate as passing. The exact Worker source passed all four new built-image scenarios in Linux CI run 41427196532 at b1c4540. Subsequent follow-up changes affect only UI selectors, their tests and this packet; final follow-up CI is pending.

## Evidence boundaries

Regression model replies and browser catalogs are synthetic. Production traces were read through the existing authenticated API to diagnose the defect, retained only in ignored local files and never copied into fixtures, this packet or the PR. No image push, template deletion, ECR cleanup, Policy activation, merge, application deployment or AWS resource change is part of these local results.

Version selection is a display convention, not image attestation or a grant. Highest numeric -vN registration is selected within a project/family and release series; the known Advisor series succeeds legacy Researcher, so advisor-v6 wins over legacy v7 in either API order. An unversioned sole registration remains visible. Explicit older-version mode keeps historical registrations reachable.

## Follow-up: selection lists

The owner confirmed English-only demo scope; Chinese Advisor planning is deferred. New Work and configuration selectors now reuse directory version selection. Ordinary selection shows newest registrations only. An explicitly selected historical registration remains visibly labeled previous version, preserving its exact submit identity. Switching to a newer registration reloads the server snapshot and clears copied permission choices. Browser tests exercise both ordinary creation and actual historical-copy submission. No Advisor or authorization code changed.
