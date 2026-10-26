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
- Compatibility commit 8850167 preserves old product behavior while accepting
  schema 011 and the exact legacy line-ending checksum variants. Published
  rollback digest: `sha256:232f4671e9be37a53e0cd02fde8386ce22a9d4e6ede4fde25e134521d2a8e15d`
  in the existing control-plane ECR repository. Not yet deployed.
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
- Final human role decision: Admin manages all organization projects/users and
  may appoint Admins/operators; Owner adds confirmed ownership transfer. Keep
  Owner/self and cross-organization guards. The new no-project-grant delegation
  matrix and rendered Admin controls are part of the release regression suite.
- The previous CI run exhausted its ten-minute budget after cold browser/dependency
  installation consumed eight minutes. Increase baseline timeout to twenty minutes
  while preserving every quality gate; a fresh completed run is mandatory.
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

## Observed release and cloud acceptance — 23 October

- #253 merged as `c6cf342c77f21c477eb20937312dc71c2359318d`; PR CI and
  independent re-review passed. [Automatic release 39917887916](https://github.com/wunderforge/agenova/actions/runs/39917887916)
  passed validation, snapshot, publication and deployment. EKS control plane is
  healthy on `sha256:7a21c53971f9f64ecb57412165c859e7458dc375d4f2f4b8d486d7a72bc8e451`.
- Schema 9 -> 11 and minimal grants verified; existing member/project counts
  unchanged. Owned temporary migration Jobs/Secrets removed. Old-product/schema11
  bridge was healthy before the automatic cutover; retain its published digest.
- Existing single MCP Pod updated to the reviewed `b66531e...` digest. All old
  routes retained; three Researcher tool profiles appended. Final Platform
  revision `sha256:b797fdd5dbce505db3777528d3030436876e218013a77d01c354e4b2ddfe45cf`.
- `research` created through authenticated audited API; identical reapply returns
  changed=false. `research-governance/3` and immutable researcher-v2 registered
  through normal CLI. Live Admin has no explicit research grant but successfully
  manages it; one active Owner remains unchanged. Researcher is absent from the
  payments catalog; payments retains reference-default-deny/network-mcp-20261021-v2.

| Actual Work | Result | Evidence |
| --- | --- | --- |
| Public research | Succeeded; only web.read / web:public-pages; real Nova Micro and correlated MCP call | [Work](https://app.agenova.app/?project=research#/work/work-1526c4aa1258268f6b41f91e280a68b1976cd7dfd871820824acd46b0de848f6) |
| Internal retention | Succeeded; only crm.read / crm:customer-retention; same caller/template; real Nova Micro and correlated MCP call | [Work](https://app.agenova.app/?project=research#/work/work-94ef3d758b9e3791a7611404fdad7dcf36d32c936a2d77e39e44e7c4a9110b32) |
| Missing web permission | **Acceptance failed:** no web authority or web downstream call, but unrelated CRM observation then refusal was incorrectly labelled Succeeded | [Retained failed attempt](https://app.agenova.app/?project=research#/work/work-11c25e5ed9aac49d037ee1df839263712dd0309865d21996876c7659885d8212) |
| Existing Engineer | Execution/grants/MCP passed, template unchanged. Answer-quality review does not accept the fresh-deadline retry recommendation; do not claim semantic quality passed | [Work](https://app.agenova.app/?project=payments#/work/work-f0dcba20992d59b24c427b72c0069d491109c0bc2252010e92b58d62432dc724) |

Public answer checked against [the actual Jitsi source](https://jitsi.org/security/)
for ephemeral rooms and sensitive room names. CRM answer checked against the
synthetic dataset: C001 (14 inactive days / 62% usage drop) and C002 (21 / 45).
No source-grounding claim is made for the incorrect legacy recommendation.

Researcher outcome follow-up: separate its prompt from the generic mandatory-read
prompt and add explicit blocked completion handling. Preserve platform/legacy
contracts and all failed evidence; test and independently review before a new
immutable Researcher version. Administrator bypass authorization for #253 does
not authorize a new PR. Overall cloud acceptance is **NOT PASSED** yet.
