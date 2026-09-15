# E8-S1 Agent Sandbox Mapping Evidence

- Ticket: [#66](https://github.com/wunderforge/agenova/issues/66)
- Pull request: [#105](https://github.com/wunderforge/agenova/pull/105)
- Date: 2026-09-14
- Experiment commit: `6274b8167669c32bf9f3c1e7ac61da6375fd6286`
- Upstream substrate: Agent Sandbox `v0.4.6`
- Docker Engine: `29.7.2`
- kind: `v0.33.0`
- kubectl client / Kubernetes server: `v1.36.1` / `v1.37.0`
- Cluster / explicit context: `agenova-k8s-lab` / `kind-agenova-k8s-lab`
- Raw-output SHA256: `a474ae9c31ffa9cecc96f59ad9d03391badd888de78856dc734238a1781207dc`
- Result: pass

## Exact Commands

From the repository root:

```sh
bash harness/spike/agent-sandbox-substrate/reproduce.sh up --capture
bash harness/spike/agent-sandbox-mapping/reproduce.sh kind-agenova-k8s-lab
bash harness/spike/agent-sandbox-substrate/reproduce.sh down --capture
```

The mapping script records all four installed CRD schemas through
`kubectl explain --recursive`, then runs only:

```sh
go test -count=1 -v -tags integration -timeout 5m \
  ./harness/integration/agentsandbox/ \
  -run '^TestRuntimeBackend_AllocateObserveCleanup$' \
  -args -kube-context kind-agenova-k8s-lab
```

## Demonstrated Result

The adapter allocated a unique claim, preserved the assigned backend identity,
and observed the upstream Ready condition. On that same valid, unreleased Ready
identity:

- `Start` returned `runtime.ErrUnsupported` because upstream readiness is not a
  work-start acknowledgement and the adapter has no worker-start channel.
- `Terminate` returned `runtime.ErrUnsupported` because the adapter has no
  worker-stop evidence channel distinct from resource deletion.
- `Cleanup` then confirmed both the claim and assigned Sandbox absent and
  returned `Released=true`, `Replaced=false`.

The full command transcript and CRD schemas are in [output.txt](output.txt).

## Classification Effect

This real-cluster negative closes the E8-S1 reproduced-unsupported requirement.
It confirms the existing `unsupported` classifications for Start and
Termination. The same bounded run provides real-cluster observations for the
translated allocation, readiness, backend-identity, and cleanup paths, while
leaving their documented recovery and atomicity gaps intact.

Durability remains `unsupported`: this run does not restart the adapter or add
persistence. General isolation remains `unknown`, and the observed filesystem
evidence remains `Unsupported`; no filesystem, process, credential, or network
enforcement claim follows from this experiment. Those proofs belong to #48 and
#51.

## Retained Revision and Provenance

The accepted run was repeated on 2026-09-14 after merging main through #129.
The clean tested commit [6274b81](https://github.com/wunderforge/agenova/commit/6274b8167669c32bf9f3c1e7ac61da6375fd6286)
was pushed before execution and remains an ancestor of this delivery.
Its Git tree is `a127a4150c9f72c62b6b91b5bbaf27d0e0dd1332`.
The former local-only 3ce542b transcript is superseded by this run.
Untracked local .DS_Store and meeting-PDF scratch files were not test inputs.

[Source SHA256 values](source-sha256.txt) pin the reproduction script, integration
test, adapter, allocation implementation, and substrate script at the retained
commit. From the repository root, verify with `shasum -a 256 -c docs/evidence/E8-S1/agent-sandbox-mapping/source-sha256.txt`.

[Substrate deployment](substrate-up.txt) records the v0.4.6 controller and CRD
versions. [Substrate cleanup](substrate-down.txt) confirms the owned cluster was
deleted. These phase captures accompany the adapter test; they are not an E8-T3
full-lifecycle rerun or #51 isolation proof.

## Final Repository Gates — 2026-09-14

Both gates ran on the clean, retained delivery commit
[d9d2f38](https://github.com/wunderforge/agenova/commit/d9d2f381a5f2cfd9e2ffc1ab9271b3d106bcf90b),
tree `70fbe01400cd0173adf02051b57a2725dc6e81b6`. That commit includes
the final reproduction script/test, updated mapping, and September 14 cluster
evidence. The subsequent capture commit only appends these gate transcripts
and documents their provenance; it does not change executable inputs.

| Command | Exit | Raw output |
| --- | --- | --- |
| `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -Docs` | 0 | [docs-output.txt](docs-output.txt) |
| `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All` | 0 | [baseline-output.txt](baseline-output.txt) |

Each transcript begins with the tested commit, tree, and UTC start time and ends
with its exit code. The full gate includes Go formatting, module tidiness, vet,
unit tests, integration compilation, frontend contracts/types/components/build,
and seven passing browser smoke cases. These are repository gates, separate
from the real-backend evidence above.

Local tools were on PATH from `.tmp/toolchain/go/bin`, `.tmp/toolchain/bin`,
the bundled Node runtime, and `~/.docker/bin`; PowerShell was invoked from
`.tmp/toolchain/pwsh/pwsh`. GOPATH, GOCACHE, DOTNET_CLI_HOME, and
PLAYWRIGHT_BROWSERS_PATH were absolute paths to `.tmp/go-path`, `.tmp/go-cache`,
`.tmp/dotnet-home`, and `.tmp/playwright` respectively. No credentials are
required by these gate commands. Local untracked meeting scratch files and
`.DS_Store` are excluded from the deliverable.
