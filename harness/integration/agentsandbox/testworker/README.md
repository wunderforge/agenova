# Disposable Agent Sandbox test worker

This image is only for #51's kind integration test. It has no agent model,
external credentials, network service, or application outcome semantics.

Build at the repository root:

```sh
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
```

Use `agenova-testworker:kind` as the Agent Sandbox template image and
`["/agenova-workerctl", "serve"]` as its command. The container becomes Ready
without starting a task. From the allocated Pod's `agent` container, invoke:

```sh
kubectl exec pod/WORKER -c agent -- /agenova-workerctl start CLAIM
kubectl exec pod/WORKER -c agent -- /agenova-workerctl status CLAIM
kubectl exec pod/WORKER -c agent -- /agenova-workerctl stop CLAIM
kubectl exec pod/WORKER -c agent -- /agenova-workerctl status CLAIM
```

`start` waits for a real child process to calculate and report a deterministic
claim-bound probe result. The child stays alive so `stop` can kill it and wait
for confirmed exit without deleting the Pod. Before a start, `status` reports
`state=idle claim=CLAIM result=none` without reserving the worker; `stop` binds
that claim and reports `state=stopped claim=CLAIM result=none`, permanently
preventing a later start. Commands fail on a missing or wrong claim, duplicate
start, unavailable control server, or unconfirmed acknowledgement. The result
proves test-worker execution, **not** agent-task
success, runtime isolation, or production workload identity. One server owns
one claim for its lifetime; a pooled Pod must not reuse it for another claim.
