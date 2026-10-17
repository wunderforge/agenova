# c8 export manifest

This is a reduced, sanitised export of the local c8 acceptance at `c50a6bc9af3caeba8cef815ebb29d7dcad4b1e5d`. [The summary](summary.md) distinguishes product results, execution problems, scan completion and remaining limits.

- `run.json`: source, images, toolchains, dataset hashes, step exits/timing, every Work attempt, probe verdicts and scan status.
- `campaign/`: build/deploy records, controlled read, complete server logs, each Work's CLI/API/Portal records and screenshots, probe receipts, initial incomplete scan and completed scan.
- `gates/`: full repository gate, independent fixture tests and 52 runner groups, all executed on this source immediately before c8.
- `steps/`: command output and timings, plus sanitised copies of the local execution wrappers.
- `reference-update/`: subsequent default reference deployment and its independent installed smoke evidence.
- `SHA256SUMS`: SHA-256 of every other exported file, generated after the final additions.

The completion scan searched the raw campaign, its CLI state, this export and the step directory before restore. Later additions are the scan result, restoration output, this explanatory metadata, and the independent reference update. Copies of already-scanned records are sanitised as described below. The initial FIFO failure is retained, rather than replaced with the completed scan's result.

Local-only records include binaries, image archives, binary build-info files, worker Pod/environment/mount captures, Secret identity records, raw access reviews, control-plane Pod JSON, old reference Work history, and protected-image/replica records. They were included in the raw campaign scan. The local omission list is `.tmp/e16-c8-steps/export-left-local.txt`; raw output remains in `.tmp/e16-c8/`, `.tmp/e16-c8-state/` and `.tmp/e16-c8-steps/`. The controller FIFO was moved outside all evidence roots before the completed scan.

Sanitisation replaces the checkout path with `<repo>`, the home directory with `<home>`, temporary host roots with `<tmp>`, and the local user name with `<user>`. Text line endings and trailing whitespace are normalised. Screenshots and other binary evidence are copied unchanged. The initial scan log's filename is shortened to `scan-initial.log` to omit the host path; its content and failure are preserved after those replacements.
