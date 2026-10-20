# Feature Specification: Independent Agent images and automatic platform delivery

- Ticket: [#240](https://github.com/wunderforge/agenova/issues/240)
- PRD outcome: backend-neutral Agent artifact selection and governed execution evidence.

## Intent

A template chooses an application artifact. Platform installation/release does not choose all future Agent artifacts. Retain controlled-v1 and its executable paths; this is not arbitrary-program adaptation.

## In Scope

- Independent template artifacts, safe runtime diagnostics, legacy configuration and automatic default platform delivery.

## Out of Scope

- New authority APIs, protocol/SDK, image-provenance trust, generic container execution, HA/drain, live switches and production deployment.

## Requirements

- Registration checks canonical structure and runtime-supported names, not image/entrypoint equality. Preserve the reference operator's existing positive timeout ceiling of five minutes; do not retroactively reject member templates with broader ceilings. Actual stdio execution remains bounded to five minutes. Registration certifies neither compatibility nor trust.
- Each Work uses its immutable template image and command. Separate images retain separate pools and claim-bound identities.
- Public template names are project-local. Internal runtime template/pool identities derive from the complete immutable resolved template, never its display name alone. Different images, commands or ceilings cannot repoint another template's warm resources. Equal-content warm resources may be shared without sharing claim identity or authority.
- Missing control/stdio programs, invalid acknowledgements and image-pull/readiness failures have safe actionable reasons. Never expose raw worker output or fabricate cleanup confirmation.
- Legacy `compatible-worker-image` and `AGENOVA_ALLOWED_WORKER_IMAGE` are non-authoritative deprecated inputs; omitted image settings work. Unknown explicitly configured protocols remain unsupported.
- main publication/deployment defaults on after green tests; the new `SHARED_EKS_RELEASE_DISABLED` / `SHARED_EKS_DEPLOY_DISABLED` switches stop on `true`. Obsolete opt-in variables are no longer gates. PR/other-branch/docs-only events access no cloud role.
- Emergency stops cover both main and explicitly authorized candidate paths at workflow admission and script gates. A deploy-only stop permits publication, not deployment; invalid stop values fail closed.
- Reference Worker publication never rewrites templates. Worker-only changes do not restart the control plane or claim its image source changed.
- Control-plane rollout preserves auth/configuration, refuses active claims and retains guarded rollback. Protocol-breaking code changes still require version/compatibility handling.

## Negative Cases

- Invalid structural fields, unbounded execution, unknown protocols, cross-claim tokens and terminal authority remain rejected. Incompatible images fail before governed dispatch where the protocol fails, with cleanup confirmed or explicitly unknown.

## Compatibility

- Existing immutable templates and legacy Platform documents remain valid; successful execution still requires controlled-v1. Requested access cannot grant authority; authentication, per-Work scope, workload tokens and credential boundaries are unchanged.

## Open Decisions

- Owner approved this bounded scope on 20 October 2026. No new protocol or generic application runtime is included.
