#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# E16 Slice 3 kind acceptance runner. Every subcommand needs an explicit kube
# context, install namespace, CLI state directory and output directory; none
# defaults to the current context. Subcommands run one campaign step each and
# record what they did under the output directory. See
# work/0178-governed-mcp-tools/slice3-kind-acceptance-plan.md.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
FIXTURE_NAMESPACE="agenova-e16"
CONTROL_PLANE_IMAGE="agenova-control-plane:0.1.0"
WORKER_IMAGE="agenova-testworker:kind"
FIXTURE_DEFAULT_IMAGE="agenova-e16-mcp:0.1.0"
PROBE_JOB="e16-probe"

CONTEXT=""
INSTALL_NAMESPACE=""
STATE_DIR=""
OUTPUT=""
YES=0

info() { printf '[info] %s\n' "$*"; }
pass() { printf '[pass] %s\n' "$*"; }
fail() { printf '[fail] %s\n' "$*" >&2; exit 1; }
run() { "$@"; }

usage() {
  cat <<'EOF'
Usage: bash harness/integration/e16/campaign.sh --context <kind-context> \
         --install-namespace <namespace> --state-dir <dir> --output <dir> [--yes] <subcommand> [args]

Read-only:
  preflight          Check tools, cluster ownership inputs, namespaces and other
                     installs that share the fixed image tags.
  status             Print the recorded campaign files.
Mutating (each prints its plan; protect, load and restore also need --yes):
  protect            Record, export and stop other installs that use the fixed
                     control-plane and worker image tags.
  build              Build the control-plane, worker, fixture and probe images.
  load               Load the built images into the kind node and verify them.
  fixture            Deploy the MCP fixture and start complete log capture.
  install            Install the E16 Platform, Policy and AgentTemplate.
  work <name>        Run harness/integration/e16/work-<name>.yaml and record it.
  probe              Run the probe Job and check it against the server log.
  parity             Compare CLI, API and Portal for the recorded Works
                     (positive, n8-truncation, n6-timeout, admission-deny).
  restore            Re-import protected images and restart other installs.
EOF
}

kctl() { run kubectl --context "$CONTEXT" "$@"; }
cluster_name() { printf '%s' "${CONTEXT#kind-}"; }
node_name() { printf '%s-control-plane' "$(cluster_name)"; }
source_sha() { git -C "$ROOT" rev-parse HEAD; }
short_sha() { git -C "$ROOT" rev-parse --short=12 HEAD; }
fixture_image() { printf 'agenova-e16-mcp:%s' "$(short_sha)"; }
probe_image() { printf 'agenova-e16-probe:%s' "$(short_sha)"; }
cli() { printf '%s/bin/agenova' "$OUTPUT"; }
stamp() { date -u +%Y-%m-%dT%H:%M:%SZ; }
record() { printf '%s %s\n' "$(stamp)" "$*" >>"$OUTPUT/campaign.log"; }

require_args() {
  [ -n "${CONTEXT// /}" ] || fail "--context is required; the current context is never used"
  [ -n "${INSTALL_NAMESPACE// /}" ] || fail "--install-namespace is required"
  [ -n "${STATE_DIR// /}" ] || fail "--state-dir is required"
  [ -n "${OUTPUT// /}" ] || fail "--output is required"
  case "$CONTEXT" in kind-?*) ;; *) fail "--context must name a kind context (kind-<cluster>)" ;; esac
  case "$INSTALL_NAMESPACE" in default|kube-*|"$FIXTURE_NAMESPACE") fail "install namespace $INSTALL_NAMESPACE is reserved" ;; esac
  mkdir -p "$OUTPUT" "$STATE_DIR"
}

require_yes() {
  [ "$YES" -eq 1 ] || fail "$1 changes cluster state outside the E16 namespaces; rerun with --yes after reviewing the plan above"
}

# Platform inputs must target exactly the context and namespace given here.
check_platform_targets() {
  local platform="$SCRIPT_DIR/platform.yaml"
  grep -Eq "^[[:space:]]+context: ${CONTEXT}\$" "$platform" || fail "platform.yaml does not target $CONTEXT"
  [ "$(grep -Ec "^[[:space:]]+namespace: ${INSTALL_NAMESPACE}\$" "$platform")" -eq 2 ] ||
    fail "platform.yaml control plane and runtime namespaces must both be $INSTALL_NAMESPACE"
}

# Objects outside the E16 namespaces that run the fixed tags, as
# "kind namespace name replicas" lines.
other_installs() {
  kctl get deployments -A -o jsonpath='{range .items[*]}Deployment {.metadata.namespace} {.metadata.name} {.spec.replicas} {.spec.template.spec.containers[*].image}{"\n"}{end}' |
    awk -v cp="$CONTROL_PLANE_IMAGE" -v ns="$INSTALL_NAMESPACE" '$2 != ns && index($0, cp) {print $1, $2, $3, $4}'
  local pool
  kctl get sandboxwarmpools -A -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name} {.spec.replicas}{"\n"}{end}' 2>/dev/null |
    while read -r namespace name replicas; do
      [ -n "$namespace" ] && [ "$namespace" != "$INSTALL_NAMESPACE" ] || continue
      pool="$(kctl -n "$namespace" get sandboxes -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].name=='$name')]}{.spec.podTemplate.spec.containers[*].image}{\"\\n\"}{end}" 2>/dev/null || true)"
      if [ -z "$pool" ] || printf '%s' "$pool" | grep -q "$WORKER_IMAGE"; then
        printf 'SandboxWarmPool %s %s %s\n' "$namespace" "$name" "${replicas:-0}"
      fi
    done
}

node_image_id() {
  run docker exec "$(node_name)" crictl inspecti -o go-template --template '{{.status.id}}' "docker.io/library/$1" 2>/dev/null || true
}

preflight() {
  local cmd
  for cmd in docker kind kubectl go git; do command -v "$cmd" >/dev/null 2>&1 || fail "missing prerequisite: $cmd"; done
  run docker info >/dev/null 2>&1 || fail "Docker daemon is unreachable"
  run kind get clusters | grep -qx "$(cluster_name)" || fail "kind cluster $(cluster_name) does not exist"
  kctl get nodes >/dev/null || fail "context $CONTEXT is unreachable"
  check_platform_targets
  [ -z "$(git -C "$ROOT" status --porcelain)" ] || fail "working tree is not clean; evidence must name one source commit"
  {
    echo "source $(source_sha)"
    echo "context $CONTEXT"
    echo "install-namespace $INSTALL_NAMESPACE"
    echo "node $(node_name) created $(docker inspect -f '{{.Created}}' "$(node_name)")"
    echo "node-image $CONTROL_PLANE_IMAGE $(node_image_id "$CONTROL_PLANE_IMAGE")"
    echo "node-image $WORKER_IMAGE $(node_image_id "$WORKER_IMAGE")"
    echo "namespace $INSTALL_NAMESPACE: $(kctl get namespace "$INSTALL_NAMESPACE" --no-headers 2>&1 | head -1)"
    echo "namespace $FIXTURE_NAMESPACE: $(kctl get namespace "$FIXTURE_NAMESPACE" --no-headers 2>&1 | head -1)"
    echo "other installs sharing fixed tags:"
    other_installs | sed 's/^/  /'
  } | tee "$OUTPUT/preflight.txt"
  record "preflight"
  pass "preflight recorded in $OUTPUT/preflight.txt"
}

protect() {
  local state="$OUTPUT/protect-state.txt" archive kind namespace name replicas image
  [ ! -s "$state" ] || fail "$state exists; run restore before protecting again"
  other_installs >"$OUTPUT/protect-plan.txt"
  info "plan: export node images for $CONTROL_PLANE_IMAGE and $WORKER_IMAGE, then scale to zero:"
  sed 's/^/  /' "$OUTPUT/protect-plan.txt"
  require_yes protect
  mkdir -p "$OUTPUT/protected-images"
  # Record first, so a failure part-way can still be restored.
  : >"$state"
  for image in "$CONTROL_PLANE_IMAGE" "$WORKER_IMAGE"; do
    archive="$OUTPUT/protected-images/${image//[:\/]/_}.tar"
    printf 'image %s %s %s\n' "$image" "$(node_image_id "$image")" "$archive" >>"$state"
  done
  while read -r kind namespace name replicas; do
    [ -n "$kind" ] && printf 'scale %s %s %s %s\n' "$kind" "$namespace" "$name" "$replicas" >>"$state"
  done <"$OUTPUT/protect-plan.txt"
  while read -r kind image id archive; do
    [ "$kind" = image ] || continue
    [ -n "$id" ] || { info "no node image for $image; nothing to protect"; continue; }
    run docker exec "$(node_name)" ctr -n k8s.io images export "/tmp/e16-protect.tar" "docker.io/library/$image"
    run docker cp "$(node_name):/tmp/e16-protect.tar" "$archive"
    run docker exec "$(node_name)" rm -f /tmp/e16-protect.tar
  done <"$state"
  while read -r kind object namespace name replicas; do
    [ "$kind" = scale ] || continue
    case "$object" in
      Deployment) kctl -n "$namespace" scale deployment "$name" --replicas=0 ;;
      SandboxWarmPool) kctl -n "$namespace" patch sandboxwarmpool "$name" --type merge -p '{"spec":{"replicas":0}}' ;;
    esac
  done <"$state"
  record "protect $(wc -l <"$state" | tr -d ' ') entries"
  pass "other installs stopped and their images exported; run restore after the campaign"
}

restore() {
  local state="$OUTPUT/protect-state.txt" kind image id archive object namespace name replicas now
  [ -s "$state" ] || { info "no protect state; nothing to restore"; return 0; }
  info "plan: re-import protected images and restore recorded replicas:"
  sed 's/^/  /' "$state"
  require_yes restore
  while read -r kind image id archive; do
    [ "$kind" = image ] && [ -n "$id" ] || continue
    run kind load image-archive "$archive" --name "$(cluster_name)"
    now="$(node_image_id "$image")"
    [ "$now" = "$id" ] || fail "restored $image is $now, recorded $id"
  done <"$state"
  while read -r kind object namespace name replicas; do
    [ "$kind" = scale ] || continue
    case "$object" in
      Deployment) kctl -n "$namespace" scale deployment "$name" --replicas="$replicas" ;;
      SandboxWarmPool) kctl -n "$namespace" patch sandboxwarmpool "$name" --type merge -p "{\"spec\":{\"replicas\":$replicas}}" ;;
    esac
  done <"$state"
  mv "$state" "$state.restored-$(date -u +%Y%m%dT%H%M%SZ)"
  record "restore"
  pass "protected images and replicas restored"
}

image_line() { # name tag
  local id archive
  id="$(run docker image inspect -f '{{.Id}}' "$2")"
  archive="$OUTPUT/images/${2//[:\/]/_}.tar"
  run docker save -o "$archive" "$2"
  printf '%s tag=%s content=%s archive-sha256=%s\n' "$1" "$2" "$id" "$(shasum -a 256 "$archive" | cut -d' ' -f1)"
}

# The probe tag must appear only in the probe binary's build settings.
check_build_tags() {
  local dir="$OUTPUT/binaries" container
  mkdir -p "$dir"
  container="$(run docker create "$CONTROL_PLANE_IMAGE")"
  run docker cp "$container:/agenova-control-plane" "$dir/agenova-control-plane"
  run docker rm "$container" >/dev/null
  container="$(run docker create "$(probe_image)")"
  run docker cp "$container:/e16-probe.test" "$dir/e16-probe.test"
  run docker rm "$container" >/dev/null
  run go version -m "$dir/agenova-control-plane" >"$dir/control-plane.buildinfo"
  run go version -m "$dir/e16-probe.test" >"$dir/probe.buildinfo"
  grep -q 'build' "$dir/control-plane.buildinfo" || fail "control-plane build information is unreadable"
  ! grep -q 'agenovaprobe' "$dir/control-plane.buildinfo" || fail "control-plane binary was built with the probe tag"
  grep -q -- '-tags=agenovaprobe' "$dir/probe.buildinfo" || fail "probe binary lacks the probe tag"
  (cd "$ROOT" && run go list -f '{{.GoFiles}}' ./internal/console) | grep -vq probe_hooks.go || fail "untagged build selects probe_hooks.go"
  pass "probe tag present only in the probe binary"
}

build() {
  [ -z "$(git -C "$ROOT" status --porcelain)" ] || fail "working tree is not clean"
  info "plan: build $CONTROL_PLANE_IMAGE, $WORKER_IMAGE, $(fixture_image), $(probe_image) and the CLI from $(source_sha)"
  mkdir -p "$OUTPUT/images" "$OUTPUT/bin"
  run docker build -f "$ROOT/deploy/reference/Dockerfile" -t "$CONTROL_PLANE_IMAGE" "$ROOT"
  run docker build -f "$ROOT/harness/integration/agentsandbox/testworker/Dockerfile" -t "$WORKER_IMAGE" "$ROOT"
  run docker build -f "$ROOT/harness/integration/mcpfixture/Dockerfile" -t "$(fixture_image)" "$ROOT/harness/integration/mcpfixture"
  run docker build -f "$SCRIPT_DIR/probe.Dockerfile" -t "$(probe_image)" "$ROOT"
  (cd "$ROOT" && GOTOOLCHAIN=go1.22.12 run go build -o "$(cli)" ./cmd/agenova)
  {
    echo "source $(source_sha)"
    echo "builder golang:1.22-alpine $(run docker image inspect -f '{{index .RepoDigests 0}}' golang:1.22-alpine 2>/dev/null || echo unresolved)"
    image_line control-plane "$CONTROL_PLANE_IMAGE"
    image_line worker "$WORKER_IMAGE"
    image_line fixture "$(fixture_image)"
    image_line probe "$(probe_image)"
    echo "cli $(GOTOOLCHAIN=go1.22.12 go version -m "$(cli)" | head -1)"
  } | tee "$OUTPUT/build-identity.txt"
  check_build_tags | tee -a "$OUTPUT/build-identity.txt"
  record "build $(source_sha)"
}

load() {
  local state="$OUTPUT/protect-state.txt" image want got
  if [ -n "$(other_installs)" ] && [ ! -s "$state" ]; then
    fail "other installs share the fixed image tags; run protect first"
  fi
  info "plan: kind load $CONTROL_PLANE_IMAGE $WORKER_IMAGE $(fixture_image) $(probe_image) into $(cluster_name)"
  require_yes load
  for image in "$CONTROL_PLANE_IMAGE" "$WORKER_IMAGE" "$(fixture_image)" "$(probe_image)"; do
    run kind load docker-image "$image" --name "$(cluster_name)"
    want="$(run docker image inspect -f '{{.Id}}' "$image")"
    got="$(node_image_id "$image")"
    [ "$want" = "$got" ] || fail "node image for $image is $got, local content is $want"
    printf 'node-image %s %s\n' "$image" "$got" | tee -a "$OUTPUT/load-identity.txt"
  done
  record "load"
}

pod_identity() { # namespace selector
  kctl -n "$1" get pods -l "$2" -o jsonpath='{range .items[*]}{.metadata.name} uid={.metadata.uid} restarts={.status.containerStatuses[0].restartCount} container={.status.containerStatuses[0].containerID} imageID={.status.containerStatuses[0].imageID}{"\n"}{end}'
}

fixture() {
  local rendered="$OUTPUT/fixture-rendered.yaml"
  info "plan: apply the MCP fixture to $FIXTURE_NAMESPACE with $(fixture_image) and capture its log"
  kctl kustomize "$ROOT/harness/integration/mcpfixture" | sed "s#image: ${FIXTURE_DEFAULT_IMAGE}#image: $(fixture_image)#" >"$rendered"
  grep -q "image: $(fixture_image)" "$rendered" || fail "fixture image override did not apply"
  kctl apply -f "$rendered"
  kctl -n "$FIXTURE_NAMESPACE" rollout status deployment/e16-mcp --timeout=120s
  pod_identity "$FIXTURE_NAMESPACE" app.kubernetes.io/name=e16-mcp | tee "$OUTPUT/fixture-identity-start.txt"
  nohup kubectl --context "$CONTEXT" -n "$FIXTURE_NAMESPACE" logs -f deployment/e16-mcp >"$OUTPUT/fixture-follow.jsonl" 2>"$OUTPUT/fixture-follow.err" &
  echo $! >"$OUTPUT/fixture-follow.pid"
  record "fixture $(fixture_image)"
  pass "fixture running; log capture pid $(cat "$OUTPUT/fixture-follow.pid")"
}

install() {
  local id
  check_platform_targets
  info "plan: adapters install, platform validate/plan/apply/status, policy and template apply, identical reapply"
  for id in agenova.io/deployment/kubernetes agenova.io/runtime/agent-sandbox agenova.io/model/openai-compatible agenova.io/tool/mcp-http; do
    run "$(cli)" --state-dir "$STATE_DIR" adapters install "$id"
  done
  run "$(cli)" --state-dir "$STATE_DIR" platform validate -f "$SCRIPT_DIR/platform.yaml"
  run "$(cli)" --state-dir "$STATE_DIR" platform plan -f "$SCRIPT_DIR/platform.yaml"
  run "$(cli)" --state-dir "$STATE_DIR" platform apply -f "$SCRIPT_DIR/platform.yaml" --yes
  run "$(cli)" --state-dir "$STATE_DIR" platform status
  run "$(cli)" --state-dir "$STATE_DIR" policy apply -f "$SCRIPT_DIR/policy.yaml"
  run "$(cli)" --state-dir "$STATE_DIR" agent-template apply -f "$SCRIPT_DIR/template.yaml"
  run "$(cli)" --state-dir "$STATE_DIR" platform plan -f "$SCRIPT_DIR/platform.yaml" | tee "$OUTPUT/reapply-plan.txt"
  pod_identity "$INSTALL_NAMESPACE" app.kubernetes.io/name=agenova-control-plane | tee "$OUTPUT/control-plane-identity.txt"
  record "install"
}

work() {
  local name="${1:-}" file ref out
  [ -n "$name" ] || fail "work needs a name, for example: work positive"
  file="$SCRIPT_DIR/work-$name.yaml"
  [ -f "$file" ] || fail "no $file"
  ref="e16-$name"
  out="$OUTPUT/work/$name"
  mkdir -p "$out"
  # Capture worker identity while the Work runs, before cleanup.
  (for _ in $(seq 1 600); do
    pod_identity "$INSTALL_NAMESPACE" agents.x-k8s.io/sandbox-template-ref-hash >>"$out/worker-identity.txt" 2>/dev/null || true
    sleep 1
  done) &
  local watcher=$!
  set +e
  run "$(cli)" --state-dir "$STATE_DIR" run -f "$file" >"$out/run.txt" 2>&1
  echo "run-exit $?" >"$out/exit-codes.txt"
  run "$(cli)" --state-dir "$STATE_DIR" work show "$ref" --json >"$out/work-show.json" 2>"$out/work-show.err"
  echo "show-exit $?" >>"$out/exit-codes.txt"
  set -e
  kill "$watcher" 2>/dev/null || true
  sort -u "$out/worker-identity.txt" -o "$out/worker-identity.txt" 2>/dev/null || true
  record "work $name $(tr '\n' ' ' <"$out/exit-codes.txt")"
  cat "$out/exit-codes.txt"
}

# A Job that exits 0 can still have skipped or emitted nothing.
validate_probe_output() {
  local file="$1"
  grep -q -- '--- PASS: TestE16Probes ' "$file" || fail "probe test did not pass (skipped, failed or absent)"
  ! grep -q -- '--- SKIP' "$file" || fail "probe output contains a skipped test"
  grep -q '^E16_PROBE ' "$file" || fail "probe emitted no receipts"
}

probe() {
  local out="$OUTPUT/probe" rendered before after
  mkdir -p "$out"
  info "plan: run Job $PROBE_JOB in $INSTALL_NAMESPACE with $(probe_image) and check it against the fixture log"
  kctl -n "$INSTALL_NAMESPACE" create configmap e16-probe-inputs --from-file=template.yaml="$SCRIPT_DIR/template.yaml" --from-file=policy.yaml="$SCRIPT_DIR/policy.yaml" --dry-run=client -o yaml | kctl apply -f -
  kctl -n "$INSTALL_NAMESPACE" delete job "$PROBE_JOB" --ignore-not-found --wait=true
  rendered="$out/probe-job.yaml"
  sed "s#image: PROBE_IMAGE#image: $(probe_image)#" "$SCRIPT_DIR/probe-job.yaml" >"$rendered"
  before="$(pod_identity "$FIXTURE_NAMESPACE" app.kubernetes.io/name=e16-mcp)"
  kctl -n "$INSTALL_NAMESPACE" apply -f "$rendered"
  kctl -n "$INSTALL_NAMESPACE" wait --for=condition=complete --timeout=900s "job/$PROBE_JOB" || true
  kctl -n "$INSTALL_NAMESPACE" logs "job/$PROBE_JOB" >"$out/probe-job.log"
  after="$(pod_identity "$FIXTURE_NAMESPACE" app.kubernetes.io/name=e16-mcp)"
  printf 'before %s\nafter  %s\n' "$before" "$after" >"$out/fixture-continuity.txt"
  [ -n "$before" ] && [ "$before" = "$after" ] || fail "fixture Pod identity or restart count changed during the probe run"
  kctl -n "$FIXTURE_NAMESPACE" logs deployment/e16-mcp >"$out/fixture-full.jsonl"
  validate_probe_output "$out/probe-job.log"
  (cd "$ROOT" && GOTOOLCHAIN=go1.22.12 run go run ./harness/integration/e16/evidence -receipts "$out/probe-job.log" -server-log "$out/fixture-full.jsonl") >"$out/evidence.jsonl"
  record "probe pass"
  pass "probe receipts match the fixture server log"
}

parity() {
  local out="$OUTPUT/parity" api ui rc
  mkdir -p "$out"
  info "plan: agenova api connect, UI dev server on 5177, run ui/installed/mcp.spec.ts"
  "$(cli)" --state-dir "$STATE_DIR" api connect >"$out/api-connect.log" 2>&1 &
  api=$!
  npm --prefix "$ROOT/ui" run dev -- --port 5177 --strictPort >"$out/ui-dev.log" 2>&1 &
  ui=$!
  trap 'kill "$api" "$ui" 2>/dev/null || true' RETURN
  sleep 5
  set +e
  (cd "$ROOT/ui" && AGENOVA_CLI_PATH="$(cli)" AGENOVA_CLI_STATE_DIR="$STATE_DIR" \
    AGENOVA_E16_POSITIVE_REF=e16-positive AGENOVA_E16_TRUNCATION_REF=e16-n8-truncation \
    AGENOVA_E16_FAILURE_REF=e16-n6-timeout AGENOVA_E16_DENIED_REF=e16-admission-deny \
    run npx playwright test --config playwright.installed.config.ts installed/mcp.spec.ts --output "$out/playwright") >"$out/playwright.log" 2>&1
  rc=$?
  set -e
  record "parity exit $rc"
  [ "$rc" -eq 0 ] || fail "CLI, API and Portal parity failed; see $out/playwright.log"
  pass "CLI, API and Portal agree for the recorded Works"
}

status() { ls -la "$OUTPUT"; [ -f "$OUTPUT/campaign.log" ] && cat "$OUTPUT/campaign.log"; }

main() {
  local subcommand=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --context) CONTEXT="${2:-}"; shift 2 ;;
      --install-namespace) INSTALL_NAMESPACE="${2:-}"; shift 2 ;;
      --state-dir) STATE_DIR="${2:-}"; shift 2 ;;
      --output) OUTPUT="${2:-}"; shift 2 ;;
      --yes) YES=1; shift ;;
      -h|--help) usage; return 0 ;;
      -*) usage; fail "unknown flag: $1" ;;
      *) subcommand="$1"; shift; break ;;
    esac
  done
  [ -n "$subcommand" ] || { usage; exit 2; }
  require_args
  OUTPUT="$(cd "$OUTPUT" && pwd)"
  STATE_DIR="$(cd "$STATE_DIR" && pwd)"
  cd "$ROOT"
  case "$subcommand" in
    preflight) preflight ;;
    protect) protect ;;
    build) build ;;
    load) load ;;
    fixture) fixture ;;
    install) install ;;
    work) work "$@" ;;
    probe) probe ;;
    parity) parity ;;
    restore) restore ;;
    status) status ;;
    *) usage; fail "unknown subcommand: $subcommand" ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
