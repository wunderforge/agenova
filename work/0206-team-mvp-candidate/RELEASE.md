# E14 team candidate release check

Status: **draft; not accepted or mergeable as a release**. This is a review record for [E14 #176](https://github.com/wunderforge/agenova/issues/176) and [M5 #206](https://github.com/wunderforge/agenova/issues/206).

| Gate | Evidence on the current member-service stack | Status |
| --- | --- | --- |
| M1 Policy evaluation | [#207](https://github.com/wunderforge/agenova/pull/207), local/full CI pass | Independent approval and merge pending |
| M2 member identity | [#228](https://github.com/wunderforge/agenova/pull/228), synthetic two-member boundary/full gate | Draft; Ticket transport and real installed users pending |
| M3 per-user Policy caps | [#229](https://github.com/wunderforge/agenova/pull/229), focused/full gate | Draft; installed two-user authority proof pending |
| M4 worker identity | [#230](https://github.com/wunderforge/agenova/pull/230), focused/full gate and real kind worker run | Draft; cross-claim/post-terminal installed worker proof passed, two-user acceptance pending |
| Host-token provider boundary | `TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow` | Focused test pass; not installed proof |
| Two real Google members | Same service, byte-equivalent Work spec, distinct Policy-bounded authority | Pending |
| CLI/API/Portal parity | Redacted installed evidence for both members | Pending |
| Independent clean run | Second teammate's versioned install, reapply and cleanup | Pending |
| M5 repository gate | `./scripts/check.ps1 -All` on this exact branch | Pending |

The former direct-JWT/JWKS fixture and token-file procedure are excluded because the Owner selected main's member service. Do not promote synthetic results to real-user acceptance. Generated outputs and secrets stay outside Git.
