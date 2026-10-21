# Reproducing the candidate probes

The replay driver operates on an **already installed disposable reference service**. It does not install a CNI, create/delete a cluster, stop a provider or provide an old-token HTTP API. Obtain the same explicit substrate authorization before preparing a new cluster. Never target member/EKS or a shared reference namespace.

## Prepare the measured substrate

The measured setup used a separately owned kind cluster, its own kubeconfig, Kubernetes v1.37.0 arm64, disabled default CNI, Cilium chart 1.20.3 and Agent Sandbox v0.4.6. The recorded kind and service configuration is under `setup/`; the exact test-only setup/probe sources are under `provenance/` as `.txt`. Helm 3.19.0 was downloaded to the workspace and SHA256-checked; the official Cilium chart used `ipam.mode=kubernetes`, `policyEnforcementMode=always`, `kubeProxyReplacement=false`, one operator, `envoy.enabled=false` and `socketLB.enabled=false`.

Consult the [official Cilium kind procedure](https://docs.cilium.io/en/stable/installation/kind/) and [policy modes](https://docs.cilium.io/en/stable/security/policy/intro/), then independently verify fresh-Pod enforcement against a reachable control before using `network-policy-mode: strict` or `--verified-strict-cni`. This is an operator attestation, not a self-certifying test switch. Do not turn ordinary kindnet into a claimed strict substrate by adding the declaration.

Use the repository [pinned Agent Sandbox substrate](../../../../harness/spike/agent-sandbox-substrate/RUNBOOK.md) as version guidance, without running its fixed-name reference-cluster creation/cleanup against an unrelated cluster. Save the created Docker node ID and kube-system UID in a private receipt:

```json
{"cluster":"YOUR-OWNED-CLUSTER","containerId":"EXACT-CREATED-DOCKER-ID","kubeSystemUID":"EXACT-KUBE-SYSTEM-UID"}
```

No receipt authorizes adopting an existing unrelated cluster. The driver checks receipt identities and explicit context, as well as CLI Platform status target/readiness. Retain kubeconfig and CLI state privately; neither belongs in evidence.

Build the [Control Plane](../../../../deploy/reference/Dockerfile), [scripted/reference Worker](../../../../harness/security/scriptedworker/Dockerfile) and [MCP fixture](../../../../harness/integration/mcpfixture/Dockerfile) from the desired source, with the fixture directory as its build context. Load into only the owned kind node. Record immutable loaded digests and use those references in Platform/template configuration; historical local image aliases in this archive cannot be pulled from a registry.

Adapt the context/namespace, endpoint and image references from `setup/platform.yaml` to the new owned installation. Keep the same logical template, model/profile and tool resource mappings if using these scenarios. Cilium system/service allow policies must not select Worker Pods. Register the baseline Policy/template through the regular CLI. The fixture's credential-free `/mcp` path is used; its Pod still requires a synthetic token Secret for a separate optional route. Create only synthetic material from stdin, do not copy a real Secret or archive its value.

## Run a bounded group

Build the CLI from the desired revision. Supply a **fresh prefix for each execution**, a new output directory, a private ownership receipt, actual Ollama log path, baseline Policy v2 and cap Policy v3. The cap Policy must differ only by the explicit resource ceiling and immutable version, as shown in `raw/group1/policy-cap.yaml`. The driver restores the baseline version after cap probes. Do not retry an unknown submission with another ID before investigating its result.

Example from repository root (replace every uppercase path/target value; group 1, 2 or 3):

```sh
python3 harness/security/run-demo-groups.py \
  --execute --verified-strict-cni --group 3 \
  --cli /ABS/PATH/agenova --state-dir /ABS/PATH/private-state \
  --kubeconfig /ABS/PATH/private-kubeconfig \
  --owner-receipt /ABS/PATH/private-owner.json \
  --context kind-YOUR-OWNED-CLUSTER --namespace YOUR-TEST-NAMESPACE \
  --policy /ABS/PATH/baseline-policy.yaml \
  --policy-cap /ABS/PATH/cap-policy.yaml \
  --prefix e15-new-unique-prefix \
  --ollama-log /ABS/PATH/ollama-server.log \
  --output /ABS/PATH/new-results-directory
```

All groups require a healthy model/tool control and no unrelated provider traffic. Group 1 runs A/B, one forced foreign-scope probe, two preset inductions and a cap probe. Group 2 measures three terminal states and quiet windows only; it never asserts installed old-token replay. Group 3 distinguishes admission Deny, resolution 422 and authority narrowing. On an unexpected counter mismatch, failed positive or ambiguous watch, stop and inspect the retained results; do not convert a missing observation to a pass.

## Verify the retained historical packet

```sh
python3 harness/security/verify-demo-evidence.py docs/evidence/198/candidate-20261022
```

This verifies the committed candidate packet and its named limitations offline. It is not a generic verdict generator for future behavior. Future runs need fresh review of actual outcomes and fixed/remaining gaps.

Clean up only resources matching the saved ownership receipt and namespace UIDs; preserve failures before removal. Do not reuse the old reference cleanup script for the separate Cilium cluster. CNI/backend claims remain limited to actually measured paths.
