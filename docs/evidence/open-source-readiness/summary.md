# Open-Source Readiness Evidence

- Task: prepare Agenova for an Apache-2.0 public release
- Gate date: 2026-08-19
- Branch: `codex/open-source-readiness`
- License source: canonical Apache License 2.0 text from `apache.org`
- Secret scanner: Gitleaks v8.30.1, official release checksum verified

## Accepted Results

1. `./scripts/check.ps1 -All`
   - license, notice, attribution, SPDX header, module path, docs, formatting,
     unit tests, reference E2E, and integration-package compilation passed.
2. `gitleaks git . --log-opts=--all`
   - 47 reachable commits scanned;
   - 714.83 KB scanned;
   - zero findings.
3. `gitleaks dir <publishable-tree-mirror>`
   - 52 tracked or publishable working-tree files scanned;
   - 171.30 KB scanned;
   - zero findings.
4. Remote-ref audit
   - preserved `main`;
   - preserved `codex/proposal-presentation` at `7437cb8`;
   - removed the obsolete remote backup branch, archive tag, merged delivery
     branch, and retired runtime-isolation documentation branch.

## Limitations

- The repository remains private until this change is reviewed and merged.
- Secret scanning reduces risk but cannot prove that no sensitive information
  exists.
- DCO sign-off is documented for new contributions; an external DCO GitHub App
  is not configured by this change.
