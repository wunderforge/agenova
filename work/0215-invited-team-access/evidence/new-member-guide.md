# New-member guide walkthrough — 13 October 2026 (Sydney)

Guide: [新人上手](../../../docs/member-quickstart.zh-CN.md).
Sample: [member-work.yaml](../../../docs/examples/member-work.yaml).

Actual installed Homebrew CLI: `0.1.0-preview.20261012.1`.
Executed `agenova login --server https://app.agenova.app`: exit 0, login succeeded.
Then executed `agenova work list`, `agenova run -f work.yaml --json`, and
`agenova work show <request-ref> --json` without server/state-dir/cloud arguments.
All completed successfully; a second list showed the new Work. The same Work was
visible in the live connected Portal as Succeeded.

- Request: `work-9f6abc3427588dc1bf42fda86baca87e66301364825bc3a3f78b387513d49f5f`.
- Trusted subject: `user:ba3c1ed4fb8044257fd7033ec6cc3151`; OIDC, team-a.
- Project/template: payments/engineer; admission Allow.
- Runtime: agent-sandbox, worker `agenova-pool-pool-engineer-65ldn`.
- Model outcome: amazon.nova-micro-v1:0, response
  `b046c616-6a7a-464e-a415-6dc334d58e19`, 616 input / 39 output tokens.
- Result text: “Your task-dependent answer will follow after a thorough review of
  the observations.” This is not an adequate answer to the task. Transport/runtime
  success does not establish answer quality; the guide explicitly distinguishes it.

This walkthrough reused an already approved Google account. It did not create a
new account, re-approve Fisher, change permissions or prove the pending-to-approved
transition anew. The live members page showed both existing accounts active and
no pending requests. The native login/browser selected the existing identity;
this run must not be attributed to Fisher. No user password/token was recorded.
No claim of a fresh-machine installation or published public tap is made; the
installed local-tap package was used. Newcomer distribution remains a preview.
