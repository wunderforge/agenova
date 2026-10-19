# Role UI simplification verification — 13 October 2026

## Scope

The Members page presents Developer, Read-only and Team administrator presets.
Deployment/governance grants and advanced account controls remain under collapsed
advanced sections. Existing backend roles, scopes and authorization checks are
unchanged. Existing memberships are not migrated or rewritten.

## Reproducible checks

- `node harness/integration/identity/invitations.mjs` — passed.
- `node harness/integration/identity/access-requests.mjs` — passed.
- `pwsh -NoProfile -File scripts/check.ps1 -All` — passed, including 55 browser smokes.
- Rendered desktop and mobile fixtures were inspected; approval controls use a
  responsive grid and the mobile page has no horizontal overflow.

The focused browser tests assert exact preset payloads, custom mappings,
Owner-only administrator choices, scoped duty grants/revocations and absence of
advanced grant controls for non-Owners. They use API fixtures, not real role writes.

## Live EKS verification

The real Fisher CLI session applied the image-only member upgrade for installation
`demo-eks-bedrock`, using the operator’s own AWS SSO identity. The deployed image is:

`sha256:183d5a04393f85d5308f6ed606ec3dce4aa2ecad19ee394f755a34c13bde2703`

Apply returned `applied: true`, `ready: true`; platform status returned
`installationReady: true`, deployment state `available`, and the same image digest.
Provider health was not checked by platform status.

The live Owner Members page showed the three common choices and collapsed advanced
controls. Expanding Fisher’s advanced section confirmed the existing
`platform-operator` grant for installation `demo-eks-bedrock` remained present.
No live member grants were changed in this UI verification.

Real `agenova work list --server https://app.agenova.app` returned the four existing
Succeeded records. The login page returned HTTP 200; an unauthenticated request to
`/api/requests` returned HTTP 401. No new model execution was needed for this UI change.

Local rendered proof (ignored artifacts):

- `.tmp/identity/invitation-ui/live-simple-roles.png`
- `.tmp/identity/invitation-ui/live-advanced-permissions.png`
- `.tmp/identity/access-ui/approval-desktop.png`
- `.tmp/identity/access-ui/approval-mobile.png`

Local deployment receipts: `.tmp/operator-upgrade/simple-roles-{plan,apply,status}.json`.
Full gate log: `/tmp/agenova-role-simple-all.log`.

## Limits

This changes presentation only. Advanced duties do not grant AWS credentials.
Existing-member account/project edits remain independent. The previously verified
CLI member upgrade remains image-only and is not yet part of the public Homebrew
preview. This verification does not claim new model configuration or production
availability guarantees.
