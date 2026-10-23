# Researcher real-model release

Owner authorization: after local acceptance on 23 October 2026, adapt the approved
product for a real model, release to the existing Neo environment, and verify.
This supersedes local-only scope for this release, not for unrelated resources,
branch-protection bypass, new model IAM permissions or credential access.

## Sequence and gates

1. Verify live account/cluster, deployed source/configuration, schema and active
   Work. Preserve all existing projects, templates, services and network denies.
2. Use the real Worker/model path, not `tools/local-product-preview`. Provide a
   separate Researcher image using the current stdio contract and real model
   tool selection. Keep generic/legacy agents unchanged. Researcher must not
   claim an evidence-based answer without a successful observation.
3. Test the exact image and real Gateway/MCP composition locally. Unit/scripted
   model tests are protocol evidence, not actual model-quality acceptance.
4. Prepare immutable release images, source/CI checks, safe MCP/data overlay,
   Platform route merge, and project-qualified registrations. No key/search
   service, new node, database, PVC, public ingress or model IAM expansion.
5. Migrations 010/011 need separate confirmation, runtime-only grants and a
   schema-011-compatible rollback image. Existing binaries reject schema 011.
   Stop before shared DB mutation or credential access without authorization.
6. Release through PR/normal CI and existing image delivery, in a quiet window;
   release automation does not migrate SQL, deploy MCP or register templates.
   Stage those prerequisites deliberately; do not let image delivery race SQL.
7. Register the Researcher only to research; preserve existing projects/templates
   and routes. Verify member authorization, catalog filtering and Gateway-only
   network intent. Never replace the shared Policy with the minimal example.
8. Cloud acceptance: same caller/template/project, public HTTPS task versus CRM
   task; real model attempts and non-scripted final answers; exact issued tools
   and scopes; independent MCP receipts; no direct Pod public authority. Retain
   every attempt and distinguish execution success from answer quality.
9. Negative acceptance: missing tool/scope and explicit boundary probes; zero
   downstream calls for rejected operations. A model's refusal/no attempted call
   is not Gateway blocking. The standard Worker also rejects out-of-catalog
   model actions before dispatch: do not weaken this just to force a banner.
10. Verify existing engineer/incident paths, Google/session, UI, cleanup and
    observed image/source. Record failures/rollback and hand over actual links.

## Release preparation status

- AWS SSO refreshed and account 931228356546 / agenova-demo verified. Current
  source 58d951c, one healthy control plane and one healthy existing MCP; no
  SandboxClaims during initial inspection. Recheck before the actual cutover.
- Owner approved 010/011, minimal runtime grants and controlled use of the one
  named RDS managed Secret; no local credential file or Agent exposure.
- CLI personal Google login refreshed through its normal browser flow.
- Existing model is amazon.nova-micro-v1:0; no IAM/model permission expansion.
- Compatibility commit 43ce35c adds only 010/011 and migration registration.
  Rollback image built locally from that exact clean source, preserving the old
  product behavior while accepting schema 011. It is not yet deployed.
- Separate Researcher and research-enabled MCP images built locally. Researcher
  image smoke proves no observation cannot produce a completed result; scripted
  source regressions preserve model-selected text input and reject absent tools.
- Windows full gate still fails the original invitation directory-sync case;
  Linux CI must pass before merge. Existing UI 161 unit / 83 smoke and nine
  actual local API/SQL/MCP journeys passed; not real-model cloud acceptance.
- Owner approved administrator merge only for #253 after all gates. New commits
  must rerun CI before that approval is exercised.
- Formal environment has an active Neo Owner; no ownership reassignment is
  needed. Owner rejected one-off project/user SQL provisioning. Account Admin /
  Owner now cover all organization projects through live application checks;
  audited idempotent `POST /api/projects` will create research after release.
  See [role contract](../../docs/development/organization-project-access.md).
- Real local PostgreSQL identity/Work/management/operator tests passed after the
  role correction, including ordinary project isolation, creation audit rollback,
  demotion/disable revocation and Owner transfer/last-Owner guards.
- First controlled migration attempt failed before commit. Verified database is
  still schema 9 and original service was restored healthy; owned temporary
  migration Jobs/Secrets removed. Read-only checks confirm DDL/minimal runtime
  preconditions; checksum-baseline diagnostics are next. No merged product or
  real-model cloud success is claimed yet.
- Read-only baseline inspection identified the cause: all applied versions 1-9
  match the current SQL exactly except LF/CRLF. No other checksum drift exists.
  The fix uses canonical LF hashes for new migrations and recognizes the exact
  legacy LF/CRLF variants of versions 1-11, without updating applied history.
  Non-line-ending changes still fail; SQL checkout is pinned to LF. This follows
  the line-ending-independent checksum practice in
  [Flyway's implementation](https://github.com/flyway/flyway/blob/main/flyway-core/src/main/java/org/flywaydb/core/internal/resolver/ChecksumCalculator.java),
  with a narrower compatibility window and unchanged SHA-256 algorithm.

## Evidence boundaries

The local preview remains synthetic. No mock records move into shared history.
CRM stays labelled synthetic; public page reads are real HTTPS through MCP.
No keyword-search provider. A real-model run does not certify native isolation;
that requires separately attributed substrate positive/negative evidence.
