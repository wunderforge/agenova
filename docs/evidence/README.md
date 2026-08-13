# Evidence

Evidence proves task acceptance; it is not a progress diary.

Store accepted evidence under:

```text
docs/evidence/<ticket>/<gate>/
```

Each gate should keep only the current accepted run, or one explicit blocker:

- `summary.md`: task, gate, date, branch/commit, command, result, and limitations;
- `output.txt`: raw command output when it materially supports the result;
- screenshots or rendered artifacts only when visual behavior is being accepted.

Superseded runs should not accumulate in the repository. If a failure teaches a reusable lesson, record the learning in `docs/harness/learnings.md` before replacing the old evidence.

Use `scripts/evidence.ps1` to capture command evidence.
