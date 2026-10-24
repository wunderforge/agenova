# #52 Worker gateway-bypass boundary on the kind reference

- Task: [#52](https://github.com/wunderforge/agenova/issues/52), packet [`work/0052-worker-gateway-bypass/task.md`](../../../../work/0052-worker-gateway-bypass/task.md)
- Gate: real-kind boundary probes with an oracle outside Agenova (Ollama host log)
- Date: probes 2026-10-05 01:57–03:16 UTC; baseline re-captured read-only 2026-10-06
- Branch/commit: probes ran against `main` `c56ba3a`; harness in `feat/52-worker-gateway-bypass`
- Target: `kind-agenova-k8s-lab` / `agenova-system` (pre-existing, disposable; not created or deleted by this run)
- Versions: kind v0.33.0, Kubernetes v1.37.0, kindnetd `v20260820-69b56db7`, Agent Sandbox controller v0.4.6, Ollama 0.34.4, Docker 29.8.0
- Raw output: [`output.txt`](output.txt); section numbers below refer to it

## Result

**Open gap.** A worker that has been running for a few seconds cannot reach the model provider, the Kubernetes API, the control plane or kube-dns: the generated NetworkPolicy blocks it and kindnet enforces that policy. A **freshly created** worker reached the provider directly within about one second of container start, in 3 of 3 runs, before the policy took effect ([#208](https://github.com/wunderforge/agenova/issues/208)). Public egress and public DNS are open by policy ([#202](https://github.com/wunderforge/agenova/issues/202)). No credential was found in the worker.

The sentence "the worker cannot bypass the Gateway" is therefore **not** supported on the kind reference until #208 is fixed and re-tested. Nothing here applies to EKS.

## Commands

```bash
C="--context kind-agenova-k8s-lab --namespace agenova-system"
bash harness/security/control-pod.sh up $C                       # control Pod, not selected by the policy
bash harness/security/probe-pod.sh $C --pod <pod> --marker <id>  # control Pod, warm worker, bound worker
bash harness/security/ollama-count.sh [--match <id>]             # provider-side oracle
bash harness/security/canary-scan.sh $C --pod <pod> --canary <text>
bash harness/security/control-pod.sh up $C --name e15-probe-deny --deny   # experimental second probe
bash harness/security/startup-window.sh $C --mode pool --marker <id>
bash harness/security/control-pod.sh down $C [--name e15-probe-deny]
```

The governed positive control is `agenova run -f deploy/reference/demo/work.yaml` with a fresh request name.

## Controls

| Control | Observation | Section |
|---|---|---|
| Governed run through the Gateway | `Allow`, `Succeeded`; Ollama log 38 → 42 (four chat completions); worker marker count 0 | 5 |
| Provider oracle attribution | Every client appears as `127.0.0.1` behind Docker Desktop, so each probe requests a unique path; the control Pod's request appears once in the Ollama log | 2, 3 |
| Target reachable without the policy | Control Pod (same image, without the template label) reaches Ollama, Kubernetes API, control-plane `:8080` and kube-dns | 3 |
| Probe tooling works in the worker | Worker reaches `1.1.1.1:443` and resolves `example.com`; `127.0.0.1:9` is refused immediately, so refusal and timeout are distinguishable | 4, 6 |
| kindnet enforces NetworkPolicy | Log line "Policy engine is ready"; an experimental default-deny egress policy blocked every target, including public egress, when probed 20 s after creation | 0, 10 |

## Network targets

`worker` covers the unbound warm-pool worker (section 4) and the worker bound to a held `SandboxClaim` that uses the adapter's `sandboxTemplateRef` and `warmpool` (section 6). Both gave identical results. The template label the policy selects stays on the Pod after binding.

| Target | Control Pod | Worker (running) | Classification |
|---|---|---|---|
| Ollama `192.168.65.254:11434` (by IP) | reachable, HTTP 404, log +1 | 3 s timeout, log +0 | Blocked by `agenova-tmpl-engineer-network-policy`, enforced by kindnet, **except at startup: open gap #208** |
| Kubernetes API `10.96.0.1:443`, node `172.18.0.2:6443` | reachable | timeout | Blocked by the same policy |
| Control plane `:8080` (Service and Pod IP) | reachable | timeout | Blocked by the same policy |
| Control plane Pod `:8081` | refused (loopback-only listener) | timeout | Blocked; also unreachable without the policy (ES-5) |
| kube-dns `10.96.0.10:53` | reachable | timeout | Blocked by the same policy |
| Metadata `169.254.169.254:80` | timeout | timeout | Inconclusive on kind: no metadata service exists |
| Public `1.1.1.1:443`, public DNS | reachable | **reachable** | Open by policy: exfiltration gap TM-1, #202. Not a Gateway bypass on kind |
| `host.docker.internal`, `kubernetes.default.svc` by name | NXDOMAIN | NXDOMAIN | Not a block: the worker uses `dnsPolicy: None` with public resolvers. By-IP results decide |

One real `agenova run` worker was probed while bound: its direct connection to Ollama timed out, then the claim ended and removed the Pod before the remaining targets ran (section 5). The full table therefore comes from the held claim.

## Startup probe (AC-9)

A claim that receives a freshly created worker Pod (the warm Pod is already bound) was probed from the moment the container accepted exec, once per second.

| Run | Pod | First request | Later requests | Ollama log |
|---|---|---|---|---|
| 1 | `…-knh45` | reached Ollama (HTTP 404) in the second the container started | 29 timeouts | 1 line with the run's marker |
| 2 | `…-9v7k5` | reached Ollama (HTTP 404) | 4 timeouts | 1 line |
| 3 | `…-x52pb` | reached Ollama (HTTP 404) | 4 timeouts | 1 line |

The policy object was 7 days old, so the gap is the time before it applies to a new Pod, not a missing object. Timing resolution is one second. A claim created without `warmpool` still bound an existing warm Pod on Agent Sandbox v0.4.6 (42 s old, all requests timed out), so that run is not a fresh-start datapoint (section 14).

## Credential exposure

| Check | Observation | Section |
|---|---|---|
| Worker Pod spec | ServiceAccount `default`, `automountServiceAccountToken=false`, no env, no volumes | 0, 8 |
| In-worker scan | No credential-like variable names, no token or cloud-credential paths, no extra mounts. Only Kubernetes service-link addresses are present | 8 |
| Canary A, planted in the worker | Found by the scan, before and after binding (scan works) | 8 |
| Canary B, control-plane env var and Secret | Present on the control-plane Pod; not found in a worker created afterwards, before or after binding | 7, 8 |
| ES-5 | No NodePort, LoadBalancer or Ingress. `:8080` serves only `/readyz` and `/v1/status` (platform name, revision, policy reference); `:8081` is unreachable from other Pods | 3, 11 |
| RI-2 | No Secrets besides the test canary, no credential-like strings in ConfigMaps, no `envFrom`, no local CLI config file | 12 |

## Matrix rows

| Row | Result on kind |
|---|---|
| AC-9 (network) | Open gap at worker startup (#208); blocked by a named mechanism once the policy applies |
| AC-4 (warm/network part), AC-6 | Warm worker holds no credential and reaches no protected target |
| AC-7 (kind scan), AC-8 | Control-plane canary not visible in the worker; no identity material in the worker |
| TM-1 | Open, #202 |
| TM-3 | Blocked for a running worker; startup not probed for these targets |
| ES-5 (worker side), RI-2 (exposure audit) | No finding |

## Limitations

- kind reference only. Docker Desktop on macOS, one node, kindnet. No result is extended to EKS, another CNI or another kindnet version.
- The startup probe was driven by `kubectl exec` from outside. Whether a real Agenova Work starts executing inside that first second was not established; the worker image allowlist was not changed to test it.
- The startup probe targeted Ollama only. Other protected targets were not probed at startup.
- The full target table uses a held `SandboxClaim`, not a claim issued by Agenova; a real run supplied one bound-worker datapoint.
- Metadata (`169.254.169.254`) cannot be assessed on kind.
- The default-deny policy was an E15 test object, removed after the run. It is not a Platform capability.
- The control-plane canary required a temporary env var on the control-plane Deployment. It was removed; `platform status` reported `changes: 0` afterwards (section 13).
