# Candidate evidence delivery validation

Original measured executions remain under `raw/`. The maintained driver was extracted afterwards and validated with fresh prefixes in the same owned installation; these additional results are under `driver-validation/`. They do not overwrite original observations or change acceptance.

| Check | Result | Evidence |
| --- | --- | --- |
| Driver help / explicit-execute guard | pass; guard rejected without cluster contact | `driver-validation/help.txt` and recorded command review |
| Maintained driver groups 1, 2, 3 | all exit 0; actual backend counters/receipts/watches captured | `driver-validation/group1.log`, `group2.log`, `group3.log` and directories |
| Offline raw/hash verifier | pass | `checks/offline-verifier.log` |
| Verifier semantic negative control | pass; false foreign MCP delta rejected even after repairing hash | `checks/verifier-negative.txt` |
| `go test -race -count=1 ./harness/security/...` | pass | `checks/harness-race.log` |
| `pwsh -NoProfile -File ./scripts/check.ps1 -All` | exit 0; Go, frontend contracts/types/tests/build and 72 browser smokes | `checks/check-all.log` |
| Final Docs / diff check | pass | `checks/docs-final.log`; `git diff --check` |

The driver checks cluster ownership, installed target/readiness, new prefixes, explicit execution and operator attestation after measured strict CNI prerequisites. It never declares an Epic pass. Group 2 driver results remain terminal quiet windows, not installed old-token replay.

The source commit in driver metadata is the unchanged product baseline HEAD `84fe4cf`; the validation ran against uncommitted delivery-only harness/docs changes. Exact maintained driver/verifier hashes are recorded in `delivery-source-sha256.json`, with matching source copies in `provenance/`. The original provider/Worker observations are not relabelled as executions of that later driver.

Archive copying was allowlisted: no kubeconfig, CLI state, real credential values, downloaded tools or binaries. JSON-key/pattern checks supplement manual scope review; they are not a universal secret-scanner guarantee. Initial preparation/assembly failures remain described in the summary and original raw files/build logs. No quality gate was skipped or product code changed.
