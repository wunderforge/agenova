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

Owner-only account promotion, privileged delegation and confirmed ownership
transfer remain unchanged. Creating a new Owner is not part of project creation.
The existing last-Owner and atomic transfer guards remain in force.

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
