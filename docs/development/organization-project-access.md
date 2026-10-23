# Organization and project access

Owner decision, 23 October 2026 ([#252](https://github.com/wunderforge/agenova/issues/252)).

| Live application role | Project access |
| --- | --- |
| Account Owner / Admin | All existing projects in the same organization, including new projects; discover/register configuration, submit Work and inspect evidence |
| Project Admin only | Existing explicit project administration and Work access; no account-wide governance or installation rights |
| Member / Viewer | Existing explicit project grants; Member submits and reads own Work, Viewer reads project Work |

Roles come from active database membership, never request JSON, cached UI state or
IdP claims. Unknown projects/actions, disabled members and foreign organizations
are denied. Demotion takes effect on the next operation. Work Policy, Template
limits, requested access and runtime restrictions still independently determine
each Work's authority. Cloud IAM/RBAC remains independent.

Raw account-role promotion, explicit scoped-role delegation, confirmed ownership
transfer and last-Owner guards remain Owner-only. The legacy `platform-role`
mutation separately permits some explicitly project-authorized Admins to assign
operational roles within their existing grant boundary. This mutation has NOT
been expanded by organization-wide Work/configuration access. Whether to tighten
that legacy delegation to Owner-only is awaiting the human Owner decision; do
not release until this is settled. Creating a new Owner is never part of project
creation.

## Create a project

`POST /api/projects` through the existing authenticated application boundary:

```json
{"project":"research"}
```

Only live account Owner/Admin can create. The server derives the organization
from the authenticated principal; clients cannot supply team, grants or roles.
New names are lowercase alphanumeric/hyphen slugs up to 63 characters. Creation
and `project.create` audit commit together; identical reapplication returns
`changed: false` without another write. Capacity is 100 projects per organization.
This endpoint neither creates grants nor registers/changes Policy or templates.
Policy/Agent registration uses the existing project-scoped CLI/API afterward.

No release-time project SQL, per-user patch or runtime DDL is required. The
runtime's existing project INSERT and audit permissions are sufficient; migrations
remain explicit operator actions, not request-time behavior.
