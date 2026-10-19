# E14 kind team candidate runbook

This is the local/kind acceptance path for [#206](https://github.com/wunderforge/agenova/issues/206). Run it from the candidate checkout. The synthetic issuer below exercises the installed boundary; final acceptance uses two real company-issued OIDC identities and a second teammate's independent clean run. Neither path proves EKS, IAM, or Bedrock.

## Prepare an isolated reference install

Use an existing `kind-agenova-k8s-lab` cluster with Agent Sandbox v0.4.6, Docker, kubectl, Go, Node, and Ollama. On macOS, `bash harness/local/macos-bootstrap.sh setup` prepares the substrate and loads the branch-built `agenova-control-plane:0.1.0` and `agenova-testworker:kind` images. For this fixture, Ollama must serve `qwen3:8b` at `host.docker.internal:11434` from the kind node. Check with `ollama list` and a real `/api/tags` request; installation readiness alone does not prove model health.

Run `go run ./harness/integration/oidcfixture` in a separate terminal for synthetic smoke. It prints a private temporary directory containing `oidc.json`, `user-a.token`, and `user-b.token`, and serves one disposable JWKS endpoint on port 18080. Keep it running through the smoke. Its token files expire after one hour and must never enter Git, Work YAML, screenshots, or evidence. For company acceptance, prepare `oidc.json` with the actual issuer, audience, JWKS URL, team claim and explicit team mappings; obtain each user's short-lived token outside Agenova. The control plane verifies, but does not issue or refresh, company tokens.

Create only this run's namespace and Secret, then install the checked-in [Platform fixture](platform.fixture.yaml). For a different cluster, change the two context/namespace entries in a local copy before validation. For a different model, change the model profile in that local copy. For company acceptance, the Secret name may remain `agenova-e14-oidc`; replace only its local source file and use a Policy copy with the real subject/team mappings.

```sh
kubectl create namespace agenova-e14-system --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace agenova-e14-system app.kubernetes.io/managed-by=agenova app.kubernetes.io/part-of=agenova --overwrite
kubectl -n agenova-e14-system create secret generic agenova-e14-oidc --from-file=oidc.json=/absolute/private/path/oidc.json --dry-run=client -o yaml | kubectl apply -f -
mkdir -p .tmp
go build -o .tmp/agenova ./cmd/agenova
.tmp/agenova platform validate -f work/0206-team-mvp-candidate/platform.fixture.yaml
.tmp/agenova platform plan -f work/0206-team-mvp-candidate/platform.fixture.yaml
.tmp/agenova platform apply -f work/0206-team-mvp-candidate/platform.fixture.yaml --yes
.tmp/agenova platform apply -f work/0206-team-mvp-candidate/platform.fixture.yaml --yes
.tmp/agenova policy apply -f work/0206-team-mvp-candidate/policy.fixture.yaml
.tmp/agenova agent-template apply -f deploy/reference/demo/engineer.yaml
```

The second identical apply must report `changed: false` and `ready: true`. The OIDC Secret contains only verifier configuration; user tokens stay in protected local files. A missing/malformed Secret must prevent authenticated control-plane readiness or submission, not select fixed-principal mode. A missing JWKS key or invalid token must fail at the authenticated HTTP boundary.

## Submit equivalent Work and inspect it

Make two local copies of [Work fixture](work.fixture.yaml), changing only `metadata.name` to unique request references. Confirm the bytes from `spec:` onward are identical. Use one protected token file for each CLI process; do not put the token on the command line. The synthetic [Policy fixture](policy.fixture.yaml) admits both subjects on one template and version, and limits `user-b` to two minutes while `user-a` retains five. For real company identities, use an equivalent versioned Policy with their verified subjects and mapped teams.

```sh
AGENOVA_TOKEN_FILE=/absolute/private/path/user-a.token .tmp/agenova run -f /absolute/private/path/user-a-work.yaml --json
AGENOVA_TOKEN_FILE=/absolute/private/path/user-b.token .tmp/agenova run -f /absolute/private/path/user-b-work.yaml --json
AGENOVA_TOKEN_FILE=/absolute/private/path/user-a.token .tmp/agenova work show <a-request-ref> --json
AGENOVA_TOKEN_FILE=/absolute/private/path/user-b.token .tmp/agenova work show <b-request-ref> --json
```

Record principal, Policy ID/version, requested/effective authority, narrowing reasons, model/tool decisions, provider outcomes, claim phase, and cleanup. The public `Principal` projection has subject/team/authentication context; no raw JWT or OIDC claim set belongs in the record. Each user's list/read must hide the other's request and claim with the same 404 shape as an absent reference. Submit an unmatched or over-scope request and confirm zero allocation/model/tool/provider calls. M4's installed worker check separately covered cross-claim and post-terminal Gateway denials; repeat it against this candidate before final release.

## Portal parity and reproducible proof

Leave `.tmp/agenova api connect` running on loopback. Start the local Portal with `AGENOVA_TOKEN_FILE` pointing to one user's file: `npm --prefix ui run dev -- --port 5177 --strictPort`. It reads the token only in the server-side proxy. To inspect the other user, stop and restart the local Portal server with that user's token file; the installed control plane remains one process. With `AGENOVA_CLI_PATH`, `E14_OWN_REF`, `E14_OTHER_REF`, `E14_SUBJECT`, and `E14_EFFECTIVE_TIMEOUT` set, run `npm --prefix ui run test:installed -- e14.spec.ts` for each user. The test compares full CLI/API records, proves cross-owner 404/list omission, checks rendered principal/Policy/timeout, and saves a Portal screenshot in local test output. Do not commit generated output.

Run focused Policy/OIDC/authority/Gateway/installer tests and `./scripts/check.ps1 -All`. Record the exact commit, `kind`/Kubernetes/Agent Sandbox/Ollama versions, Platform revision, Policy/template versions, pass/fail outputs, screenshot paths, and any transient provider failures. A second teammate repeats these steps from a clean checkout and records an independent transcript. Never mark the candidate accepted on synthetic identities or one person's run alone.

For the controlled token-requiring provider boundary, run `go test ./internal/console -run TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow -count=1 -v`. Its loopback provider rejects a request without the host-configured bearer token. It asserts zero HTTP calls for a denied Work, one authenticated HTTP call for an allowed Work, and no token in either public evidence record. This focused fixture does not replace the installed Ollama run or the company-identity acceptance run.

## Cleanup

Stop Portal, port-forward, and the fixture JWKS process. Delete only `agenova-e14-system` after confirming its `app.kubernetes.io/managed-by=agenova` label and that this run created it. Remove only the exact fixture directory printed by the fixture process. Leave the shared kind cluster and unrelated `agenova-system` installation intact. Use `bash harness/local/macos-bootstrap.sh down` only when its checkout ownership receipt confirms this run owns the whole cluster.
