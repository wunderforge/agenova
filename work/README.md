# Task Packets

`work/` contains the execution context for GitHub Tickets. A packet helps one coding Agent perform one approved unit of work; it is not a backlog or project-status mirror.

## Create a Packet

Read the Ticket, `AGENTS.md`, and `docs/product/prd.md`, then run:

```powershell
.\scripts\new-task.ps1 -Issue 72 -Slug claim-request -Title "Define ClaimRequest v0"
```

Select additional planning depth only when justified:

```powershell
.\scripts\new-task.ps1 -Issue 72 -Slug claim-request -Title "Define ClaimRequest v0" -WithSpec

.\scripts\new-task.ps1 -Issue 72 -Slug claim-request -Title "Define ClaimRequest v0" -WithSpec -WithDesign
```

This creates:

```text
work/0072-claim-request/
  task.md
  spec.md       # optional
  design.md     # optional
```

Use the zero-padded Issue number and a lowercase kebab-case slug. The script refuses to overwrite an existing packet.

## Ownership Boundary

GitHub owns Owner, Reviewer, priority, sequence, dependencies, readiness, status, PR links, and accepted completion evidence.

The packet owns the Agent's context, scope, implementation Todo, constraints, task-local decisions, commands, and required evidence. Do not copy Project fields into it.

## Review Gate

The Agent drafts the packet and resolves all template markers before implementation. Owner/Reviewer review before implementation is recommended, especially for ambiguous or shared-boundary work, but is not a blocking gate. When implementation begins from an unreviewed packet, record that status in the packet and report it at handoff. Independent review remains required before merge.

Use `spec.md` for shared behavior and `design.md` for a technical decision that dependent contributors need to rely on. Do not add them for routine local work.

After merge, the packet is frozen provenance. GitHub remains the source of current delivery state; completed task checkboxes in old packets are not a live dashboard.
