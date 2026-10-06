# c6 export manifest

This directory is a reduced, sanitised copy of the campaign output. It was written before the campaign's token scan, which searched it as an extra root.

## Contents

- `run.json`: source commit, cluster and namespaces, model and digest, Platform revision, image configs and archive checksums, toolchains, dataset SHA-256, every step's start, end, duration and exit code, and every case with its tier, layer, attempts and evidence path.
- `summary.md`: result and limitations.
- `campaign/`: the runner's output for this campaign.
  - Preflight, build identity and commands, load identity.
  - Fixture rendering, events and identity chains, controlled read.
  - Install transcript, without the access reviews and the control-plane Pod JSON.
  - Fixture and control-plane logs.
  - Every Work attempt (request, `run.json`, `work show`, checker output, worker identity lines, server snapshots, parity reports, screenshots and saved pages).
  - The probe Job, its receipts and checker input.
- `gates/`: the gates rerun on the clean tree at the source commit after the campaign; `gates/gates-source.txt` records the commit and times.
- `steps/timings.txt`: each step's wall-clock start, end, duration and exit code from the step wrapper, because `campaign.log` records only successful ends.
- `SHA256SUMS`: SHA-256 of every other file here.

## Left in the local campaign directory

- Built binaries, their build-info records and the campaign's image archives (identified by `run.json`).
- The worker Pod lists, Pod JSON, `/proc/1/environ` and `/proc/1/mountinfo` captures.
- `secrets.txt`, the access reviews (`can-i`) and the control-plane Pod JSON.
- The reference install's protect plan and state, its exported images (`protected-images/`) and its Work history.
- Process id files.

The runner checked all of these during the campaign. The list of files kept local is `export-left-local.txt` in the step directory beside the campaign output, not in the repository.

## Sanitisation

Byte replacements in every text file (PNG screenshots unchanged):
- the repository's absolute path → `<repo>`;
- `/private/var/folders`, `/var/folders` and `/private/tmp` → `<tmp>`;
- the home directory → `<home>`;
- the local user name → `<user>`.

Paths were replaced in 18 campaign files and in 3 gate files (`check-all.txt`, `check-docs.txt`, `ui-test.txt`). After the scan, and to pass the repository's whitespace check, four files had only their whitespace normalised: trailing spaces removed, a final blank line dropped, and CRLF made LF. These are `campaign/controlled-read/token-probe-headers.txt` (the fixture's 401 response headers) and `gates/check-all.txt`, `gates/ui-test.txt` and `gates/ui-typecheck.txt`. No line was joined, so the scan result still holds. Nothing else was edited.
