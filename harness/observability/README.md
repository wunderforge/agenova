# Durable evidence and OTEL reference verification

This is a single-control-plane kind reference installation. It is not HA, safe
active-Work rollout or E14 authentication evidence. Work interrupted by restart
is not resumed and old authority is not restored. Keep #180 open until real E14
users pass query-isolation acceptance.

## Install

Use `deploy/reference/platform.observability.kind.yaml`, adjusting the existing
cluster context, dedicated namespace, storage class and logical model mapping.
The five telemetry components are deployed separately. The configured model must
exist on the local Ollama server. The backend telemetry defaults are seven days;
SQLite evidence is not automatically pruned.

Create the dedicated namespace and a `grafana-admin` Kubernetes Secret containing
`username` and `password`, using the operator's trusted secret provisioning path.
Do not put the password in Platform YAML, a command transcript or a screenshot.
The install checks Secret metadata without reading its contents. Secret access
is not granted to workers or the control-plane service account.

Build `deploy/reference/Dockerfile` as `agenova-control-plane:0.1.180` and load it
and `agenova-testworker:kind` into the explicitly selected kind cluster. Then:

```sh
agenova platform validate -f deploy/reference/platform.observability.kind.yaml
agenova platform plan -f deploy/reference/platform.observability.kind.yaml
agenova platform apply -f deploy/reference/platform.observability.kind.yaml --yes
agenova platform plan -f deploy/reference/platform.observability.kind.yaml
agenova platform status
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/work.yaml
agenova work history --json
agenova work show investigate-payment-retries --json
agenova api connect
```

Use a separate state directory when another reference Platform already exists.
`work history [cursor] --json` returns bounded authorized summaries; the Portal
Work page links to a paginated History view. Existing list/item wire formats
remain supported. The current installed reference viewer is explicitly fixed
and loopback-only; trusted E14 viewer/authorizer integration is a separate gate.

## Grafana and signals

```sh
kubectl --context kind-agenova-k8s-lab -n agenova-observability-180 port-forward service/agenova-grafana 3000:3000 --address 127.0.0.1
```

Open `http://127.0.0.1:3000`, authenticate using the operator-provisioned Secret,
and open **Agenova Work diagnostics**. Grafana has Tempo, Loki and Prometheus
datasources. The Portal trace link defaults to that loopback URL; build-time
`VITE_GRAFANA_URL` can select another loopback port. Queries and dashboards are
diagnostic surfaces, never the governance source of truth.

Backend queries must confirm all three signals rather than treating Ready Pods
as successful ingestion. Query a Work's saved trace ID in Tempo, logs for
`service_name="agenova-control-plane"` in Loki, and
`agenova_operations_total` in Prometheus. Signal-loss health includes
`agenova_telemetry_export_failures_total` and Collector refusal/export counters.

## Required verification

- Run success, admission denial, provider failure and runtime failure. Preserve
  CLI/API/Portal parity and exact identities, then replace the control-plane Pod
  after terminal Work and query the same records again.
- Crash after a committed provider attempt: retain unknown outcome; no replay,
  no old claim in the authority registry, and cleanup uncertainty is explicit.
- Stop Collector, run another Work and verify its authoritative result/evidence;
  restore Collector and verify later signal delivery.
- Check first/repeated apply, backend restarts and storage/RBAC negatives.
- Keep credentials, authentication headers, raw task/model/tool bodies out of
  telemetry and captured artifacts. Use synthetic tasks for rendered proof.
- Run focused tests and `pwsh -NoProfile -File ./scripts/check.ps1 -All`.

## Cleanup and limits

Delete only the named observability Deployments/Services/ConfigMaps created in
the dedicated test namespace; retain the evidence/backend PVCs and the namespace
by default. Do not delete the namespace as a routine cleanup step because that
also deletes its PVCs. Complete active Work before stopping the control plane.
Explicitly deleting the five `*-data` PVCs is a separate destructive operation
and loses evidence/telemetry; inspect the selected context/namespace first.

The generic repository/query interfaces hide SQLite and Kubernetes details.
The reference service retains its bounded 32-request session capacity; complete
active Work before an explicit restart to begin a new session. Historical
records remain queryable across that restart. This is not a production throughput
claim.

The current driver uses a process-level exclusive file lock plus SQLite WAL and
FULL synchronous writes on a local PVC. No network filesystem, multiple writer,
backup/restore, production upgrade or production retention promise is made.
