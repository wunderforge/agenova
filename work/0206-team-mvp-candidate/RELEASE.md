# v0.x-team-mvp release check

Status: **candidate, not accepted**. This record is for [E14 #176](https://github.com/wunderforge/agenova/issues/176) and [#206](https://github.com/wunderforge/agenova/issues/206). It distinguishes completed synthetic integration work from acceptance that needs real participants.

| Gate | Check | Status |
| --- | --- | --- |
| M1 trusted authority | [PR #207](https://github.com/wunderforge/agenova/pull/207) | Open; review/merge pending |
| M2 verified OIDC installation and owner scoping | `codex/e14-m2-oidc`, `pwsh ./scripts/check.ps1 -All` | Published branch; full gate passed; predecessor pending |
| M3 versioned Policy constraints | `codex/e14-m3-authority`, `pwsh ./scripts/check.ps1 -All` | Published branch; full gate passed; predecessor pending |
| M4 worker-to-Gateway identity | `codex/e14-m4-worker-identity`, `pwsh ./scripts/check.ps1 -All` | Published branch; full gate passed; predecessor pending |
| M5 same-spec two-user installed smoke | `npm --prefix ui run test:installed -- e14.spec.ts` for each fixture user | Passed with synthetic identities on existing kind |
| Controlled host-token provider | `TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow` | Passed focused integration; installed credentialed provider not run |
| Real company OIDC A/B | Two verified human subjects, one installed service, identical spec, distinct bounded authority | Pending identity access and run |
| Independent teammate clean run | Second person's versioned install, execution, reapply, cleanup transcript | Pending second participant |
| Full repository gate | `pwsh ./scripts/check.ps1 -All` on M5 | Passed, including 52 browser smoke tests |

Before acceptance, replace the synthetic issuer and Policy subject mappings with company-controlled OIDC configuration and verify both users without switching the control-plane identity. Check each clean-environment run's exact commit and Platform revision, Policy/template versions, positive and negative invocation counts, CLI/API/Portal parity, secret scan, reapply, and bounded cleanup. Generated screenshots, JSON records, transcripts, and gate logs stay out of Git. Preserve actual failures in the review discussion; a successful synthetic smoke cannot be promoted into company-identity evidence.
