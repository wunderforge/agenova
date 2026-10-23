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

Account Admin is the highest daily management role: it can appoint other Admins,
manage project grants, delegate governance/operator duties, approve invitations
and access requests, and disable/re-enable other non-Owner users. These operations
cover existing projects in the same organization without explicit project grants.
The operational-role editor replaces old duties atomically; low-level account-role
edits change that role only and do not remove independent project/scoped grants.

Owner has the same daily capabilities plus explicitly confirmed ownership
transfer. Ordinary role, activation and grant mutations cannot modify Owner.
Self account/operational/scoped-role and activation edits are rejected to avoid
lockout. Project-only administrators cannot promote Admins or delegate platform
duties. Every change retains active-actor checks, organization scope and atomic
audit. Creating a new Owner is never part of project creation. Application
operator duties never grant cloud IAM or Kubernetes RBAC.

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
