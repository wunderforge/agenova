# Task Template

## Goal

State the smallest outcome that should be true after this task.

## Scope

In scope:

- item 1
- item 2

Out of scope:

- later-phase capability 1
- unrelated refactor 1

## Context

Read only the current phase docs and files directly needed for the task. Do not load `docs/human-design-decisions/` unless the task asks for design rationale.

## Acceptance

- Observable behavior or document change.
- No concept drift from the current phase.
- Harness evidence updated when behavior changes.

## Evidence

```powershell
go test ./...
.\scripts\check.ps1 -All
```
