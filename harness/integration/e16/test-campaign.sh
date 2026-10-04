#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# Offline checks for campaign.sh: argument guards, protect/load/restore gates,
# identity and install records, the controlled read, parity cleanup and probe
# output validation. No command reaches Docker, kind or a cluster; the parity
# checks start local stand-in listeners on free loopback ports.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT="$ROOT/harness/integration/e16/campaign.sh"
# shellcheck source=campaign.sh
source "$SCRIPT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
LOG="$TMP/calls.log"
HEX="$(printf 'a%.0s' $(seq 1 64))"
RECORDED="sha256:$HEX"

hex() { printf "$1%.0s" $(seq 1 64); }
# Fake OCI image archives whose manifest.json names the recorded config. The
# index references a platform manifest and an attestation manifest, as
# docker save and ctr export write them; make_archive can leave one blob out.
LAYER="$(hex 1)" MAN="$(hex 2)" ATT="$(hex 5)" IDX="$(hex 6)"
make_archive() { # name [hex of a blob to leave out]
  local d="$TMP/$1" b="$TMP/$1/blobs/sha256" attcfg attlayer
  attcfg="$(hex 3)" attlayer="$(hex 4)"
  rm -rf "$d"; mkdir -p "$b"
  printf '{}' >"$b/$HEX"; printf 'layer' >"$b/$LAYER"; printf '{}' >"$b/$attcfg"; printf '{}' >"$b/$attlayer"
  printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:%s"},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:%s"}]}' "$HEX" "$LAYER" >"$b/$MAN"
  printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:%s"},"layers":[{"mediaType":"application/vnd.in-toto+json","digest":"sha256:%s"}]}' "$attcfg" "$attlayer" >"$b/$ATT"
  printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:%s","platform":{"architecture":"arm64","os":"linux"}},{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:%s","platform":{"architecture":"unknown","os":"unknown"},"annotations":{"vnd.docker.reference.type":"attestation-manifest"}}]}' "$MAN" "$ATT" >"$b/$IDX"
  printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.index.v1+json","digest":"sha256:%s"}]}' "$IDX" >"$d/index.json"
  printf '[{"Config":"blobs/sha256/%s"}]' "$HEX" >"$d/manifest.json"
  [ -z "${2:-}" ] || rm -f "$b/$2"
  tar -cf "$TMP/$1.tar" -C "$d" index.json manifest.json blobs
}
# variant <name> <js>: the complete archive with one structural defect. The
# script runs in the layout directory with r(hex)/w(hex, value) for blobs.
variant() {
  rm -rf "$TMP/$1"; cp -R "$TMP/fake" "$TMP/$1"
  (cd "$TMP/$1" && IDX="$IDX" MAN="$MAN" HEX="$HEX" node -e 'const fs=require("fs");const {IDX,MAN,HEX}=process.env;
    const p=d=>"blobs/sha256/"+d;const r=d=>JSON.parse(fs.readFileSync(p(d)));
    const w=(d,v)=>fs.writeFileSync(p(d),typeof v==="string"?v:JSON.stringify(v));'"$2")
  tar -cf "$TMP/$1.tar" -C "$TMP/$1" index.json manifest.json blobs
}
make_archive fake
# ctr images export without --all-platforms leaves the attestation manifest out.
make_archive partial "$ATT"
make_archive nolayer "$LAYER"
mkdir -p "$TMP/legacy"
printf '[{"Config":"blobs/sha256/%s"}]' "$HEX" >"$TMP/legacy/manifest.json"
tar -cf "$TMP/legacy.tar" -C "$TMP/legacy" manifest.json
# A descriptor without a media type hides its missing config.
variant notype 'const v=r(IDX);delete v.manifests[0].mediaType;w(IDX,v);fs.unlinkSync(p(HEX))'
variant emptychild 'const v=r(IDX);v.manifests=[];w(IDX,v)'
variant unknowntype 'const v=r(IDX);v.manifests[0].mediaType="application/vnd.example+json";w(IDX,v)'
variant baddigest 'const v=JSON.parse(fs.readFileSync("index.json"));v.manifests[0].digest="sha256:../x";fs.writeFileSync("index.json",JSON.stringify(v))'
variant attonly 'const v=r(IDX);v.manifests=v.manifests.slice(1);w(IDX,v)'
variant garbage 'w(MAN,"not json")'
variant nolayers 'const v=r(MAN);delete v.layers;w(MAN,v)'
variant wrongtype 'const v=r(MAN);v.mediaType="application/vnd.oci.image.index.v1+json";w(MAN,v)'
# The same blob named first as a manifest and then as an index.
variant retyped 'const v=r(IDX);v.manifests.push({mediaType:"application/vnd.oci.image.index.v1+json",digest:"sha256:"+MAN});w(IDX,v)'
# One manifest named first as an attestation and then as the image: complete.
variant attfirst 'const v=r(IDX);const m=v.manifests[0];v.manifests=[{...m,annotations:{"vnd.docker.reference.type":"attestation-manifest"}},m];w(IDX,v)'

# Stubs: every external command is logged, nothing runs.
FAIL_ON=""
run() {
  printf '%s\n' "$*" >>"$LOG"
  case "$*" in
    *"cat /tmp/e16-protect.tar"*) cat "${EXPORT_TAR:-$TMP/fake.tar}" ;;
    *"kind load image-archive"*) touch "$TMP/imported" ;;
    # Any local listener answers with valid, empty Work JSON.
    *"curl "*) printf '[]' ;;
    *"logs deployment/e16-mcp"*) printf '%s' "${FIXTURE_LOG:-}" ;;
    *"get deployment agenova-control-plane -o name"*) [ -z "${E16_DEPLOYED:-}" ] || echo deployment.apps/agenova-control-plane ;;
    *"crictl images -o json"*) printf '%s' "${NODE_IMAGES:-}" ;;
    *"get pods -l app.kubernetes.io/name=agenova-control-plane"*) [ -z "${CP_POD:-}" ] || printf '%s\n' "$CP_POD" ;;
    *"platform validate"*) echo "validate output on stderr" >&2 ;;
    *"platform plan"*) echo "plan: 9 changes" ;;
    *"playwright test"*) [ -z "${PLAYWRIGHT_BLOCK:-}" ] || command sleep "$PLAYWRIGHT_BLOCK"
      [ -z "${PLAYWRIGHT_JSON_OUTPUT_NAME:-}" ] || printf '{"stats":{"expected":2,"unexpected":0,"skipped":0,"flaky":0}}' >"$PLAYWRIGHT_JSON_OUTPUT_NAME" ;;
  esac
  if [ -n "$FAIL_ON" ] && [[ "$*" == *"$FAIL_ON"* ]]; then return 1; fi
  return 0
}
git() { case "$*" in *"status --porcelain"*) : ;; *"rev-parse --short=12 HEAD"*) echo testsha12345 ;; *"rev-parse HEAD"*) echo testsha ;; esac; }
sleep() { :; }
OTHER_REPLICAS=1 OTHER_NONE="" OTHER_FAIL="" PODS_FAIL=""
eval "real_$(declare -f other_installs)"
eval "real_$(declare -f fixed_tag_pods)"
other_installs() {
  [ -z "$OTHER_FAIL" ] || return 1
  [ -n "$OTHER_NONE" ] || printf 'Deployment agenova-system agenova-control-plane %s\n' "$OTHER_REPLICAS"
}
OTHER_PODS="" E16_PODS=""
fixed_tag_pods() {
  [ -z "$PODS_FAIL" ] || return 1
  if [ "$1" = others ]; then printf '%s' "$OTHER_PODS"; else printf '%s' "$E16_PODS"; fi
}
WORK_JSON='[]' ARCHIVE_RC=0
eval "real_$(declare -f archive_other_work)"
archive_other_work() { printf '%s' "$WORK_JSON" >"$3"; return "$ARCHIVE_RC"; }
NODE_ID="$RECORDED"
# After an import the node reports the recorded image again.
node_image_id() { if [ -e "$TMP/imported" ]; then printf '%s' "$RECORDED"; else printf '%s' "$NODE_ID"; fi; }
wait_ready() { return 0; }

reset() {
  OUTPUT="$TMP/out-$1"; rm -rf "$OUTPUT" "$TMP/imported"; mkdir -p "$OUTPUT"; : >"$LOG"
  CONTEXT=kind-x INSTALL_NAMESPACE=agenova-e16-system STATE_DIR="$TMP/s" MODEL_PROFILE=coding-standard YES=1
  OTHER_REPLICAS=1 OTHER_NONE="" OTHER_FAIL="" PODS_FAIL="" OTHER_PODS="" E16_PODS="" WORK_JSON='[]' ARCHIVE_RC=0 NODE_ID="$RECORDED" FAIL_ON="" EXPORT_TAR=""
  NODE_IMAGES="" CP_POD="" PLAYWRIGHT_BLOCK=""
}
# expect_fail <description> <expected message> <command...>: the command must
# fail for the stated reason, not an earlier one.
expect_fail() {
  local d="$1" want="$2" out; shift 2
  if out="$( ( "$@" ) 2>&1 )"; then echo "[fail] accepted: $d"; exit 1; fi
  [[ "$out" == *"$want"* ]] || { echo "[fail] $d failed for another reason: $out"; exit 1; }
}
called() { grep -q -- "$1" "$LOG"; }
not_called() { ! grep -q -- "$1" "$LOG" || { echo "[fail] unexpected call: $1"; exit 1; }; }
line_of() { grep -n -- "$1" "$LOG" | head -1 | cut -d: -f1; }

# Each bad invocation must stop at its own argument check, before any
# external command or directory creation.
while IFS='|' read -r want args; do
  : >"$LOG"
  rm -rf "$TMP/o" "$TMP/s"
  # shellcheck disable=SC2086
  if out="$( (eval "main $args") 2>&1 )"; then echo "[fail] accepted: $args"; exit 1; fi
  [[ "$out" == *"$want"* ]] || { echo "[fail] '$args' failed for another reason: $out"; exit 1; }
  [ ! -s "$LOG" ] && [ ! -e "$TMP/o" ] && [ ! -e "$TMP/s" ] || { echo "[fail] '$args' ran commands or created directories"; exit 1; }
done <<EOF
--context is required|preflight
--output is required|--context kind-x --install-namespace e16 --state-dir $TMP/s --model-profile coding-standard preflight
--model-profile is required|--context kind-x --install-namespace e16 --state-dir $TMP/s --output $TMP/o preflight
must name a kind context|--context minikube --install-namespace e16 --state-dir $TMP/s --output $TMP/o --model-profile coding-standard preflight
is reserved|--context kind-x --install-namespace default --state-dir $TMP/s --output $TMP/o --model-profile coding-standard preflight
is reserved|--context kind-x --install-namespace agenova-e16 --state-dir $TMP/s --output $TMP/o --model-profile coding-standard preflight
--context is required|--context '  ' --install-namespace e16 --state-dir $TMP/s --output $TMP/o --model-profile coding-standard preflight
unknown flag|--context kind-x --frobnicate preflight
EOF
echo '[pass] context, namespace, state and output arguments are required and checked'

# --- archive completeness ---
for good in fake attfirst; do
  archive_complete "$TMP/$good.tar" 2>/dev/null || { echo "[fail] a complete archive was rejected: $good"; exit 1; }
done
while IFS='|' read -r bad want; do
  expect_fail "archive $bad" "$want" archive_complete "$TMP/$bad.tar"
done <<'EOF'
partial|referenced but absent
nolayer|referenced but absent
legacy|unreadable layout index
notype|malformed descriptor
emptychild|names no manifest
unknowntype|is neither an index nor a manifest
baddigest|malformed descriptor
attonly|no runnable image manifest
garbage|unreadable blob
nolayers|has no layer list
wrongtype|is not the
retyped|is referenced as both
EOF
[ "$(archive_config_digest "$TMP/partial.tar")" = "$RECORDED" ] || { echo "[fail] partial archive must still name the recorded config"; exit 1; }
echo '[pass] an archive counts as complete only when every blob its index reaches is in it and its structure is followable'

# --- protect ---
reset p1; YES=0
expect_fail "protect without --yes" "rerun with --yes" protect
not_called 'ctr -n k8s.io'; not_called 'scale'; [ ! -e "$OUTPUT/protect-state.txt" ]
echo '[pass] protect changes nothing without --yes'

reset p2; WORK_JSON='[{"requestRef":"running-work"}]'
expect_fail "protect with active Work" "has active Work" protect
not_called 'ctr -n k8s.io'; not_called 'scale'; [ ! -e "$OUTPUT/protect-state.txt" ]
reset p3; ARCHIVE_RC=1
expect_fail "protect without a history archive" "could not archive Work history" protect
not_called 'ctr -n k8s.io'; [ ! -e "$OUTPUT/protect-state.txt" ]
echo '[pass] protect refuses before any change when history cannot be archived or Work is active'

reset p4; NODE_ID="sha256:$(printf 'b%.0s' $(seq 1 64))"
expect_fail "protect with an export that does not match the node image" "is incomplete" protect
not_called 'scale'; ! state_complete "$OUTPUT/protect-state.txt"
echo '[pass] protect stops before scaling when an export does not match the node image'

# The config matches the node, but the archive lacks the attestation manifest
# its index references, so kind load --all-platforms could not restore it.
reset p7; EXPORT_TAR="$TMP/partial.tar"
expect_fail "protect with an export missing a referenced manifest" "lacks content its index references" protect
not_called 'scale'; ! state_complete "$OUTPUT/protect-state.txt"
echo '[pass] protect stops before scaling when an export could not be imported again'

reset p5; OTHER_PODS='agenova-system/old Running agenova-control-plane:0.1.0'
expect_fail "protect while old pods keep running" "protect is not complete" protect
called 'scale deployment agenova-control-plane --replicas=0'; ! state_complete "$OUTPUT/protect-state.txt"
echo '[pass] protect is not complete until the other installs have stopped'

reset p6
protect >/dev/null
state_complete "$OUTPUT/protect-state.txt"
grep -q "^image agenova-control-plane:0.1.0 $RECORDED " "$OUTPUT/protect-state.txt"
grep -q '^scale Deployment agenova-system agenova-control-plane 1$' "$OUTPUT/protect-state.txt"
[ -s "$OUTPUT/protected-work/agenova-system-agenova-control-plane.json" ]
[ "$(line_of 'ctr -n k8s.io images export')" -lt "$(line_of 'scale deployment')" ]
[ "$(grep -c 'ctr -n k8s.io images export' "$LOG")" -eq "$(grep -c 'ctr -n k8s.io images export --all-platforms ' "$LOG")" ] ||
  { echo "[fail] protect must export every platform"; exit 1; }
expect_fail "protect twice" "run restore before protecting again" protect
echo '[pass] protect archives history, verifies exports, stops the old install and refuses to repeat'

# --- inventory queries fail closed ---
reset q1; FAIL_ON='get deployments'
expect_fail "real inventory with a failing deployment query" "" real_other_installs
reset q2; FAIL_ON='get sandboxwarmpools'
expect_fail "real inventory with a failing warm pool query" "" real_other_installs
reset q3; FAIL_ON='get pods'
expect_fail "real pod query failure" "" real_fixed_tag_pods others
reset q4; OTHER_FAIL=1
expect_fail "protect when the inventory query fails" "could not list installs" protect
not_called 'ctr -n k8s.io'; not_called 'scale'; [ ! -e "$OUTPUT/protect-state.txt" ]
reset q5; OTHER_NONE=1; OTHER_PODS='agenova-system/orphan Running agenova-control-plane:0.1.0'
expect_fail "protect with fixed-tag pods but no known controller" "without a known controller" protect
[ ! -e "$OUTPUT/protect-state.txt" ]
reset q6; OTHER_NONE=1; PODS_FAIL=1
expect_fail "protect when the pod query fails and no controller is known" "could not list pods" protect
[ ! -e "$OUTPUT/protect-state.txt" ]
reset q7; PODS_FAIL=1
expect_fail "protect when pods can never be confirmed stopped" "protect is not complete" protect
! state_complete "$OUTPUT/protect-state.txt"
echo '[pass] failed Kubernetes queries never read as "nothing running"'

# --- Work history archive through port-forward ---
PF_MODE=fail
kubectl() {
  case "$PF_MODE" in
    fail) echo 'error: unable to listen on any of the requested ports' >&2; return 1 ;;
    die) echo 'Forwarding from 127.0.0.1:43123 -> 8081'; return 0 ;;
    ok) echo 'Forwarding from 127.0.0.1:43123 -> 8081'; command sleep 5 ;;
  esac
}
reset a1; PF_MODE=fail
expect_fail "archive when port-forward fails while another listener answers" "" real_archive_other_work agenova-system agenova-control-plane "$OUTPUT/w.json"
not_called 'curl'
reset a2; PF_MODE=die
expect_fail "archive when port-forward exits after reporting a port" "" real_archive_other_work agenova-system agenova-control-plane "$OUTPUT/w.json"
reset a3; PF_MODE=ok
real_archive_other_work agenova-system agenova-control-plane "$OUTPUT/w.json"
called 'http://127.0.0.1:43123/api/requests'
reset a4; PF_MODE=fail
archive_other_work() { real_archive_other_work "$@"; }
expect_fail "protect when the history port-forward fails" "could not archive Work history" protect
not_called 'ctr -n k8s.io'; not_called 'scale'; [ ! -e "$OUTPUT/protect-state.txt" ]
archive_other_work() { printf '%s' "$WORK_JSON" >"$3"; return "$ARCHIVE_RC"; }
unset -f kubectl
echo '[pass] Work history is read only through a port-forward that reported its port and stayed alive'

# --- load ---
write_build() { # source [archive]
  local tag src="${2:-$TMP/fake.tar}"
  mkdir -p "$OUTPUT/images"
  printf 'source %s\n' "$1" >"$OUTPUT/build-identity.txt"
  for tag in agenova-control-plane:0.1.0 agenova-testworker:kind agenova-e16-mcp:testsha12345 agenova-e16-probe:testsha12345; do
    cp "$src" "$(image_archive "$tag")"
    printf 'x tag=%s local-id=y config=%s archive-sha256=%s\n' "$tag" "$RECORDED" "$(shasum -a 256 "$src" | cut -d' ' -f1)" >>"$OUTPUT/build-identity.txt"
  done
}
protected_state() { printf 'image agenova-control-plane:0.1.0 %s x\ncomplete now\n' "$RECORDED" >"$OUTPUT/protect-state.txt"; }

reset l1; write_build testsha; printf 'image agenova-control-plane:0.1.0 %s x\n' "$RECORDED" >"$OUTPUT/protect-state.txt"
expect_fail "load after an incomplete protect" "missing or incomplete" load; not_called 'kind load'
reset l2; write_build testsha; protected_state
expect_fail "load while the old Deployment is scaled up" "protect no longer holds" load; not_called 'kind load'
reset l3; write_build testsha; protected_state; OTHER_REPLICAS=0; OTHER_PODS='agenova-system/old Running x'
expect_fail "load while old pods still run" "still run the fixed tags" load; not_called 'kind load'
reset l7; write_build testsha; protected_state; OTHER_FAIL=1
expect_fail "load when the inventory query fails" "could not list installs" load; not_called 'kind load'
reset l8; write_build testsha; protected_state; OTHER_REPLICAS=0; PODS_FAIL=1
expect_fail "load when the pod query fails" "could not list pods" load; not_called 'kind load'
echo '[pass] load requires a complete protect whose effect still holds'

reset l4; write_build testsha; protected_state; OTHER_REPLICAS=0
printf 'tampered' >>"$(image_archive agenova-testworker:kind)"
expect_fail "load a replaced archive" "does not match its recorded hash" load; not_called 'kind load'
reset l5; write_build othersha; protected_state; OTHER_REPLICAS=0
expect_fail "load archives built from another commit" "another commit" load; not_called 'kind load'
reset l9; write_build testsha "$TMP/partial.tar"; protected_state; OTHER_REPLICAS=0
expect_fail "load a recorded archive missing a referenced manifest" "lacks content its index references" load; not_called 'kind load'
echo '[pass] load refuses archives that do not match the build record for this commit or could not be imported'

reset l6; write_build testsha; protected_state; OTHER_REPLICAS=0
load >/dev/null
[ "$(grep -c 'kind load image-archive' "$LOG")" -eq 4 ]
echo '[pass] load imports exactly the four recorded archives'

# --- restore ---
reset r1; protected_state; printf 'scale Deployment agenova-system agenova-control-plane 1\n' >>"$OUTPUT/protect-state.txt"
NODE_ID="sha256:$(printf 'c%.0s' $(seq 1 64))"
E16_PODS='agenova-e16-system/cp Running agenova-control-plane:0.1.0'
expect_fail "restore while E16 pods keep running" "nothing was restored" restore
not_called 'kind load'; [ -e "$OUTPUT/protect-state.txt" ]
E16_PODS=""; PODS_FAIL=1; : >"$LOG"
expect_fail "restore when the E16 pod query fails" "nothing was restored" restore
not_called 'kind load'; [ -e "$OUTPUT/protect-state.txt" ]
echo '[pass] restore stops E16 first and changes no image while E16 pods run'

reset r2; cp "$TMP/fake.tar" "$TMP/protected.tar"
printf 'image agenova-control-plane:0.1.0 %s %s\nscale Deployment agenova-system agenova-control-plane 1\ncomplete now\n' "$RECORDED" "$TMP/protected.tar" >"$OUTPUT/protect-state.txt"
NODE_ID="sha256:$(printf 'c%.0s' $(seq 1 64))"
FAIL_ON='rollout status'
expect_fail "restore before the old install is Ready" "is not Ready" restore
[ -e "$OUTPUT/protect-state.txt" ]
[ "$(line_of 'scale deployment agenova-control-plane --replicas=0')" -lt "$(line_of 'kind load image-archive')" ]
FAIL_ON=''; : >"$LOG"
restore >/dev/null
not_called 'kind load image-archive'   # already restored: repeat skips import
called 'scale deployment agenova-control-plane --replicas=1'; called 'rollout status'
[ ! -e "$OUTPUT/protect-state.txt" ]
restore >/dev/null
echo '[pass] restore keeps state until the old install is Ready and can be repeated safely'

reset r3; cp "$TMP/partial.tar" "$TMP/protected.tar"
printf 'image agenova-control-plane:0.1.0 %s %s\nscale Deployment agenova-system agenova-control-plane 1\ncomplete now\n' "$RECORDED" "$TMP/protected.tar" >"$OUTPUT/protect-state.txt"
NODE_ID="sha256:$(printf 'c%.0s' $(seq 1 64))"
expect_fail "restore from an archive missing a referenced manifest" "no valid archive to restore" restore
not_called 'kind load image-archive'; not_called 'replicas=1'; [ -e "$OUTPUT/protect-state.txt" ]
echo '[pass] restore imports only a complete archive and keeps its state otherwise'

# --- fixture rendering and install guard ---
reset f1
render_fixture "$OUTPUT/fixture.yaml"
grep -q '^        image: agenova-e16-mcp:testsha12345$' "$OUTPUT/fixture.yaml"
! grep -q 'agenova-e16-mcp:0.1.0' "$OUTPUT/fixture.yaml"
echo '[pass] the fixture renders locally with exactly the recorded source-derived image'

reset i1; write_build testsha; E16_DEPLOYED=1; WORK_JSON='[{"requestRef":"e16-positive"}]'
check_platform_targets() { :; }
expect_fail "install while E16 Work is active" "E16 Work is active" install
not_called 'platform apply'
E16_DEPLOYED=""
reset i2; write_build testsha; FAIL_ON='get deployment agenova-control-plane'
expect_fail "install when the control plane query fails" "could not query the E16 control plane" install
not_called 'platform apply'
echo '[pass] install never applies while E16 Work is active or unknown'

# --- identity chain, log continuity and per-Work gates ---
# The real resolver against a saved node image list, and the mapping archive
# each identity check keeps next to its identity file.
reset c0; write_build testsha
IMPORT="docker.io/library/import-2026-10-04@sha256:$(hex 7)"
SHARED="docker.io/library/import-2026-10-04@sha256:$(hex 9)"
NODE_IMAGES="{\"images\":[{\"id\":\"$RECORDED\",\"repoTags\":[\"docker.io/library/agenova-control-plane:0.1.0\"],\"repoDigests\":[\"$IMPORT\",\"$SHARED\"]},{\"id\":\"sha256:$(hex 8)\",\"repoTags\":[],\"repoDigests\":[\"$SHARED\"]}]}"
printf 'cp-1 uid=1 restarts=0 container=containerd://c imageID=%s\n' "$IMPORT" >"$OUTPUT/cp.txt"
verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/cp.txt"
[ "$(cat "$OUTPUT/cp.txt.node-images.json")" = "$NODE_IMAGES" ] || { echo "[fail] the node image list was not archived"; exit 1; }
grep -qx "tag agenova-control-plane:0.1.0 recorded-config $RECORDED" "$OUTPUT/cp.txt.mapping"
grep -qx "node-images cp.txt.node-images.json sha256 $(shasum -a 256 "$OUTPUT/cp.txt.node-images.json" | cut -d' ' -f1)" "$OUTPUT/cp.txt.mapping"
grep -qx "pod $(cat "$OUTPUT/cp.txt")" "$OUTPUT/cp.txt.mapping"
grep -qx "  resolves to $RECORDED" "$OUTPUT/cp.txt.mapping" || { echo "[fail] the resolution was not archived"; exit 1; }
printf 'cp-2 uid=2 restarts=0 container=containerd://c imageID=%s\n' "$SHARED" >"$OUTPUT/shared.txt"
expect_fail "a Pod image two node images carry" "resolves to nothing" verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/shared.txt"
grep -qx "  resolves to nothing" "$OUTPUT/shared.txt.mapping" || { echo "[fail] a failed resolution was not archived"; exit 1; }
printf 'cp-3 uid=3 restarts=0 container= imageID=\n' >"$OUTPUT/empty.txt"
expect_fail "a Pod without an image" "resolves to nothing" verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/empty.txt"
FAIL_ON='crictl images'
expect_fail "a node image list that cannot be read" "could not list the node images" verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/cp.txt"
echo '[pass] each identity check resolves against one saved node image list and archives it with every resolution'

reset c1; write_build testsha
image_ref_id() { case "$1" in good) printf '%s' "$RECORDED" ;; *) printf 'sha256:other' ;; esac; }
printf 'pod-a uid=1 restarts=0 container=c imageID=good\n' >"$OUTPUT/ok.txt"
verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/ok.txt"
printf 'pod-a uid=1 restarts=0 container=c imageID=good\npod-b uid=2 restarts=0 container=c imageID=replaced\n' >"$OUTPUT/mixed.txt"
expect_fail "a Pod on a replaced image" "resolves to sha256:other" verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/mixed.txt"
: >"$OUTPUT/none.txt"
expect_fail "no Pod identity captured" "no Pod identity captured" verify_identity_lines agenova-control-plane:0.1.0 "$OUTPUT/none.txt"
NODE_ID="sha256:$(printf 'd%.0s' $(seq 1 64))"
expect_fail "a node tag replaced after load" "the image changed after load" verify_node_tag agenova-control-plane:0.1.0
echo '[pass] image identity is enforced from the build record to the running Pods'

reset c2
expect_fail "a collector that never started" "is not running" require_collector fixture
command sleep 30 &
collector=$!
echo "$collector" >"$OUTPUT/fixture-follow.pid"
require_collector fixture
kill "$collector"; wait "$collector" 2>/dev/null || true
expect_fail "a collector that exited" "is not running" require_collector fixture
printf 'a\nb\nc\n' >"$OUTPUT/follow"; printf 'a\nb\nc\nd\n' >"$OUTPUT/full"
check_log_continuity "$OUTPUT/follow" "$OUTPUT/full"
printf 'a\nc\n' >"$OUTPUT/truncated"
expect_fail "a truncated snapshot" "missing from the snapshot" check_log_continuity "$OUTPUT/follow" "$OUTPUT/truncated"
expect_fail "a missing snapshot" "missing or unreadable" check_log_continuity "$OUTPUT/follow" "$OUTPUT/absent"
expect_fail "a missing collector log" "missing or unreadable" check_log_continuity "$OUTPUT/absent" "$OUTPUT/full"
echo '[pass] a stopped collector or truncated log snapshot invalidates the window'

reset c3; write_build testsha
printf '{"state":{"claim":{"backendIdentity":{"workerId":"agenova-pool-x"}}}}' >"$OUTPUT/show.json"
printf 'agenova-pool-other uid=1 restarts=0 container=c imageID=good\n' >"$OUTPUT/workers.txt"
expect_fail "claimed worker never captured" "was never captured" verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/workers.txt"
printf 'agenova-pool-x uid=1 restarts=0 container=c imageID=good\n' >>"$OUTPUT/workers.txt"
verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/workers.txt"
# L7: the first Work creates the warm pool, so its worker can be captured
# Pending or ContainerCreating, with no container and no image yet.
printf 'agenova-pool-x uid=1 restarts= container= imageID=\nagenova-pool-x uid=1 restarts=0 container= imageID=\n' >"$OUTPUT/prestart.txt"
{ cat "$OUTPUT/prestart.txt"; printf 'agenova-pool-x uid=1 restarts=0 container=c imageID=good\n'; } >"$OUTPUT/started.txt"
verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/started.txt"
[ "$(cat "$OUTPUT/started.txt.running")" = 'agenova-pool-x uid=1 restarts=0 container=c imageID=good' ] && [ -s "$OUTPUT/started.txt.running.mapping" ] ||
  { echo "[fail] the started worker capture and its mapping were not kept"; exit 1; }
expect_fail "claimed worker captured only before its container started" "never captured with a started container" verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/prestart.txt"
# Skipping a capture without a container never excuses a started one.
{ cat "$OUTPUT/started.txt"; printf 'agenova-pool-x uid=1 restarts=1 container=c2 imageID=replaced\n'; } >"$OUTPUT/replaced.txt"
expect_fail "a started worker capture on another image" "resolves to sha256:other" verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/replaced.txt"
{ cat "$OUTPUT/started.txt"; printf 'agenova-pool-x uid=1 restarts=0 container=c imageID=\n'; } >"$OUTPUT/noimage.txt"
expect_fail "a worker capture with a container but no image" "resolves to sha256:other" verify_claimed_worker "$OUTPUT/show.json" "$OUTPUT/noimage.txt"
reset c4
expect_fail "work while the fixture collector is down" "is not running" work positive
not_called "run -f"
reset c6; write_build testsha
command sleep 30 & fixture_pid=$!; command sleep 30 & cp_pid=$!
echo "$fixture_pid" >"$OUTPUT/fixture-follow.pid"; echo "$cp_pid" >"$OUTPUT/control-plane-follow.pid"
printf 'agenova-control-plane-1 uid=1 restarts=0 container=c imageID=good\n' >"$OUTPUT/control-plane-identity.txt"
expect_fail "work after the control plane Pod changed" "changed since install" work positive
not_called "run -f"
kill "$fixture_pid" "$cp_pid"; wait "$fixture_pid" "$cp_pid" 2>/dev/null || true
echo '[pass] each Work needs live collectors and its claimed worker on the recorded image; captures before its container started are skipped'

# --- install transcript (Codex finding 1) ---
INSTALL_STEPS="adapters-install-deployment-kubernetes adapters-install-runtime-agent-sandbox adapters-install-model-openai-compatible adapters-install-tool-mcp-http platform-validate platform-plan platform-apply platform-status policy-apply agent-template-apply platform-reapply platform-status-after-reapply rollout-status"
reset i3; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'
( start_collector() { :; }; install >/dev/null )
dirs="$(ls -d "$OUTPUT"/install/*/)"
[ "$(printf '%s\n' "$dirs" | grep -c .)" = 1 ] || { echo "[fail] install did not keep exactly one attempt directory"; exit 1; }
dir="${dirs%/}"
for step in $INSTALL_STEPS; do
  [ -e "$dir/$step.txt" ] && grep -q "^command $step " "$dir/commands.txt" && grep -qx "exit $step 0" "$dir/commands.txt" ||
    { echo "[fail] install step $step has no transcript or exit status"; exit 1; }
done
[ "$(grep -c '^command ' "$dir/commands.txt")" = 13 ] || { echo "[fail] install recorded unexpected commands"; exit 1; }
grep -qx 'plan: 9 changes' "$dir/platform-plan.txt" && grep -qx 'validate output on stderr' "$dir/platform-validate.txt" ||
  { echo "[fail] install transcripts lost stdout or stderr"; exit 1; }
grep -q "^command platform-plan .* --state-dir $STATE_DIR platform plan -f $SCRIPT_DIR/platform.yaml\$" "$dir/commands.txt"
grep -q "^command rollout-status kubectl --context kind-x -n agenova-e16-system rollout status " "$dir/commands.txt"
[ -s "$OUTPUT/control-plane-identity.txt.mapping" ] || { echo "[fail] the control-plane mapping was not archived"; exit 1; }
reset i4; write_build testsha; FAIL_ON='platform apply'
expect_fail "install whose first apply fails" "install step platform-apply exited 1" install
dir="$(ls -d "$OUTPUT"/install/*/)"; dir="${dir%/}"
grep -qx 'exit platform-apply 1' "$dir/commands.txt" && ! grep -q '^command policy-apply' "$dir/commands.txt" ||
  { echo "[fail] a failed install step was not recorded, or install went on"; exit 1; }
not_called 'policy apply'
reset i5; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'
date() { echo 20261004T000000Z; }
( start_collector() { :; }; install >/dev/null )
cp "$OUTPUT/install/20261004T000000Z/commands.txt" "$OUTPUT/first-commands.txt"
expect_fail "a second install attempt in the same directory" "exists; wait a second and rerun" install
cmp -s "$OUTPUT/install/20261004T000000Z/commands.txt" "$OUTPUT/first-commands.txt" || { echo "[fail] a rerun overwrote an install transcript"; exit 1; }
unset -f date
echo '[pass] install keeps every command, its output and exit status per attempt, and stops at the first failure'

reset c7
printf '{"a":1}\n{"b":2}\n{"c":' >"$OUTPUT/live"
freeze_complete_lines "$OUTPUT/live" "$OUTPUT/frozen"
[ "$(cat "$OUTPUT/frozen")" = "$(printf '{"a":1}\n{"b":2}')" ] || { echo "[fail] a partial record was frozen"; exit 1; }
printf '{"a":1}\n{"b":2}\n{"c":3}\n' >"$OUTPUT/snapshot"
check_log_continuity "$OUTPUT/frozen" "$OUTPUT/snapshot"
printf '{"facts":[{"operation":"tool.invoke","invocationId":"t1","reasonCode":"tool-timeout"},{"operation":"tool.invoke","invocationId":"t1"},{"operation":"tool.invoke","invocationId":"ok1","reasonCode":"configured-tool"},{"operation":"model.invoke","invocationId":"m1"}]}' >"$OUTPUT/show.json"
[ "$(work_invocations "$OUTPUT/show.json" | sort | tr '\n' ' ')" = "ok1 t1 " ] || { echo "[fail] invocation extraction"; exit 1; }
[ "$(work_invocations "$OUTPUT/show.json" tool-timeout)" = "t1" ] || { echo "[fail] timeout extraction"; exit 1; }
printf '{"facts":[{"kind":"ToolDecision","operation":"tool.invoke","invocationId":"t1","timestamp":"2026-10-03T01:00:00Z"},{"kind":"ProviderAttempt","operation":"tool.invoke","invocationId":"t1","timestamp":"2026-10-03T01:00:00.5Z"},{"kind":"ProviderOutcome","operation":"tool.invoke","invocationId":"t1","reasonCode":"tool-timeout","timestamp":"2026-10-03T01:00:05Z"},{"kind":"ToolDecision","operation":"tool.invoke","invocationId":"d1","timestamp":"2026-10-03T01:00:06Z"}]}' >"$OUTPUT/records.json"
[ "$(work_prior_records "$OUTPUT/records.json" | sort | tr '\n' '|')" = "d1 2026-10-03T01:00:06Z denied|t1 2026-10-03T01:00:05Z tool-timeout|" ] || { echo "[fail] prior records: $(work_prior_records "$OUTPUT/records.json")"; exit 1; }
# The end is the latest instant, not the lexically largest string.
printf '{"facts":[{"kind":"ToolDecision","operation":"tool.invoke","invocationId":"s1","timestamp":"2026-10-03T01:00:00.001Z"},{"kind":"ProviderAttempt","operation":"tool.invoke","invocationId":"s1","timestamp":"2026-10-03T01:00:00.002Z"},{"kind":"ProviderOutcome","operation":"tool.invoke","invocationId":"s1","reasonCode":"configured-tool","timestamp":"2026-10-03T01:00:00.002500001Z"},{"kind":"ProviderAttempt","operation":"tool.invoke","invocationId":"n1","timestamp":"2026-10-03T01:00:00Z"}]}' >"$OUTPUT/instants.json"
[ "$(work_prior_records "$OUTPUT/instants.json" | sort | tr '\n' '|')" = "n1 2026-10-03T01:00:00Z no-outcome|s1 2026-10-03T01:00:00.002500001Z configured-tool|" ] || { echo "[fail] instant ordering: $(work_prior_records "$OUTPUT/instants.json")"; exit 1; }
FIXTURE_LOG='{"event":"receipt","correlation":"t1"}'
expect_fail "a timed-out call that never finishes on the server" "did not finish on the server" settle_timeouts "$OUTPUT/show.json"
FIXTURE_LOG='{"event":"tool","correlation":"t1","outcome":"ok"}'
settle_timeouts "$OUTPUT/show.json"
FIXTURE_LOG=""
echo '[pass] partial log records are not frozen, and a timed-out call settles before the next Work'

# --- controlled read (Phase 2 exit) ---
ev() { printf '{"time":"2026-10-03T01:00:00Z",%s,"correlation":"inv-42"%s}\n' "$1" "${2:+,$2}"; }
# The interop test's three calls as the fixture logs them (the shape of a
# real local fixture run): one session each, responses interleaved.
good_read() {
  local i=0 f s handler
  for f in README.md logs/full-trace.log missing.md; do
    i=$((i + 1)); s="\"session\":\"s$i\""; handler='"outcome":"ok"'
    [ "$f" != missing.md ] || handler='"outcome":"error","error":"not-found"'
    ev '"event":"receipt","httpMethod":"POST","rpcMethod":"initialize","rpcId":"1"'
    ev '"event":"response","httpMethod":"POST","rpcMethod":"initialize","rpcId":"1","status":200'
    ev '"event":"receipt","httpMethod":"POST","rpcMethod":"notifications/initialized"' "$s"
    ev '"event":"response","httpMethod":"POST","rpcMethod":"notifications/initialized","status":202'
    ev "\"event\":\"receipt\",\"httpMethod\":\"POST\",\"rpcMethod\":\"tools/call\",\"rpcId\":\"2\",\"file\":\"$f\"" "$s"
    ev "\"event\":\"tool\",\"file\":\"$f\",$handler"
    ev '"event":"response","httpMethod":"POST","rpcMethod":"tools/call","rpcId":"2","status":200'
    ev '"event":"receipt","httpMethod":"DELETE"' "$s"
    ev '"event":"response","httpMethod":"DELETE","status":204'
  done
}
first_only() { awk -v pat="$1" -v rep="$2" 'index($0, pat) && !done {sub(pat, rep); done=1} {print}'; }
reset cr1
{ good_read; printf '{"time":"2026-10-03T01:00:00Z","event":"receipt","rpcMethod":"tools/call","file":"README.md","correlation":"other"}\n'; } >"$OUTPUT/good.jsonl"
check_controlled_read "$OUTPUT/good.jsonl"
good_read | grep -v '"event":"tool","file":"README.md"' >"$OUTPUT/nohandler.jsonl"
{ good_read; ev '"event":"receipt","httpMethod":"POST","rpcMethod":"tools/call","rpcId":"2","file":"README.md"' '"session":"s1"'; } >"$OUTPUT/twice.jsonl"
{ good_read; ev '"event":"tool","file":"README.md","outcome":"error","error":"not-found"'; } >"$OUTPUT/extrahandler.jsonl"
{ good_read; echo 'not json'; } >"$OUTPUT/garbage.jsonl"
good_read | sed 's/"outcome":"error","error":"not-found"/"outcome":"ok"/' >"$OUTPUT/wrongmissing.jsonl"
good_read | first_only '"file":"README.md","outcome":"ok"' '"file":"README.md","outcome":"error"' >"$OUTPUT/readmefailed.jsonl"
good_read | awk '/"event":"receipt","httpMethod":"DELETE"/ && !n++ {next} {print}' >"$OUTPUT/unclosed.jsonl"
good_read | sed '/"httpMethod":"DELETE"/s/"session":"s[0-9]"/"session":"s1"/' >"$OUTPUT/samedelete.jsonl"
good_read | sed 's/"session":"s2"/"session":"s1"/' >"$OUTPUT/reused.jsonl"
good_read | awk 'NR == 3 {held = $0; next} NR == 5 {print; print held; next} {print}' >"$OUTPUT/outoforder.jsonl"
good_read | first_only '"status":200' '"status":500' >"$OUTPUT/badresponse.jsonl"
good_read | awk '/"event":"response"/ && !n++ {next} {print}' >"$OUTPUT/noresponse.jsonl"
# Twelve 2xx responses, but the DELETEs are answered by repeated initialize responses.
good_read | awk '/"event":"response","httpMethod":"DELETE"/ {print init; next} /"event":"response","httpMethod":"POST","rpcMethod":"initialize"/ {init = $0} {print}' >"$OUTPUT/dupresponse.jsonl"
good_read | first_only '"rpcMethod":"tools/call","rpcId":"2","status":200' '"rpcMethod":"tools/call","rpcId":"9","status":200' >"$OUTPUT/wrongid.jsonl"
good_read | awk 'NR == 1 {held = $0; next} NR == 2 {print; print held; next} {print}' >"$OUTPUT/early.jsonl"
while IFS='|' read -r bad want; do
  expect_fail "controlled read log $bad" "$want" check_controlled_read "$OUTPUT/$bad.jsonl"
done <<'EOF'
nohandler|14 request and handler entries, want 15
twice|16 request and handler entries, want 15
extrahandler|16 request and handler entries, want 15
garbage|unreadable log
wrongmissing|call 3 (missing.md) is not
readmefailed|call 1 (README.md) is not
unclosed|14 request and handler entries, want 15
samedelete|call 2 (logs/full-trace.log) is not
reused|call 2 reuses session s1
outoforder|call 1 (README.md) is not
badresponse|non-2xx response POST initialize 1: 500
noresponse|unanswered requests: POST initialize 1
dupresponse|unmatched response POST initialize 1
wrongid|unmatched response POST tools/call 9
early|unmatched response POST initialize 1
EOF
echo '[pass] the controlled read must show the three interop calls, each in its own complete session, with exact handler outcomes'

kubectl() { echo 'Forwarding from 127.0.0.1:43123 -> 8080'; command sleep 5; }
eval "real_$(declare -f host_go)"
cr_setup() { # name
  reset "$1"; write_build testsha
  command sleep 30 & fixture_pid=$!
  echo "$fixture_pid" >"$OUTPUT/fixture-follow.pid"
  : >"$OUTPUT/fixture-identity-start.txt"; : >"$OUTPUT/fixture-follow.jsonl"
}
reset cr2; write_build testsha; E16_DEPLOYED=1
expect_fail "controlled read after install" "runs only before install" controlled_read
E16_DEPLOYED=""
reset cr3; write_build testsha; FAIL_ON='get deployment agenova-control-plane'
expect_fail "controlled read when the control plane query fails" "could not query the E16 control plane" controlled_read
reset cr4; write_build testsha; mkdir -p "$OUTPUT/controlled-read"
expect_fail "controlled read twice" "recorded once per campaign" controlled_read
not_called 'go test'
cr_setup cr5; good_read >"$OUTPUT/fixture-follow.jsonl"
expect_fail "controlled read when the log already holds its calls" "already holds inv-42 entries" controlled_read
not_called 'go test'
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
echo '[pass] the controlled read runs once, before install, on a log without its calls'

cr_setup cr6   # the stubbed go test prints nothing
expect_fail "controlled read whose interop test did not pass" "did not pass against the in-cluster fixture" controlled_read
called 'go test ./internal/adapters/bundled -run'
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
cr_setup cr7
host_go() { printf -- '--- PASS: %s (0.10s)\n' "$INTEROP_TEST"; }
expect_fail "controlled read the collector never saw" "did not capture the controlled read" controlled_read
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
cr_setup cr8; FIXTURE_LOG="$(good_read)"
host_go() { printf -- '--- PASS: %s (0.10s)\n' "$INTEROP_TEST"; good_read >>"$OUTPUT/fixture-follow.jsonl"; }
controlled_read >/dev/null
[ -s "$OUTPUT/controlled-read/fixture-full.jsonl" ] && grep -q 'controlled-read pass' "$OUTPUT/campaign.log"
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
host_go() { real_host_go "$@"; }; FIXTURE_LOG=""; unset -f kubectl
echo '[pass] the controlled read passes only when the interop test passed and the collector captured it'

reset c5
printf '{"stats":{"expected":2,"unexpected":0,"skipped":0,"flaky":0}}' >"$OUTPUT/r.json"; parity_report_ok "$OUTPUT/r.json"
for bad in '{"stats":{"expected":1,"unexpected":0,"skipped":1,"flaky":0}}' '{"stats":{"expected":6,"unexpected":0,"skipped":0,"flaky":0}}' '{"stats":{"expected":2,"unexpected":1,"skipped":0,"flaky":0}}' 'not json'; do
  printf '%s' "$bad" >"$OUTPUT/r.json"
  if parity_report_ok "$OUTPUT/r.json"; then echo "[fail] accepted parity report: $bad"; exit 1; fi
done
echo '[pass] parity counts only a run of exactly the setup and case tests'

# --- parity servers (L8) ---
# Stand-ins on free loopback ports: api connect runs like the real CLI, whose
# kubectl port-forward child keeps the port when its parent is terminated;
# npm keeps a Vite stand-in below it.
free_port() { node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }
API_PORT="$(free_port)" UI_PORT="$(free_port)"
while [ "$UI_PORT" = "$API_PORT" ]; do UI_PORT="$(free_port)"; done
export API_PORT UI_PORT
LISTEN_JS='require("net").createServer(c=>c.destroy()).listen(Number(process.argv[1]),"127.0.0.1",()=>{if(process.argv[2])console.log(process.argv[2])})'
fake_cli() { # mode: forward | exit
  mkdir -p "$OUTPUT/bin"
  if [ "$1" = forward ]; then
    printf '#!/bin/bash\nnode -e %q "$API_PORT" "Forwarding from 127.0.0.1:$API_PORT -> 8081" &\nwait\n' "$LISTEN_JS" >"$OUTPUT/bin/agenova"
  else
    printf '#!/bin/bash\necho "error: unable to listen on port $API_PORT" >&2\nexit 1\n' >"$OUTPUT/bin/agenova"
  fi
  chmod +x "$OUTPUT/bin/agenova"
}
npm() { node -e "$LISTEN_JS" "$UI_PORT"; }
# Polling loops need real (short) pauses here.
sleep() { command sleep 0.05; }
no_listeners() {
  ! port_listening "$API_PORT" && ! port_listening "$UI_PORT" || { echo "[fail] $1: a parity port still has a listener"; exit 1; }
}

reset pa1; fake_cli forward
parity positive >/dev/null
no_listeners "after a passing parity run"
grep -q "^Forwarding from 127.0.0.1:$API_PORT -> 8081\$" "$OUTPUT/work/positive/parity/api-connect.log"
called 'npx playwright test'
reset pa2; fake_cli forward
node -e "$LISTEN_JS" "$API_PORT" >/dev/null 2>&1 & blocker=$!
for _ in $(seq 1 50); do port_listening "$API_PORT" && break; sleep; done
expect_fail "parity while the API port is taken" "port $API_PORT is already in use" parity positive
not_called 'playwright'; [ ! -e "$OUTPUT/work/positive/parity" ]
kill "$blocker"; wait "$blocker" 2>/dev/null || true
for _ in $(seq 1 50); do port_listening "$API_PORT" || break; sleep; done
node -e "$LISTEN_JS" "$UI_PORT" >/dev/null 2>&1 & blocker=$!
for _ in $(seq 1 50); do port_listening "$UI_PORT" && break; sleep; done
expect_fail "parity while the UI port is taken" "port $UI_PORT is already in use" parity positive
kill "$blocker"; wait "$blocker" 2>/dev/null || true
for _ in $(seq 1 50); do port_listening "$UI_PORT" || break; sleep; done
reset pa3; fake_cli forward; FAIL_ON='playwright'
expect_fail "parity whose tests fail" "parity failed for positive" parity positive
no_listeners "after a failing parity run"
reset pa4; fake_cli exit
expect_fail "parity whose api connect never forwards" "did not forward 127.0.0.1:$API_PORT" parity positive
not_called 'playwright'
no_listeners "after api connect failed"
# SIGTERM while the tests run, after both servers are up, stops them too.
reset pa5; fake_cli forward; PLAYWRIGHT_BLOCK=31.7
( parity positive >/dev/null 2>&1 ) & runner=$!
for _ in $(seq 1 100); do called 'playwright test' && port_listening "$UI_PORT" && break; sleep; done
called 'playwright test' && port_listening "$API_PORT" && port_listening "$UI_PORT" || { echo "[fail] parity never reached its tests"; exit 1; }
kill -TERM "$runner"
set +e; wait "$runner"; rc=$?; set -e
[ "$rc" = 143 ] || { echo "[fail] parity stopped by SIGTERM exited $rc, want 143"; exit 1; }
no_listeners "after SIGTERM during the parity tests"
! pgrep -f "sleep 31.7" >/dev/null || { echo "[fail] the interrupted test run was left behind"; exit 1; }
# A passing run restores the traps it replaced.
reset pa6; fake_cli forward
trap 'rm -rf "$TMP"' EXIT
parity positive >/dev/null
[ "$(trap -p EXIT)" = "trap -- 'rm -rf \"\$TMP\"' EXIT" ] && [ -z "$(trap -p TERM)" ] || { echo "[fail] parity did not restore the traps: $(trap -p EXIT TERM)"; exit 1; }
no_listeners "after a passing run with traps"
sleep() { :; }; unset -f npm
echo '[pass] parity needs both ports free, uses only its own tunnel and leaves nothing listening on any exit, including SIGTERM and kubectl below api connect'

# --- host binaries actually run ---
if [ "$(uname -s)" = Darwin ]; then
  [ "$(host_goflags)" = "-ldflags=-linkmode=external" ] || { echo "[fail] macOS host builds must link externally"; exit 1; }
fi
( run() { "$@"; }; cd "$ROOT" && host_go build -o "$TMP/agenova-host" ./cmd/agenova ) ||
  { echo "[fail] host CLI build failed"; exit 1; }
"$TMP/agenova-host" --help >/dev/null 2>&1 || { echo "[fail] the host CLI built by host_go does not run"; exit 1; }
echo '[pass] host-built binaries (CLI, evidence checker) run on this host'

printf '=== RUN   TestE16Probes\n    skipped\n--- SKIP: TestE16Probes (0.00s)\nPASS\n' >"$TMP/skip.log"
expect_fail "skipped probe run" "did not pass" validate_probe_output "$TMP/skip.log"
printf '=== RUN   TestE16Probes\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/empty.log"
expect_fail "probe run without receipts" "no receipts" validate_probe_output "$TMP/empty.log"
printf 'E16_PROBE {}\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/ok.log"
validate_probe_output "$TMP/ok.log"
echo '[pass] a probe Job that skipped or emitted nothing is rejected even with exit 0'
