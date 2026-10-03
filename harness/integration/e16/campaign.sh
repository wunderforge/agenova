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
MODEL_PROFILE=""
YES=0

info() { printf '[info] %s\n' "$*"; }
pass() { printf '[pass] %s\n' "$*"; }
fail() { printf '[fail] %s\n' "$*" >&2; exit 1; }
run() { "$@"; }

usage() {
  cat <<'EOF'
Usage: bash harness/integration/e16/campaign.sh --context <kind-context> \
         --install-namespace <namespace> --state-dir <dir> --output <dir> \
         --model-profile <profile> [--yes] <subcommand> [args]

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
  work <name>        Run harness/integration/e16/work-<name>.yaml, check it
                     against the server log and its answer oracle, archive parity.
  probe              Run the probe Job and check it against the server log.
  parity <case>      Compare CLI, API and Portal for one recorded Work (work
                     runs it automatically right after the Work).
  restore            Stop E16 pods on the fixed tags, re-import protected images,
                     restart other installs and wait until they are Ready.

Recovery: protect records its plan before changing anything and marks itself
complete only after the other installs have stopped. If any step fails or is
interrupted, the other installs stay stopped (never running E16 images); run
restore with the same --output to return them. restore is safe to repeat and
keeps its state until the other installs are Ready. Archive E16 evidence
(work, probe, parity) before restore: it stops the E16 control plane, whose
Work history is in memory.
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
  [ -n "${MODEL_PROFILE// /}" ] || fail "--model-profile is required"
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
  grep -Eq "^[[:space:]]+- name: ${MODEL_PROFILE}\$" "$platform" || fail "platform.yaml has no model profile $MODEL_PROFILE"
  grep -Eq "modelProfiles: \[([^]]*[ ,])?${MODEL_PROFILE}([ ,][^]]*)?\]" "$SCRIPT_DIR/template.yaml" ||
    fail "template.yaml does not allow model profile $MODEL_PROFILE"
}

# Objects outside the E16 namespaces that run the fixed tags, as
# "kind namespace name replicas" lines.
other_installs() {
  local deployments pools namespace name replicas pool
  # Any failed query fails the inventory; an empty answer must mean "none".
  deployments="$(kctl get deployments -A -o jsonpath='{range .items[*]}Deployment {.metadata.namespace} {.metadata.name} {.spec.replicas} {.spec.template.spec.containers[*].image}{"\n"}{end}')" || return 1
  pools="$(kctl get sandboxwarmpools -A -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name} {.spec.replicas}{"\n"}{end}')" || return 1
  printf '%s\n' "$deployments" | awk -v cp="$CONTROL_PLANE_IMAGE" -v ns="$INSTALL_NAMESPACE" '$2 != ns && index($0, cp) {print $1, $2, $3, $4}'
  while read -r namespace name replicas; do
    [ -n "$namespace" ] && [ "$namespace" != "$INSTALL_NAMESPACE" ] || continue
    pool="$(kctl -n "$namespace" get sandboxes -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].name=='$name')]}{.spec.podTemplate.spec.containers[*].image}{\"\\n\"}{end}")" || return 1
    if [ -z "$pool" ] || printf '%s' "$pool" | grep -q "$WORKER_IMAGE"; then
      printf 'SandboxWarmPool %s %s %s\n' "$namespace" "$name" "${replicas:-0}"
    fi
  done <<<"$pools"
}

node_image_id() {
  run docker exec "$(node_name)" crictl inspecti -o go-template --template '{{.status.id}}' "docker.io/library/$1" 2>/dev/null || true
}

preflight() {
  local cmd installs
  for cmd in docker kind kubectl go git; do command -v "$cmd" >/dev/null 2>&1 || fail "missing prerequisite: $cmd"; done
  run docker info >/dev/null 2>&1 || fail "Docker daemon is unreachable"
  run kind get clusters | grep -qx "$(cluster_name)" || fail "kind cluster $(cluster_name) does not exist"
  kctl get nodes >/dev/null || fail "context $CONTEXT is unreachable"
  check_platform_targets
  [ -z "$(git -C "$ROOT" status --porcelain)" ] || fail "working tree is not clean; evidence must name one source commit"
  installs="$(other_installs)" || fail "could not list installs that share the fixed tags"
  {
    echo "source $(source_sha)"
    echo "context $CONTEXT"
    echo "install-namespace $INSTALL_NAMESPACE"
    echo "model-profile $MODEL_PROFILE"
    echo "node $(node_name) created $(docker inspect -f '{{.Created}}' "$(node_name)")"
    echo "node-image $CONTROL_PLANE_IMAGE $(node_image_id "$CONTROL_PLANE_IMAGE")"
    echo "node-image $WORKER_IMAGE $(node_image_id "$WORKER_IMAGE")"
    echo "namespace $INSTALL_NAMESPACE: $(kctl get namespace "$INSTALL_NAMESPACE" --no-headers 2>&1 | head -1)"
    echo "namespace $FIXTURE_NAMESPACE: $(kctl get namespace "$FIXTURE_NAMESPACE" --no-headers 2>&1 | head -1)"
    echo "other installs sharing fixed tags:"
    printf '%s\n' "$installs" | sed 's/^/  /'
  } | tee "$OUTPUT/preflight.txt"
  record "preflight"
  pass "preflight recorded in $OUTPUT/preflight.txt"
}

# Running pods outside (others) or inside (e16) the install namespace that use
# either fixed tag, as "namespace/name phase image..." lines.
fixed_tag_pods() { # others|e16
  local pods
  pods="$(kctl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name} {.status.phase} {.spec.containers[*].image}{"\n"}{end}')" || return 1
  printf '%s\n' "$pods" |
    awk -v scope="$1" -v ns="$INSTALL_NAMESPACE" -v cp="$CONTROL_PLANE_IMAGE" -v worker="$WORKER_IMAGE" '
      NF == 0 || $2 == "Succeeded" || $2 == "Failed" {next}
      !(index($0, cp) || index($0, worker)) {next}
      { split($1, p, "/"); if ((scope == "e16") == (p[1] == ns)) print }'
}

# Succeeds only after a successful query shows no such pods; a failing query
# never counts as "stopped".
wait_no_fixed_tag_pods() { # others|e16
  local pods="" _
  for _ in $(seq 1 90); do
    if pods="$(fixed_tag_pods "$1")"; then
      [ -z "$pods" ] && return 0
    else
      pods="(pod query failed)"
    fi
    sleep 2
  done
  printf '%s\n' "$pods" >&2
  return 1
}

# Copy an old control plane's in-memory Work list through its private API.
# kubectl picks a free local port; the copy counts only if this port-forward
# reported that port and stayed alive across the request, so another local
# listener can never answer in its place.
archive_other_work() { # namespace deployment file
  local log pf port="" rc _
  log="$(mktemp)"
  kubectl --context "$CONTEXT" -n "$1" port-forward --address 127.0.0.1 "deployment/$2" :8081 >"$log" 2>&1 &
  pf=$!
  for _ in $(seq 1 50); do
    port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9][0-9]*\) -> 8081$/\1/p' "$log" | head -1)"
    [ -n "$port" ] && break
    kill -0 "$pf" 2>/dev/null || break
    sleep 0.2
  done
  if [ -z "$port" ] || ! kill -0 "$pf" 2>/dev/null; then
    cat "$log" >&2
    kill "$pf" 2>/dev/null || true
    rm -f "$log"
    return 1
  fi
  set +e
  run curl -fsS --max-time 20 "http://127.0.0.1:$port/api/requests" >"$3"
  rc=$?
  set -e
  kill -0 "$pf" 2>/dev/null || rc=1
  kill "$pf" 2>/dev/null || true
  wait "$pf" 2>/dev/null || true
  rm -f "$log"
  return "$rc"
}

# Work without an outcome is still active.
active_work_count() { # file
  node -e 'const v=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));if(!Array.isArray(v))process.exit(2);console.log(v.filter(w=>!w.outcome).length)' "$1"
}

state_complete() { grep -q '^complete ' "$1" 2>/dev/null; }

protect() {
  local state="$OUTPUT/protect-state.txt" archive kind object namespace name replicas image id got file pods
  [ ! -e "$state" ] || fail "$state exists (complete or interrupted); run restore before protecting again"
  other_installs >"$OUTPUT/protect-plan.txt" || fail "could not list installs that share the fixed tags; nothing changed"
  if [ ! -s "$OUTPUT/protect-plan.txt" ]; then
    # No known controller, but running consumers of the tags still need one.
    pods="$(fixed_tag_pods others)" || fail "could not list pods on the fixed tags; nothing changed"
    [ -z "$pods" ] || fail "pods outside $INSTALL_NAMESPACE run the fixed tags without a known controller; stop them first: $pods"
    printf 'complete %s nothing-to-protect\n' "$(stamp)" >"$state"
    pass "no other install shares the fixed tags"
    return 0
  fi
  info "plan: archive Work history, export node images for $CONTROL_PLANE_IMAGE and $WORKER_IMAGE, then stop:"
  sed 's/^/  /' "$OUTPUT/protect-plan.txt"
  info "stopping a control plane discards its in-memory Work history; the plan archives it first"
  require_yes protect
  mkdir -p "$OUTPUT/protected-images" "$OUTPUT/protected-work"
  # Refuse before any change if history cannot be archived or Work is active.
  while read -r object namespace name replicas; do
    [ "$object" = Deployment ] || continue
    file="$OUTPUT/protected-work/$namespace-$name.json"
    archive_other_work "$namespace" "$name" "$file" || fail "could not archive Work history from $namespace/$name"
    [ "$(active_work_count "$file")" = 0 ] || fail "$namespace/$name has active Work; wait for it to finish"
  done <"$OUTPUT/protect-plan.txt"
  # Record before mutating, so an interrupted protect can still be restored.
  : >"$state"
  for image in "$CONTROL_PLANE_IMAGE" "$WORKER_IMAGE"; do
    printf 'image %s %s %s\n' "$image" "$(node_image_id "$image")" "$OUTPUT/protected-images/${image//[:\/]/_}.tar" >>"$state"
  done
  while read -r object namespace name replicas; do
    [ -n "$object" ] && printf 'scale %s %s %s %s\n' "$object" "$namespace" "$name" "$replicas" >>"$state"
  done <"$OUTPUT/protect-plan.txt"
  while read -r kind image id archive; do
    [ "$kind" = image ] && [ -n "$id" ] || continue
    # The node /tmp is tmpfs, which docker cp cannot read; stream it instead.
    run docker exec "$(node_name)" ctr -n k8s.io images export /tmp/e16-protect.tar "docker.io/library/$image"
    run docker exec "$(node_name)" cat /tmp/e16-protect.tar >"$archive"
    run docker exec "$(node_name)" rm -f /tmp/e16-protect.tar
    got="$(archive_config_digest "$archive" 2>/dev/null || true)"
    [ -s "$archive" ] && [ "$got" = "$id" ] || fail "export of $image is incomplete (config $got, node $id)"
  done <"$state"
  while read -r kind object namespace name replicas; do
    [ "$kind" = scale ] || continue
    case "$object" in
      Deployment) kctl -n "$namespace" scale deployment "$name" --replicas=0 ;;
      SandboxWarmPool) kctl -n "$namespace" patch sandboxwarmpool "$name" --type merge -p '{"spec":{"replicas":0}}' ;;
    esac
  done <"$state"
  wait_no_fixed_tag_pods others || fail "other installs still run the fixed tags; protect is not complete"
  printf 'complete %s\n' "$(stamp)" >>"$state"
  record "protect complete"
  pass "other installs stopped, history archived and images exported; run restore after the campaign"
}

# Load may proceed only after a complete protect whose effect still holds.
require_protected() {
  local state="$OUTPUT/protect-state.txt" installs pods object namespace name replicas
  installs="$(other_installs)" || fail "could not list installs that share the fixed tags"
  if [ -n "$installs" ]; then
    state_complete "$state" || fail "protect is missing or incomplete; run protect (or restore after an interrupted one)"
    while read -r object namespace name replicas; do
      [ "${replicas:-0}" = 0 ] || fail "$object $namespace/$name is at $replicas replicas; protect no longer holds"
    done <<<"$installs"
  fi
  pods="$(fixed_tag_pods others)" || fail "could not list pods on the fixed tags"
  [ -z "$pods" ] || fail "pods outside $INSTALL_NAMESPACE still run the fixed tags"
}

wait_ready() { # namespace selector
  local _
  for _ in $(seq 1 90); do
    if [ -n "$(kctl -n "$1" get pods -l "$2" -o name 2>/dev/null)" ] &&
      kctl -n "$1" wait --for=condition=Ready pod -l "$2" --timeout=5s >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

# Restore stops the E16 install first (its in-memory evidence must already be
# archived), re-imports any replaced image, restarts the other installs and
# waits for them. Every step is safe to repeat after an interruption; the
# state file is kept until the other installs are Ready again.
restore() {
  local state="$OUTPUT/protect-state.txt" kind image id archive object namespace name replicas now pool
  [ -e "$state" ] || { info "no protect state; nothing to restore"; return 0; }
  info "plan: stop E16 pods on the fixed tags in $INSTALL_NAMESPACE (archive E16 evidence first),"
  info "      re-import replaced images, restore replicas and wait until the other installs are Ready:"
  sed 's/^/  /' "$state"
  require_yes restore
  kctl -n "$INSTALL_NAMESPACE" scale deployment agenova-control-plane --replicas=0 2>/dev/null || true
  for pool in $(kctl -n "$INSTALL_NAMESPACE" get sandboxwarmpools -o name 2>/dev/null); do
    kctl -n "$INSTALL_NAMESPACE" patch "$pool" --type merge -p '{"spec":{"replicas":0}}'
  done
  wait_no_fixed_tag_pods e16 || fail "E16 pods still run the fixed tags; nothing was restored"
  while read -r kind image id archive; do
    [ "$kind" = image ] && [ -n "$id" ] || continue
    now="$(node_image_id "$image")"
    [ "$now" = "$id" ] && continue
    [ -s "$archive" ] && [ "$(archive_config_digest "$archive")" = "$id" ] || fail "no valid archive to restore $image ($id)"
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
  while read -r kind object namespace name replicas; do
    [ "$kind" = scale ] && [ "${replicas:-0}" != 0 ] || continue
    case "$object" in
      Deployment) kctl -n "$namespace" rollout status "deployment/$name" --timeout=180s || fail "$namespace/$name is not Ready" ;;
      SandboxWarmPool) wait_ready "$namespace" agents.x-k8s.io/warm-pool-sandbox || fail "$namespace/$name has no Ready worker" ;;
    esac
  done <"$state"
  mv "$state" "$state.restored-$(date -u +%Y%m%dT%H%M%SZ)"
  record "restore"
  pass "other installs restored and Ready"
}

image_archive() { printf '%s/images/%s.tar' "$OUTPUT" "${1//[:\/]/_}"; }

# The config digest is what containerd on the node reports as the image ID.
archive_config_digest() {
  tar -xOf "$1" manifest.json | grep -o '"Config":"blobs/sha256/[a-f0-9]*"' | head -1 | sed 's#.*blobs/sha256/#sha256:#; s#"$##'
}

image_line() { # name tag build-tags
  local id archive platform
  id="$(run docker image inspect -f '{{.Id}}' "$2")"
  platform="$(run docker image inspect -f '{{.Os}}/{{.Architecture}}' "$2")"
  archive="$(image_archive "$2")"
  run docker save -o "$archive" "$2"
  printf '%s tag=%s platform=%s build-tags=%s local-id=%s config=%s archive-sha256=%s\n' "$1" "$2" "$platform" "$3" "$id" "$(archive_config_digest "$archive")" "$(shasum -a 256 "$archive" | cut -d' ' -f1)"
}

build_one() { # name dockerfile tag context
  local context="${4#"$ROOT"/}"
  [ "$4" != "$ROOT" ] || context=.
  printf 'command %s docker build --progress=plain -f %s -t %s %s\n' "$1" "${2#"$ROOT"/}" "$3" "$context" >>"$OUTPUT/build-commands.txt"
  run docker build --progress=plain -f "$2" -t "$3" "$4" 2>&1 | tee "$OUTPUT/build-$1.log"
}

# The probe tag must appear only in the probe binary's build settings.
extract_binary() { # image path name
  local container
  container="$(run docker create "$1")"
  run docker cp "$container:$2" "$OUTPUT/binaries/$3"
  run docker rm "$container" >/dev/null
  run go version -m "$OUTPUT/binaries/$3" >"$OUTPUT/binaries/$3.buildinfo"
  printf 'toolchain %s %s\n' "$3" "$(head -1 "$OUTPUT/binaries/$3.buildinfo" | awk '{print $2}')"
}

check_build_tags() {
  local dir="$OUTPUT/binaries"
  mkdir -p "$dir"
  extract_binary "$CONTROL_PLANE_IMAGE" /agenova-control-plane agenova-control-plane
  extract_binary "$WORKER_IMAGE" /agenova-workerctl agenova-workerctl
  extract_binary "$(fixture_image)" /mcpfixture mcpfixture
  extract_binary "$(probe_image)" /e16-probe.test e16-probe.test
  cp "$dir/agenova-control-plane.buildinfo" "$dir/control-plane.buildinfo"
  cp "$dir/e16-probe.test.buildinfo" "$dir/probe.buildinfo"
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
  : >"$OUTPUT/build-commands.txt"
  build_one control-plane "$ROOT/deploy/reference/Dockerfile" "$CONTROL_PLANE_IMAGE" "$ROOT"
  build_one worker "$ROOT/harness/integration/agentsandbox/testworker/Dockerfile" "$WORKER_IMAGE" "$ROOT"
  build_one fixture "$ROOT/harness/integration/mcpfixture/Dockerfile" "$(fixture_image)" "$ROOT/harness/integration/mcpfixture"
  build_one probe "$SCRIPT_DIR/probe.Dockerfile" "$(probe_image)" "$ROOT"
  (cd "$ROOT" && GOTOOLCHAIN=go1.22.12 run go build -o "$(cli)" ./cmd/agenova)
  {
    echo "source $(source_sha)"
    # Base images as BuildKit actually resolved them for each build.
    for name in control-plane worker fixture probe; do
      grep -ho 'FROM [^ ]*@sha256:[a-f0-9]*' "$OUTPUT/build-$name.log" | sort -u | sed "s/^/base $name /"
    done
    image_line control-plane "$CONTROL_PLANE_IMAGE" none
    image_line worker "$WORKER_IMAGE" none
    image_line fixture "$(fixture_image)" none
    image_line probe "$(probe_image)" agenovaprobe
    sed 's/^/build-/' "$OUTPUT/build-commands.txt"
    echo "cli $(GOTOOLCHAIN=go1.22.12 go version -m "$(cli)" | head -1)"
  } | tee "$OUTPUT/build-identity.txt"
  check_build_tags | tee -a "$OUTPUT/build-identity.txt"
  record "build $(source_sha)"
}

# The archive about to be loaded must be the one build recorded for this commit.
verify_recorded_archive() { # tag
  local identity="$OUTPUT/build-identity.txt" archive line want_config want_sha
  [ -s "$identity" ] || fail "no build identity; run build first"
  [ "$(sed -n 's/^source //p' "$identity")" = "$(source_sha)" ] || fail "build identity is for another commit; rebuild"
  line="$(grep " tag=$1 " "$identity" || true)"
  [ -n "$line" ] || fail "build identity has no record for $1"
  want_config="$(printf '%s' "$line" | sed -n 's/.* config=\([^ ]*\).*/\1/p')"
  want_sha="$(printf '%s' "$line" | sed -n 's/.* archive-sha256=\([^ ]*\).*/\1/p')"
  archive="$(image_archive "$1")"
  [ -s "$archive" ] || fail "no recorded archive for $1"
  [ "$(shasum -a 256 "$archive" | cut -d' ' -f1)" = "$want_sha" ] || fail "archive for $1 does not match its recorded hash"
  [ "$(archive_config_digest "$archive")" = "$want_config" ] || fail "archive for $1 does not match its recorded config"
}

load() {
  local image want got
  [ -z "$(git -C "$ROOT" status --porcelain)" ] || fail "working tree is not clean"
  require_protected
  for image in "$CONTROL_PLANE_IMAGE" "$WORKER_IMAGE" "$(fixture_image)" "$(probe_image)"; do
    verify_recorded_archive "$image"
  done
  info "plan: kind load the recorded archives for $CONTROL_PLANE_IMAGE $WORKER_IMAGE $(fixture_image) $(probe_image) into $(cluster_name)"
  require_yes load
  for image in "$CONTROL_PLANE_IMAGE" "$WORKER_IMAGE" "$(fixture_image)" "$(probe_image)"; do
    run kind load image-archive "$(image_archive "$image")" --name "$(cluster_name)"
    want="$(archive_config_digest "$(image_archive "$image")")"
    got="$(node_image_id "$image")"
    [ -n "$want" ] && [ "$want" = "$got" ] || fail "node image for $image is $got, recorded config is $want"
    printf 'node-image %s %s\n' "$image" "$got" | tee -a "$OUTPUT/load-identity.txt"
  done
  record "load"
}

pod_identity() { # namespace selector
  kctl -n "$1" get pods -l "$2" -o jsonpath='{range .items[*]}{.metadata.name} uid={.metadata.uid} restarts={.status.containerStatuses[0].restartCount} container={.status.containerStatuses[0].containerID} imageID={.status.containerStatuses[0].imageID}{"\n"}{end}'
}

# --- identity chain ---

recorded_config() { # tag
  local line
  line="$(grep " tag=$1 " "$OUTPUT/build-identity.txt" 2>/dev/null || true)"
  printf '%s' "$line" | sed -n 's/.* config=\([^ ]*\).*/\1/p'
}

# Resolve an image reference the node knows (image ID, tag or repo digest) to
# its containerd image ID, which is the config digest. kind-loaded images
# appear in Pod status as docker.io/library/import-<date>@sha256:..., which
# only the image list maps back; anything but exactly one match resolves to
# nothing.
image_ref_id() {
  run docker exec "$(node_name)" crictl images -o json 2>/dev/null |
    node -e 'const ref=process.argv[1];let v;try{v=JSON.parse(require("fs").readFileSync(0,"utf8"))}catch{process.exit(0)}
      const hits=(v.images||[]).filter(i=>i.id===ref||(i.repoDigests||[]).includes(ref)||(i.repoTags||[]).includes(ref));
      if(hits.length===1)process.stdout.write(hits[0].id)' "$1" || true
}

# The tag on the node must still be the image build recorded.
verify_node_tag() { # tag
  local want got
  want="$(recorded_config "$1")"
  got="$(node_image_id "$1")"
  [ -n "$want" ] && [ "$want" = "$got" ] || fail "node tag $1 is $got, recorded config is ${want:-missing}; the image changed after load"
}

# Every running Pod's image must resolve to the recorded config.
verify_identity_lines() { # tag file
  local want line image got n=0
  want="$(recorded_config "$1")"
  [ -n "$want" ] || fail "no recorded config for $1"
  while read -r line; do
    [ -n "$line" ] || continue
    image="$(printf '%s' "$line" | sed -n 's/.* imageID=\([^ ]*\).*/\1/p')"
    got="$(image_ref_id "$image")"
    [ "$got" = "$want" ] || fail "Pod image $image resolves to ${got:-nothing}, recorded config for $1 is $want"
    n=$((n + 1))
  done <"$2"
  [ "$n" -gt 0 ] || fail "no Pod identity captured for $1"
}

# --- complete log capture ---

start_collector() { # name namespace deployment
  local log="$OUTPUT/$1-follow.jsonl"
  nohup kubectl --context "$CONTEXT" -n "$2" logs -f "deployment/$3" >"$log" 2>"$OUTPUT/$1-follow.err" &
  echo $! >"$OUTPUT/$1-follow.pid"
  sleep 2
  kill -0 "$(cat "$OUTPUT/$1-follow.pid")" 2>/dev/null && [ ! -s "$OUTPUT/$1-follow.err" ] ||
    fail "log collector $1 did not start: $(cat "$OUTPUT/$1-follow.err" 2>/dev/null)"
}

# A collector that exited may have missed lines; the campaign window it
# covered cannot be trusted.
require_collector() { # name
  [ -s "$OUTPUT/$1-follow.pid" ] && kill -0 "$(cat "$OUTPUT/$1-follow.pid")" 2>/dev/null ||
    fail "log collector $1 is not running; start a new campaign"
}

# Every line the collector saw must be in the complete snapshot, so a
# truncated snapshot fails. Both files must exist; grep exit 1 means nothing
# is missing, 0 means lines are missing, anything else is an error.
check_log_continuity() { # follow full
  local rc
  [ -r "$1" ] && [ -r "$2" ] || fail "log artifact missing or unreadable: $1 or $2"
  set +e
  grep -Fxv -f "$2" "$1" >"$1.missing"
  rc=$?
  set -e
  case "$rc" in
    1) rm -f "$1.missing" ;;
    0) fail "$(grep -c . "$1.missing") captured log lines are missing from the snapshot $2" ;;
    *) fail "could not compare $1 with $2" ;;
  esac
}

# The fixture Pod must be the one recorded at start, never restarted.
check_fixture_identity() { # file
  pod_identity "$FIXTURE_NAMESPACE" app.kubernetes.io/name=e16-mcp >"$1"
  cmp -s "$OUTPUT/fixture-identity-start.txt" "$1" || fail "fixture Pod identity or restart count changed since it started"
}

# Copy only complete records: a live collector may be halfway through a line.
freeze_complete_lines() { # source destination
  node -e 'const fs=require("fs");const b=fs.readFileSync(process.argv[1]);const end=b.lastIndexOf(10)+1;fs.writeFileSync(process.argv[2],b.subarray(0,end))' "$1" "$2" ||
    fail "could not freeze $1"
}

snapshot_fixture() { # out-dir
  require_collector fixture
  check_fixture_identity "$1/fixture-identity.txt"
  # Freeze what the collector has seen before taking the snapshot; lines it
  # receives later are not expected in this snapshot.
  [ -r "$OUTPUT/fixture-follow.jsonl" ] || fail "fixture log collector output is missing"
  freeze_complete_lines "$OUTPUT/fixture-follow.jsonl" "$1/fixture-follow-before-snapshot.jsonl"
  kctl -n "$FIXTURE_NAMESPACE" logs deployment/e16-mcp >"$1/fixture-full.jsonl"
  check_log_continuity "$1/fixture-follow-before-snapshot.jsonl" "$1/fixture-full.jsonl"
  kctl -n "$FIXTURE_NAMESPACE" get events -o wide >"$1/fixture-events.txt" 2>&1 || true
}

# Local rendering only; it needs no cluster.
render_fixture() { # output
  kubectl kustomize "$ROOT/harness/integration/mcpfixture" | sed "s#image: ${FIXTURE_DEFAULT_IMAGE}\$#image: $(fixture_image)#" >"$1"
  [ "$(grep -c "image: $(fixture_image)\$" "$1")" -eq 1 ] && ! grep -q "image: ${FIXTURE_DEFAULT_IMAGE}" "$1" ||
    fail "fixture image override did not apply exactly once"
}

fixture() {
  local rendered="$OUTPUT/fixture-rendered.yaml"
  info "plan: apply the MCP fixture to $FIXTURE_NAMESPACE with $(fixture_image) and capture its log"
  verify_node_tag "$(fixture_image)"
  render_fixture "$rendered"
  kctl apply -f "$rendered"
  kctl -n "$FIXTURE_NAMESPACE" rollout status deployment/e16-mcp --timeout=120s
  pod_identity "$FIXTURE_NAMESPACE" app.kubernetes.io/name=e16-mcp >"$OUTPUT/fixture-identity-start.txt"
  [ "$(grep -c . "$OUTPUT/fixture-identity-start.txt")" = 1 ] || fail "the fixture must run exactly one Pod"
  verify_identity_lines "$(fixture_image)" "$OUTPUT/fixture-identity-start.txt"
  start_collector fixture "$FIXTURE_NAMESPACE" e16-mcp
  kctl -n "$FIXTURE_NAMESPACE" get events -o wide >"$OUTPUT/fixture-events-start.txt" 2>&1 || true
  record "fixture $(fixture_image)"
  pass "fixture running on the recorded image; log capture pid $(cat "$OUTPUT/fixture-follow.pid")"
}

install() {
  local id existing
  check_platform_targets
  verify_node_tag "$CONTROL_PLANE_IMAGE"
  verify_node_tag "$WORKER_IMAGE"
  # Never apply while the E16 control plane is running Work. A failed query is
  # not "absent"; only a successful empty answer is.
  existing="$(kctl -n "$INSTALL_NAMESPACE" get deployment agenova-control-plane -o name --ignore-not-found)" ||
    fail "could not query the E16 control plane; not applying"
  if [ -n "$existing" ]; then
    archive_other_work "$INSTALL_NAMESPACE" agenova-control-plane "$OUTPUT/e16-work-before-install.json" ||
      fail "could not read the E16 Work list; not applying"
    [ "$(active_work_count "$OUTPUT/e16-work-before-install.json")" = 0 ] || fail "E16 Work is active; not applying"
  fi
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
  # Identical reapply: a second apply of the same revision, recorded for idempotence.
  run "$(cli)" --state-dir "$STATE_DIR" platform apply -f "$SCRIPT_DIR/platform.yaml" --yes | tee "$OUTPUT/reapply.txt"
  run "$(cli)" --state-dir "$STATE_DIR" platform status | tee "$OUTPUT/status-after-reapply.txt"
  kctl -n "$INSTALL_NAMESPACE" rollout status deployment/agenova-control-plane --timeout=180s
  pod_identity "$INSTALL_NAMESPACE" app.kubernetes.io/name=agenova-control-plane >"$OUTPUT/control-plane-identity.txt"
  verify_identity_lines "$CONTROL_PLANE_IMAGE" "$OUTPUT/control-plane-identity.txt"
  start_collector control-plane "$INSTALL_NAMESPACE" agenova-control-plane
  kctl -n "$INSTALL_NAMESPACE" get events -o wide >"$OUTPUT/install-events.txt" 2>&1 || true
  record "install"
}

# The control plane serving Work must still be the Pod install verified.
check_control_plane_identity() { # file
  pod_identity "$INSTALL_NAMESPACE" app.kubernetes.io/name=agenova-control-plane >"$1"
  cmp -s "$OUTPUT/control-plane-identity.txt" "$1" || fail "the E16 control plane Pod, image or restart count changed since install"
  verify_identity_lines "$CONTROL_PLANE_IMAGE" "$1"
}

# Tool invocation IDs in a Work's evidence, one per line.
work_invocations() { # work-show.json [reason]
  node -e 'const v=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));const want=process.argv[2]||"";
    const ids=new Set((v.facts||[]).filter(f=>f.operation==="tool.invoke"&&f.invocationId&&(!want||f.reasonCode===want)).map(f=>f.invocationId));
    for(const id of ids)console.log(id)' "$@"
}

# "<invocation ID> <end> <state>" for each tool invocation of a Work: end is
# its latest fact, state is "denied" (never attempted), "no-outcome" or the
# outcome reason, so the checkers can tell its history from anything new.
work_prior_records() { # work-show.json
  # Compare instants, not strings: "01:00:00Z" sorts after "01:00:00.002Z".
  node -e 'const v=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));
    const ns=t=>{const ms=Date.parse(t);if(Number.isNaN(ms))throw new Error("bad timestamp "+t);
      const frac=((t.match(/\.(\d+)/)||[,""])[1]+"000000000").slice(3,9);return BigInt(ms)*1000000n+BigInt(frac)};
    const calls=new Map();
    for(const f of v.facts||[]){if(f.operation!=="tool.invoke"||!f.invocationId)continue;
      const c=calls.get(f.invocationId)||{end:f.timestamp,attempted:false,reason:""};
      if(ns(f.timestamp)>ns(c.end))c.end=f.timestamp;
      if(f.kind==="ProviderAttempt")c.attempted=true;
      if(f.kind==="ProviderOutcome")c.reason=f.reasonCode||"outcome";
      calls.set(f.invocationId,c)}
    for(const [id,c] of calls)console.log(id,c.end,!c.attempted?"denied":(c.reason||"no-outcome"))' "$1"
}

# A timed-out call keeps running on the server; wait until the fixture logs
# its handler finishing so it cannot overlap the next Work.
settle_timeouts() { # work-show.json
  local id _
  for id in $(work_invocations "$1" tool-timeout); do
    for _ in $(seq 1 60); do
      kctl -n "$FIXTURE_NAMESPACE" logs deployment/e16-mcp 2>/dev/null | grep -F "\"correlation\":\"$id\"" | grep -q '"event":"tool"' && continue 2
      sleep 1
    done
    fail "the timed-out call $id did not finish on the server within 60s"
  done
}

# The worker that the claim names must have been seen running the recorded
# worker image while the Work ran.
verify_claimed_worker() { # work-show.json worker-identity.txt
  local worker
  worker="$(node -e 'const v=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));process.stdout.write(v.state?.claim?.backendIdentity?.workerId||"")' "$1")"
  [ -n "$worker" ] || fail "the Work evidence names no worker"
  grep "^$worker " "$2" >"$2.claimed" || fail "worker $worker was never captured while the Work ran"
  verify_identity_lines "$WORKER_IMAGE" "$2.claimed"
}

work() {
  local name="${1:-}" file ref out watcher
  [ -n "$name" ] || fail "work needs a name, for example: work positive"
  file="$SCRIPT_DIR/work-$name.yaml"
  [ -f "$file" ] || fail "no $file"
  ref="e16-$name"
  out="$OUTPUT/work/$name"
  [ ! -e "$out" ] || fail "$out exists; each Work is recorded once per campaign"
  mkdir -p "$out"
  require_collector fixture
  require_collector control-plane
  verify_node_tag "$CONTROL_PLANE_IMAGE"
  verify_node_tag "$WORKER_IMAGE"
  check_control_plane_identity "$out/control-plane-before.txt"
  # Capture worker identity while the Work runs, before cleanup.
  (for _ in $(seq 1 600); do
    pod_identity "$INSTALL_NAMESPACE" agents.x-k8s.io/sandbox-template-ref-hash >>"$out/worker-identity.txt" 2>/dev/null || true
    sleep 1
  done) &
  watcher=$!
  set +e
  run "$(cli)" --state-dir "$STATE_DIR" run -f "$file" >"$out/run.txt" 2>&1
  echo "run-exit $?" >"$out/exit-codes.txt"
  run "$(cli)" --state-dir "$STATE_DIR" work show "$ref" --json >"$out/work-show.json" 2>"$out/work-show.err"
  echo "show-exit $?" >>"$out/exit-codes.txt"
  set -e
  kill "$watcher" 2>/dev/null || true
  sort -u "$out/worker-identity.txt" -o "$out/worker-identity.txt" 2>/dev/null || true
  grep -q '^show-exit 0$' "$out/exit-codes.txt" || fail "work show failed for $ref; see $out"
  check_control_plane_identity "$out/control-plane-after.txt"
  [ "$name" = admission-deny ] || verify_claimed_worker "$out/work-show.json" "$out/worker-identity.txt"
  settle_timeouts "$out/work-show.json"
  snapshot_fixture "$out"
  require_collector control-plane
  kctl -n "$INSTALL_NAMESPACE" get events -o wide >"$out/install-events.txt" 2>&1 || true
  (cd "$ROOT" && GOTOOLCHAIN=go1.22.12 run go run ./harness/integration/e16/evidence work -case "$name" \
    -view "$out/work-show.json" -server-log "$out/fixture-full.jsonl" -fixture-data "$ROOT/harness/integration/mcpfixture/data" \
    -prior "$OUTPUT/work-invocations.txt") \
    >"$out/evidence.txt" 2>&1 || fail "Work $name does not meet its acceptance; see $out/evidence.txt"
  work_prior_records "$out/work-show.json" >>"$OUTPUT/work-invocations.txt"
  # Per-run parity, while this Work's in-memory evidence still exists.
  parity "$name"
  record "work $name pass"
  pass "Work $name accepted and its CLI, API and Portal evidence archived"
}

# A Job that exits 0 can still have skipped or emitted nothing.
validate_probe_output() {
  local file="$1"
  grep -q -- '--- PASS: TestE16Probes ' "$file" || fail "probe test did not pass (skipped, failed or absent)"
  ! grep -q -- '--- SKIP' "$file" || fail "probe output contains a skipped test"
  grep -q '^E16_PROBE ' "$file" || fail "probe emitted no receipts"
}

probe() {
  local out="$OUTPUT/probe" rendered
  [ ! -e "$out" ] || fail "$out exists; start a new campaign directory for another probe run"
  mkdir -p "$out"
  info "plan: run Job $PROBE_JOB in $INSTALL_NAMESPACE with $(probe_image) and check it against the fixture log"
  require_collector fixture
  verify_node_tag "$(probe_image)"
  check_fixture_identity "$out/fixture-identity-before.txt"
  kctl -n "$INSTALL_NAMESPACE" create configmap e16-probe-inputs --from-file=template.yaml="$SCRIPT_DIR/template.yaml" --from-file=policy.yaml="$SCRIPT_DIR/policy.yaml" --dry-run=client -o yaml | kctl apply -f -
  kctl -n "$INSTALL_NAMESPACE" delete job "$PROBE_JOB" --ignore-not-found --wait=true
  rendered="$out/probe-job.yaml"
  sed "s#image: PROBE_IMAGE#image: $(probe_image)#" "$SCRIPT_DIR/probe-job.yaml" >"$rendered"
  kctl -n "$INSTALL_NAMESPACE" apply -f "$rendered"
  kctl -n "$INSTALL_NAMESPACE" wait --for=condition=complete --timeout=900s "job/$PROBE_JOB" || true
  kctl -n "$INSTALL_NAMESPACE" logs "job/$PROBE_JOB" >"$out/probe-job.log"
  pod_identity "$INSTALL_NAMESPACE" "job-name=$PROBE_JOB" >"$out/probe-identity.txt"
  verify_identity_lines "$(probe_image)" "$out/probe-identity.txt"
  snapshot_fixture "$out"
  validate_probe_output "$out/probe-job.log"
  sed -n 's/^E16_PROBE //p' "$out/probe-job.log" >"$out/probes.jsonl"
  (cd "$ROOT" && GOTOOLCHAIN=go1.22.12 run go run ./harness/integration/e16/evidence -receipts "$out/probes.jsonl" -server-log "$out/fixture-full.jsonl" -prior "$OUTPUT/work-invocations.txt") >"$out/evidence.jsonl" ||
    fail "probe receipts do not match the fixture server log; see $out/evidence.jsonl"
  record "probe pass"
  pass "probe receipts match the fixture server log"
}

# Exactly two expected passes, nothing failed, skipped or flaky.
parity_report_ok() { # playwright.json
  node -e 'let s;try{s=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8")).stats}catch{process.exit(1)}
    if(!s||s.expected!==2||s.unexpected||s.skipped||s.flaky)process.exit(1)' "$1"
}

# parity <case>: compare CLI, API and Portal for one recorded Work. Exactly the
# setup test and that Work's test must run and pass; nothing may be skipped.
parity() {
  local name="${1:-}" out api ui rc report
  case "$name" in positive|n6-timeout|n7-oversize|n8-truncation|admission-deny) ;; *) fail "parity needs a Work case" ;; esac
  out="$OUTPUT/work/$name/parity"
  [ ! -e "$out" ] || fail "$out exists; parity is archived once per Work"
  mkdir -p "$out"
  info "plan: agenova api connect, UI dev server on 5177, run the @setup and @$name tests"
  "$(cli)" --state-dir "$STATE_DIR" api connect >"$out/api-connect.log" 2>&1 &
  api=$!
  npm --prefix "$ROOT/ui" run dev -- --port 5177 --strictPort >"$out/ui-dev.log" 2>&1 &
  ui=$!
  sleep 5
  report="$out/playwright.json"
  set +e
  (cd "$ROOT/ui" && AGENOVA_CLI_PATH="$(cli)" AGENOVA_CLI_STATE_DIR="$STATE_DIR" AGENOVA_E16_CASE="$name" AGENOVA_E16_REF="e16-$name" \
    PLAYWRIGHT_JSON_OUTPUT_NAME="$report" \
    run npx playwright test --config playwright.installed.config.ts installed/mcp.spec.ts --grep "@setup\$|@$name\$" \
      --reporter=list,json --output "$out/playwright") >"$out/playwright.log" 2>&1
  rc=$?
  set -e
  kill "$api" "$ui" 2>/dev/null || true
  record "parity $name exit $rc"
  [ "$rc" -eq 0 ] || fail "CLI, API and Portal parity failed for $name; see $out/playwright.log"
  parity_report_ok "$report" || fail "parity for $name did not run exactly its two tests; see $report"
  pass "CLI, API and Portal agree for $name"
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
      --model-profile) MODEL_PROFILE="${2:-}"; shift 2 ;;
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
    parity) parity "$@" ;;
    restore) restore ;;
    status) status ;;
    *) usage; fail "unknown subcommand: $subcommand" ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
