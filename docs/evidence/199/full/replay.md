# Live replay of #199

Use a fresh local checkout of main `22355d5` plus this harness. Source-based image builds and the reference installation guide are prerequisites. This is opt-in destructive testing of a dedicated disposable kind cluster only.

1. Refuse an existing `agenova-e15-199-faults` cluster or `agenova-e15-199-full-provider` container. Create kind with the same node image recorded in ../full/original-node-identity.json, name `agenova-e15-199-faults` and `--kubeconfig .tmp/faults.kubeconfig`. Use an independent kubeconfig on **all** subsequent calls. Save its Docker node ID and kube-system UID before preparing resources.
2. Apply the pinned upstream Agent Sandbox v0.4.6 manifest.yaml and extensions.yaml using explicit `--context kind-agenova-e15-199-faults --kubeconfig .tmp/faults.kubeconfig`. Wait for the real controller Deployment; do not mutate the original kind controller. The known manifest hashes are retained alongside this report.
3. Build the repository control-plane and testworker Dockerfiles and load `agenova-control-plane:0.1.0` / `agenova-testworker:kind` with `kind load docker-image ... --name agenova-e15-199-faults`. Capture image identities and actual Pod imageIDs.
4. Start a dedicated real Ollama 0.13.5 container named `agenova-e15-199-full-provider` with label `agenova.ticket=199`, only loopback 11439:11434 and the existing models directory mounted read-only. Save its full container ID. Do not stop the host Ollama. Ensure llama3.1:latest exists and is reachable from the dedicated kind node.
5. Create `agenova-e15-199-full` and `agenova-e15-199-controller` namespaces, saving create response UIDs. With `KUBECONFIG` set to the independent file, apply full.platform.yaml and controller.platform.yaml using separate state dirs `.tmp/faults/state-full` and `.tmp/faults/state-controller`; register deploy/reference/demo/policy.yaml and engineer.yaml in each. Save owner.json containing exactly `node`, `kubeSystemUID`, `providerID` for the resources just created; never reuse this run's IDs.

```bash
go build -o .tmp/agenova ./cmd/agenova
go build -o .tmp/faults/terminalprobe ./harness/security/terminalprobe
python3 harness/security/component-failures.py --output .tmp/faults/replay-campaign \
  --owner .tmp/faults/replay-owner.json --kubeconfig .tmp/faults.kubeconfig
python3 harness/security/controller-timeout.py --output .tmp/faults/replay-cold \
  --owner .tmp/faults/replay-owner.json --kubeconfig .tmp/faults.kubeconfig
kubectl --context kind-agenova-e15-199-faults --kubeconfig .tmp/faults.kubeconfig \
  create namespace agenova-e15-199-terminal -o json
# Run for each mode: succeeded, failed, expired. Use the fresh recorded Provider ID.
.tmp/faults/terminalprobe --mode succeeded --container-id <fresh-full-provider-id> \
  --kubeconfig <absolute-independent-kubeconfig> --output .tmp/faults/replay-terminal/succeeded.json
python3 harness/security/provider-outage.py --container-id <fresh-full-provider-id> \
  --context kind-agenova-e15-199-faults --kubeconfig .tmp/faults.kubeconfig \
  --state-dir .tmp/faults/state-full --output .tmp/faults/replay-provider
```

The cold-controller harness reads the captured controller.platform.yaml; its context is fixed to the disposable cluster. The component scripts use loopback ports 18099, 18100 and 18101 and fail if unavailable. Their finally blocks restore a paused provider/controller and stop their own port forwards. Run campaigns sequentially to keep Provider counter windows isolated.

After collecting late residual-resource snapshots, delete only namespaces created by this replay using their recorded UID preconditions, then wait for deletion. Verify both kube-system UID and Docker node ID before deleting the dedicated kind cluster with KUBECONFIG pointing at its independent file. Verify the dedicated Provider's label/name/full ID before stopping/removing it. Keep host model files and reusable images. Save absence receipts and confirm original reference Pod UIDs and host Ollama health. Do not package kubeconfig or private keys with evidence.
