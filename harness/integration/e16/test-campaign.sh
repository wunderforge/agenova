#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# Offline checks for campaign.sh: argument guards, protect/load/restore gates,
# identity and install records, the controlled read, parity cleanup, Work
# attempts and probe output validation, and the Slice 4 token Secrets, worker
# captures and final scan. No command reaches Docker, kind or a cluster; the
# parity checks start local stand-in listeners on free loopback ports.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT="$ROOT/harness/integration/e16/campaign.sh"
# shellcheck source=campaign.sh
source "$SCRIPT"
# The stubs drain their stdin, so nothing here may read the terminal.
exec </dev/null

TMP="$(mktemp -d)"
# bash 3.2 runs an EXIT trap with status 0 after an unbound variable, which
# would read as a pass; only a run that reached its last line passes.
COMPLETE=""
on_exit() { rm -rf "$TMP"; [ -n "$COMPLETE" ] || exit 1; }
trap on_exit EXIT
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

# Synthetic tokens (64 lowercase hex): two seeded into fake Secrets, and the
# values the stubbed openssl prints, in order. A campaign generates two; a
# third call is a regeneration and stops the test on an unbound GEN_3.
TOKEN_F=2ce9bdfdaa0aa2cb3f4affb79c9c0173238b4858299089b7f536a5954a86b37a
TOKEN_W=b55f4d9a7aa9b48232cf96150a9009dc1b4360d55db4f4e5e858ffa84c63a56d
GEN_1=7911037def519c30613c3d164f50484b15a2af9e035d71eb8d03ac9a6236113f
GEN_2=3b00656fa9b6c99e8935f2b47d9957ac0a5ad6789e623cb92285d56a2f512a6a
# The fake cluster's Secrets: <namespace>_<name> holds the raw token and
# <namespace>_<name>.id its identity line; namespaces are files too.
SECRETS="$TMP/secrets" NAMESPACES="$TMP/namespaces"
fake_secret() { # kubectl arguments
  local ns="" name="" prev="" a
  for a in "$@"; do
    case "$prev" in -n) ns="$a" ;; secret | generic) name="$a" ;; esac
    prev="$a"
  done
  printf '%s/%s_%s' "$SECRETS" "$ns" "$name"
}
seed_secret() { # namespace name value
  printf '%s' "$3" >"$SECRETS/$1_$2"
  printf '%s/%s uid=uid-seed-%s resourceVersion=7' "$1" "$2" "$2" >"$SECRETS/$1_$2.id"
}
failing() { [ -n "$FAIL_ON" ] && [[ "$*" == *"$FAIL_ON"* ]]; }

# Stubs: every external command is logged, nothing runs. Each drains its
# stdin as the real command would, so a pipeline into it never dies of
# SIGPIPE; kubectl create secret and the scan keep what they read.
FAIL_ON=""
run() {
  local f="" n="" answer="" all="$*"
  printf '%s\n' "$all" >>"$LOG"
  case "$*" in
    *"openssl rand -hex 32")
      n=$(($(cat "$TMP/openssl-calls" 2>/dev/null || echo 0) + 1)); echo "$n" >"$TMP/openssl-calls"
      if [ -n "${OPENSSL_OUT+set}" ]; then printf '%s' "$OPENSSL_OUT"; else eval "printf '%s\n' \"\$GEN_$n\""; fi ;;
    *" get namespace "*" -o name --ignore-not-found") f="${all#* get namespace }"; f="${f%% *}"
      failing "$*" || [ ! -e "$NAMESPACES/$f" ] || echo "namespace/$f" ;;
    *" create namespace "*) failing "$*" || touch "$NAMESPACES/${all##* }" ;;
    *" create secret generic "*" --from-file=token=/dev/stdin") f="$(fake_secret "$@")"
      if failing "$*" || [ -e "$f" ]; then cat >/dev/null; return 1; fi
      cat >"$f"; n="${f##*/}"; printf '%s/%s uid=uid-%s resourceVersion=1' "${n%%_*}" "${n#*_}" "$n" >"$f.id" ;;
    *" get secret "*" -o name --ignore-not-found") f="$(fake_secret "$@")"; [ ! -e "$f" ] || echo "secret/${f##*_}" ;;
    *" get secret "*" -o jsonpath={.data.token}") f="$(fake_secret "$@")"; [ -e "$f" ] || return 1; base64 <"$f" | tr -d '\n' ;;
    *" get secret "*"{.metadata.uid}"*) f="$(fake_secret "$@")"; [ -e "$f" ] || return 1; cat "$f.id" ;;
    *" auth can-i get secret/e16-mcp-token-unlisted --as=system:serviceaccount:"*":agenova-control-plane") answer="${CAN_I_UNLISTED-no}" ;;
    *" auth can-i get secret/e16-mcp-token --as=system:serviceaccount:"*":agenova-control-plane") answer="${CAN_I_CP-yes}" ;;
    *" auth can-i get secret/e16-mcp-token --as=system:serviceaccount:"*":default") answer="${CAN_I_DEFAULT-no}" ;;
    *" get pods -l agents.x-k8s.io/sandbox-template-ref-hash -o json") printf '%s' "${WORKER_PODS:-}" ;;
    *" exec "*" -c agent -- cat /proc/1/environ") printf '%s' "${WORKER_ENVIRON:-}" ;;
    *" exec "*" -c agent -- cat /proc/1/mountinfo") printf '%s' "${WORKER_MOUNTINFO:-}" ;;
    *"logs deployment/agenova-control-plane") printf '%s' "${CP_LOG:-}" ;;
    *"/bin/e16-evidence scan "*) cat >"$TMP/scan-stdin"; echo 'scanned 3 files, 0 matches'; return "${SCAN_RC:-0}" ;;
    *"go build -o "*"/bin/e16-evidence "*) printf 'go-env GOTOOLCHAIN=%s GOFLAGS=%s\n' "${GOTOOLCHAIN:-}" "${GOFLAGS:-}" >>"$LOG" ;;
    *"curl "*"/mcp-token") printf '%s\n' "${TOKEN_PROBE_STATUS-401}" ;;
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
    # WAIT_FOR holds the submission until the worker watcher has written it.
    *" run -f "*) for _ in $(seq 1 200); do [ -z "${WAIT_FOR:-}" ] || [ -e "$WAIT_FOR" ] && break; command sleep 0.05; done
      [ -z "${RUN_JSON:-}" ] || cat "$RUN_JSON" ;;
    *" work show "*) [ -z "${WORK_SHOW:-}" ] || cat "$WORK_SHOW" ;;
    # What the checker would read as earlier attempts' records.
    *"evidence work "*) cat "$OUTPUT/work-invocations.txt" >"$OUTPUT/prior-at-check.txt" 2>/dev/null || : >"$OUTPUT/prior-at-check.txt" ;;
    *"playwright test"*) printf 'playwright-env case=%s ref=%s\n' "${AGENOVA_E16_CASE:-}" "${AGENOVA_E16_REF:-}" >>"$LOG"
      [ -z "${PLAYWRIGHT_BLOCK:-}" ] || command sleep "$PLAYWRIGHT_BLOCK"
      [ -z "${PLAYWRIGHT_JSON_OUTPUT_NAME:-}" ] || printf '{"stats":{"expected":2,"unexpected":0,"skipped":0,"flaky":0}}' >"$PLAYWRIGHT_JSON_OUTPUT_NAME" ;;
    *"kind get clusters") printf '%s\n' "${KIND_CLUSTERS-x}" ;;
  esac
  cat >/dev/null
  if [ -n "$answer" ]; then printf '%s\n' "$answer"; [ "${answer%% *}" != no ] || return 1; fi
  if failing "$*"; then return 1; fi
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
  OUTPUT="$TMP/out-$1"; rm -rf "$OUTPUT" "$TMP/imported" "$SECRETS" "$NAMESPACES" "$TMP/openssl-calls" "$TMP/scan-stdin"
  mkdir -p "$OUTPUT" "$SECRETS" "$NAMESPACES"; : >"$LOG"
  CONTEXT=kind-x INSTALL_NAMESPACE=agenova-e16-system STATE_DIR="$TMP/s" MODEL_PROFILE=coding-standard YES=1
  OTHER_REPLICAS=1 OTHER_NONE="" OTHER_FAIL="" PODS_FAIL="" OTHER_PODS="" E16_PODS="" WORK_JSON='[]' ARCHIVE_RC=0 NODE_ID="$RECORDED" FAIL_ON="" EXPORT_TAR=""
  NODE_IMAGES="" CP_POD="" PLAYWRIGHT_BLOCK="" WORK_SHOW="" RUN_JSON="" WORKER_FAIL=""
  CAN_I_CP=yes CAN_I_UNLISTED=no CAN_I_DEFAULT=no TOKEN_PROBE_STATUS=401 SCAN_RC=0 CP_LOG="" KIND_CLUSTERS=x
  WORKER_PODS="" WORKER_ENVIRON="" WORKER_MOUNTINFO="" WORKER_LISTS=0 CAPTURES="" WORKER_LINE="" WAIT_FOR=""
  unset OPENSSL_OUT
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

# --- preflight: fresh namespaces (Slice 4) ---
# Prerequisites are stand-ins; the namespace queries go to the fake cluster.
preflight_stubbed() { ( docker() { :; }; kind() { :; }; kubectl() { :; }; go() { :; }; openssl() { :; }; base64() { :; }; preflight ); }
reset pf1
preflight_stubbed >/dev/null
grep -qx 'namespace agenova-e16-system: absent' "$OUTPUT/preflight.txt" && grep -qx 'namespace agenova-e16: absent' "$OUTPUT/preflight.txt" &&
  called 'get namespace agenova-e16-system -o name --ignore-not-found' && called 'get namespace agenova-e16 -o name --ignore-not-found' ||
  { echo "[fail] preflight did not record both namespaces as absent"; exit 1; }
FRESH="c6 needs a fresh install namespace and fixture namespace because the template ceiling changed and Secrets must be generated for this campaign"
reset pf2; touch "$NAMESPACES/agenova-e16-system"
expect_fail "preflight with an existing install namespace" "namespace agenova-e16-system exists; $FRESH" preflight_stubbed
[ ! -e "$OUTPUT/preflight.txt" ] || { echo "[fail] a refused preflight was recorded"; exit 1; }
reset pf3; touch "$NAMESPACES/agenova-e16"
expect_fail "preflight with an existing fixture namespace" "namespace agenova-e16 exists; $FRESH" preflight_stubbed
reset pf4; FAIL_ON='get namespace agenova-e16-system '
expect_fail "preflight when the install namespace query fails" "could not query namespace agenova-e16-system; not starting" preflight_stubbed
reset pf5; FAIL_ON='get namespace agenova-e16 '
expect_fail "preflight when the fixture namespace query fails" "could not query namespace agenova-e16; not starting" preflight_stubbed
[ ! -e "$OUTPUT/preflight.txt" ] || { echo "[fail] a refused preflight was recorded"; exit 1; }
echo '[pass] preflight refuses an existing install or fixture namespace, and a failed namespace query'

# --- fixture token Secret (Slice 4) ---
fixture_stubbed() {
  ( start_collector() { echo 1 >"$OUTPUT/fixture-follow.pid"; }; verify_identity_lines() { :; }
    pod_identity() { printf 'pod_identity %s\n' "$1" >>"$LOG"; printf 'e16-mcp-1 uid=1 restarts=0 container=c imageID=x\n'; }
    fixture )
}
# in_order <pattern...>: each first appears in the call log after the one before.
in_order() {
  local prev=0 n step
  for step in "$@"; do
    n="$(line_of "$step")" || n=""
    [ -n "$n" ] && [ "$n" -gt "$prev" ] || { echo "[fail] '$step' is missing or out of order in the call log"; exit 1; }
    prev="$n"
  done
}
# no_token <token...>: no token appears in the call log (every argv) or in
# any file under the output directory, campaign.log included.
no_token() {
  local token
  for token in "$@"; do
    ! grep -qF "$token" "$LOG" || { echo "[fail] a token appears in a command line"; exit 1; }
    ! grep -rqF "$token" "$OUTPUT" || { echo "[fail] a token appears under the output: $(grep -rlF "$token" "$OUTPUT")"; exit 1; }
  done
}
# none_like <glob>: the glob matched no file.
none_like() { [ ! -e "$1" ]; }
same_value() { # store-file token
  printf '%s' "$2" >"$TMP/want-token"
  cmp -s "$1" "$TMP/want-token" || { echo "[fail] $1 does not hold exactly the expected token"; exit 1; }
}
reset t1; write_build testsha
fixture_stubbed >/dev/null
no_token "$GEN_1"
in_order 'get namespace agenova-e16 -o name --ignore-not-found' 'create namespace agenova-e16' \
  '-n agenova-e16 get secret e16-mcp-token -o name --ignore-not-found' 'openssl rand -hex 32' \
  '-n agenova-e16 create secret generic e16-mcp-token --from-file=token=/dev/stdin' "apply -f $OUTPUT/fixture-rendered.yaml" \
  '-n agenova-e16 rollout restart deployment/e16-mcp' '-n agenova-e16 rollout status deployment/e16-mcp' 'pod_identity agenova-e16'
same_value "$SECRETS/agenova-e16_e16-mcp-token" "$GEN_1"
grep -q '^secret agenova-e16/e16-mcp-token uid=uid-agenova-e16_e16-mcp-token resourceVersion=1 created ' "$OUTPUT/secrets.txt" ||
  { echo "[fail] the created Secret's identity was not recorded"; exit 1; }
# A second fixture run reuses the Secret: nothing is created or generated, and
# the Pod is still restarted before its identity is taken.
: >"$LOG"
fixture_stubbed >/dev/null
not_called 'create namespace'; not_called 'openssl'; not_called 'create secret'
same_value "$SECRETS/agenova-e16_e16-mcp-token" "$GEN_1"
in_order "apply -f $OUTPUT/fixture-rendered.yaml" 'rollout restart deployment/e16-mcp' 'rollout status deployment/e16-mcp' 'pod_identity agenova-e16'
[ "$(grep -c '^secret agenova-e16/e16-mcp-token uid=uid-agenova-e16_e16-mcp-token resourceVersion=1 ' "$OUTPUT/secrets.txt")" = 2 ] &&
  grep -q ' resourceVersion=1 reused ' "$OUTPUT/secrets.txt" || { echo "[fail] the reused Secret was not recorded as the same object"; exit 1; }
# A Secret replaced since it was recorded is not reused.
printf 'agenova-e16/e16-mcp-token uid=uid-other resourceVersion=9' >"$SECRETS/agenova-e16_e16-mcp-token.id"
expect_fail "fixture with a Secret replaced since it was recorded" "Secret agenova-e16/e16-mcp-token changed since it was recorded" fixture_stubbed
reset t2; write_build testsha; touch "$NAMESPACES/agenova-e16"; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
fixture_stubbed >/dev/null
not_called 'create namespace'; not_called 'openssl'; not_called 'create secret'
same_value "$SECRETS/agenova-e16_e16-mcp-token" "$TOKEN_F"
grep -q '^secret agenova-e16/e16-mcp-token uid=uid-seed-e16-mcp-token resourceVersion=7 reused ' "$OUTPUT/secrets.txt" ||
  { echo "[fail] an existing Secret was not recorded as reused"; exit 1; }
in_order "apply -f $OUTPUT/fixture-rendered.yaml" 'rollout restart deployment/e16-mcp' 'rollout status deployment/e16-mcp' 'pod_identity agenova-e16'
no_token "$TOKEN_F"
echo '[pass] fixture creates its namespace and Secret only when absent, never regenerates one, and restarts the Pod before its identity is taken'

# The generated token is checked before kubectl create starts: an invalid one
# creates nothing and stops before the fixture is applied.
for bad in "$(printf 'A%.0s' $(seq 1 64))" "${GEN_1%?}" "${GEN_1}0" "${GEN_1%?}g" "${GEN_1%?}"$'\r' '' "${GEN_1:0:32} ${GEN_1:32:31}"; do
  reset t3; write_build testsha; OPENSSL_OUT="$bad"
  expect_fail "fixture with a generated token of ${#bad} characters that is not 64 lowercase hex" \
    "could not create Secret agenova-e16/e16-mcp-token from a valid generated token" fixture_stubbed
  called 'openssl rand -hex 32'; not_called 'create secret'; not_called 'apply -f'
  [ ! -e "$SECRETS/agenova-e16_e16-mcp-token" ] || { echo "[fail] an invalid generated token created a Secret"; exit 1; }
done
# A newline, a NUL or anything after the token is never trimmed into a valid one.
reset t4
for bad in "$GEN_1"$'\n' $'\n'"$GEN_1" "$GEN_1"' '; do
  printf '%s' "$bad" >"$TMP/bad-token"
  expect_fail "a token of ${#bad} characters with a newline or space" "" create_token_secret ns1 s1 <"$TMP/bad-token"
done
printf '%s\0%s' "$GEN_1" "$GEN_2" >"$TMP/bad-token"
expect_fail "a token followed by a NUL" "" create_token_secret ns1 s1 <"$TMP/bad-token"
not_called 'create secret'
printf '%s' "$GEN_1" | create_token_secret ns1 s1 >/dev/null
same_value "$SECRETS/ns1_s1" "$GEN_1"
reset t5; write_build testsha; seed_secret agenova-e16 e16-mcp-token "$GEN_1"$'\n'
expect_fail "fixture reusing a Secret with a trailing newline" "Secret agenova-e16/e16-mcp-token does not hold a valid token" fixture_stubbed
not_called 'apply -f'
reset t6; write_build testsha; FAIL_ON='get namespace agenova-e16 '
expect_fail "fixture when the namespace query fails" "could not query namespace agenova-e16" fixture_stubbed
not_called 'create '; not_called 'apply -f'
reset t7; write_build testsha; FAIL_ON='-n agenova-e16 get secret e16-mcp-token -o name'
expect_fail "fixture when the Secret query fails" "could not query Secret agenova-e16/e16-mcp-token" fixture_stubbed
not_called 'openssl'; not_called 'create secret'; not_called 'apply -f'
echo '[pass] a token is checked before kubectl create: a newline, NUL, non-hex or wrong-length value creates nothing'

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
reset i3; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
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
reset i5; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
date() { echo 20261004T000000Z; }
( start_collector() { :; }; install >/dev/null )
cp "$OUTPUT/install/20261004T000000Z/commands.txt" "$OUTPUT/first-commands.txt"
expect_fail "a second install attempt in the same directory" "exists; wait a second and rerun" install
cmp -s "$OUTPUT/install/20261004T000000Z/commands.txt" "$OUTPUT/first-commands.txt" || { echo "[fail] a rerun overwrote an install transcript"; exit 1; }
unset -f date
echo '[pass] install keeps every command, its output and exit status per attempt, and stops at the first failure'

# --- install token Secrets and access evidence (Slice 4) ---
install_stubbed() { ( start_collector() { :; }; install ); }
install_dir() { local d; d="$(ls -d "$OUTPUT"/install/*/)"; printf '%s' "${d%/}"; }
reset ti1; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
install_stubbed >/dev/null
dir="$(install_dir)"
no_token "$TOKEN_F" "$GEN_1"
same_value "$SECRETS/agenova-e16-system_e16-mcp-token" "$TOKEN_F"
same_value "$SECRETS/agenova-e16-system_e16-mcp-token-wrong" "$GEN_1"
[ ! -e "$SECRETS/agenova-e16-system_e16-mcp-token-absent" ] || { echo "[fail] install created the absent Secret"; exit 1; }
in_order 'platform status' '-n agenova-e16-system get secret e16-mcp-token -o name --ignore-not-found' \
  '-n agenova-e16-system create secret generic e16-mcp-token --from-file=token=/dev/stdin' 'openssl rand -hex 32' \
  '-n agenova-e16-system create secret generic e16-mcp-token-wrong --from-file=token=/dev/stdin' \
  '-n agenova-e16-system get secret e16-mcp-token-absent -o name --ignore-not-found' 'policy apply' \
  'rollout status deployment/agenova-control-plane' 'get pod -l app.kubernetes.io/name=agenova-control-plane -o json' \
  'get role agenova-control-plane-runtime -o json' 'auth can-i get secret/e16-mcp-token '
grep -q '^secret agenova-e16-system/e16-mcp-token uid=uid-agenova-e16-system_e16-mcp-token resourceVersion=1 created ' "$OUTPUT/secrets.txt" &&
  grep -q '^secret agenova-e16-system/e16-mcp-token-wrong uid=uid-agenova-e16-system_e16-mcp-token-wrong resourceVersion=1 created ' "$OUTPUT/secrets.txt" ||
  { echo "[fail] install did not record the Secrets it created"; exit 1; }
grep -qx 'compare agenova-e16-system/e16-mcp-token agenova-e16-system/e16-mcp-token-wrong mismatch' "$dir/tokens.txt" &&
  grep -q '^secret agenova-e16-system/e16-mcp-token-absent absent ' "$dir/tokens.txt" ||
  { echo "[fail] install did not record that the wrong token differs and the absent Secret is absent"; exit 1; }
[ "$(cat "$dir/can-i.txt")" = "can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:agenova-control-plane: yes (want yes)
can-i get secret/e16-mcp-token-unlisted as system:serviceaccount:agenova-e16-system:agenova-control-plane: no (want no)
can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:default: no (want no)" ] || { echo "[fail] the access reviews were not archived: $(cat "$dir/can-i.txt")"; exit 1; }
[ -e "$dir/control-plane-pod.json" ] && [ -e "$dir/control-plane-role.json" ] || { echo "[fail] the control-plane Pod and Role were not archived"; exit 1; }
[ "$(grep -c '^command ' "$dir/commands.txt")" = 13 ] && ! grep -qi 'secret\|token' "$dir/commands.txt" ||
  { echo "[fail] a token step went through install_step"; exit 1; }
not_called 'get secret .* -o yaml'; not_called 'get secret .* -o json$'; not_called 'get secrets\? -o'
# A rerun compares and keeps what exists; it creates and generates nothing.
reset ti2; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'
seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; seed_secret agenova-e16-system e16-mcp-token "$TOKEN_F"; seed_secret agenova-e16-system e16-mcp-token-wrong "$TOKEN_W"
install_stubbed >/dev/null
not_called 'create secret'; not_called 'openssl'
same_value "$SECRETS/agenova-e16-system_e16-mcp-token" "$TOKEN_F"; same_value "$SECRETS/agenova-e16-system_e16-mcp-token-wrong" "$TOKEN_W"
grep -qx 'compare agenova-e16/e16-mcp-token agenova-e16-system/e16-mcp-token match' "$(install_dir)/tokens.txt" &&
  [ "$(grep -c ' reused ' "$OUTPUT/secrets.txt")" = 2 ] || { echo "[fail] existing Secrets were not compared and recorded as reused"; exit 1; }
no_token "$TOKEN_F" "$TOKEN_W"
echo '[pass] install copies the fixture token, adds a different wrong token and records both without a token in any command line or file'

install_fails() { # description message
  expect_fail "$1" "$2" install_stubbed
  not_called 'policy apply'
}
reset ti3; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; seed_secret agenova-e16-system e16-mcp-token "$TOKEN_W"
install_fails "install with a copy that differs from the fixture token" "Secret agenova-e16-system/e16-mcp-token does not hold the fixture token (mismatch)"
grep -qx 'compare agenova-e16/e16-mcp-token agenova-e16-system/e16-mcp-token mismatch' "$(install_dir)/tokens.txt" || { echo "[fail] the mismatch was not recorded"; exit 1; }
same_value "$SECRETS/agenova-e16-system_e16-mcp-token" "$TOKEN_W"; not_called 'create secret'
reset ti4; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; seed_secret agenova-e16-system e16-mcp-token-absent "$TOKEN_W"
install_fails "install while the absent Secret exists" "Secret agenova-e16-system/e16-mcp-token-absent exists; the token-missing case needs it absent"
reset ti5; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; FAIL_ON='get secret e16-mcp-token-absent'
install_fails "install when the absent Secret query fails" "could not query Secret agenova-e16-system/e16-mcp-token-absent"
reset ti6; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; seed_secret agenova-e16-system e16-mcp-token-wrong "$TOKEN_F"
install_fails "install with a wrong token equal to the valid one" "Secret agenova-e16-system/e16-mcp-token-wrong holds the valid token"
reset ti7; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"$'\n'
install_fails "install from a fixture token with a trailing newline" "does not hold a valid token; run fixture first"
none_like "$SECRETS"/agenova-e16-system_* || { echo "[fail] an invalid fixture token was copied"; exit 1; }
reset ti8; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'
install_fails "install before the fixture token exists" "does not hold a valid token; run fixture first"
reset ti9; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; FAIL_ON='-n agenova-e16-system get secret e16-mcp-token -o name'
install_fails "install when the copy query fails" "could not query Secret agenova-e16-system/e16-mcp-token"
not_called 'create secret'
reset ti10; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"; OPENSSL_OUT="${GEN_2%?}G"
install_fails "install with an invalid generated wrong token" "could not create Secret agenova-e16-system/e16-mcp-token-wrong from a valid generated token"
[ ! -e "$SECRETS/agenova-e16-system_e16-mcp-token-wrong" ] || { echo "[fail] an invalid wrong token created a Secret"; exit 1; }
echo '[pass] install refuses a differing copy, a wrong token equal to the valid one, an existing or unknown absent Secret, and an invalid token'

# Each access review must give its expected answer; an error is never one.
while IFS='|' read -r setting want archived; do
  reset ti11; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
  eval "$setting"
  expect_fail "install with $setting" "$want" install_stubbed
  grep -qxF "$archived" "$(install_dir)/can-i.txt" || { echo "[fail] the failing access review was not archived: $(cat "$(install_dir)/can-i.txt")"; exit 1; }
done <<'EOF'
CAN_I_CP=no|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:agenova-control-plane answered 'no', want yes|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:agenova-control-plane: no (want yes)
CAN_I_CP=|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:agenova-control-plane answered '', want yes|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:agenova-control-plane:  (want yes)
CAN_I_UNLISTED=yes|can-i get secret/e16-mcp-token-unlisted as system:serviceaccount:agenova-e16-system:agenova-control-plane answered 'yes', want no|can-i get secret/e16-mcp-token-unlisted as system:serviceaccount:agenova-e16-system:agenova-control-plane: yes (want no)
CAN_I_DEFAULT=yes|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:default answered 'yes', want no|can-i get secret/e16-mcp-token as system:serviceaccount:agenova-e16-system:default: yes (want no)
EOF
reset ti12; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'; seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
CAN_I_DEFAULT='no - RBAC: no rule'
install_stubbed >/dev/null
echo '[pass] install archives the control-plane Pod, Role and access reviews, and fails when one gives the wrong answer'

# --- worker captures (Slice 4) ---
WUID=11111111-2222-3333-4444-555555555555
OUID=99999999-2222-3333-4444-555555555555
good_spec='{"automountServiceAccountToken":false,"containers":[{"name":"agent","image":"agenova-testworker:kind","command":["/agenova-workerctl","serve"],"env":[{"name":"MODE","value":"worker"}]}]}'
worker_pod() { # name uid running|waiting spec
  local id=""
  [ "$3" != running ] || id=containerd://c
  printf '{"metadata":{"name":"%s","uid":"%s"},"spec":%s,"status":{"containerStatuses":[{"name":"agent","containerID":"%s","state":{"%s":{}}}]}}' "$1" "$2" "$4" "$id" "$3"
}
pod_list() { local IFS=,; printf '{"apiVersion":"v1","kind":"List","items":[%s]}' "$*"; }
MOUNTS_OK='2900 2800 0:300 / / ro,relatime master:1 - overlay overlay rw,lowerdir=/l
2901 2900 0:301 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
2902 2900 0:302 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2903 2900 254:1 /etc/hosts /etc/hosts rw,relatime - ext4 /dev/vda1 rw'
reset w1; out="$OUTPUT/attempt"; d="$out/worker-pods"; mkdir -p "$out"
WORKER_PODS="$(pod_list "$(worker_pod agenova-pool-x "$WUID" running "$good_spec")" "$(worker_pod agenova-pool-y "$OUID" waiting "$good_spec")")"
WORKER_ENVIRON='PATH=/bin' WORKER_MOUNTINFO="$MOUNTS_OK"
capture_workers "$out"
[ -s "$d/pods-0001.json" ] && [ -s "$d/agenova-pool-x-$WUID.json" ] && [ "$(cat "$d/agenova-pool-x-$WUID.environ")" = 'PATH=/bin' ] &&
  [ "$(cat "$d/agenova-pool-x-$WUID.mountinfo")" = "$MOUNTS_OK" ] || { echo "[fail] a started worker was not captured"; exit 1; }
[ ! -e "$d/agenova-pool-y-$OUID.json" ] && not_called 'exec agenova-pool-y' || { echo "[fail] a worker without a started container was captured"; exit 1; }
[ "$(grep -c ' exec agenova-pool-x -c agent -- cat /proc/1/' "$LOG")" = 2 ] || { echo "[fail] the worker was not read once per file"; exit 1; }
capture_workers "$out"
[ ! -e "$d/pods-0002.json" ] && [ "$(grep -c ' exec ' "$LOG")" = 2 ] || { echo "[fail] an unchanged list was kept again, or a captured UID was read again"; exit 1; }
# Two workers start at once; each read must leave the list of the next.
ZUID=77777777-2222-3333-4444-555555555555
WORKER_PODS="$(pod_list "$(worker_pod agenova-pool-x "$WUID" running "$good_spec")" "$(worker_pod agenova-pool-y "$OUID" running "$good_spec")" "$(worker_pod agenova-pool-z "$ZUID" running "$good_spec")")"
capture_workers "$out"
[ -s "$d/pods-0002.json" ] && [ -s "$d/agenova-pool-y-$OUID.mountinfo" ] && [ -s "$d/agenova-pool-z-$ZUID.mountinfo" ] && [ "$(grep -c ' exec agenova-pool-x ' "$LOG")" = 2 ] ||
  { echo "[fail] a changed list or a newly started worker was not captured"; exit 1; }
reset w2; out="$OUTPUT/attempt"; d="$out/worker-pods"; mkdir -p "$out"
WORKER_PODS="$(pod_list "$(worker_pod agenova-pool-x "$WUID" running "$good_spec")")" WORKER_ENVIRON='PATH=/bin' WORKER_MOUNTINFO="$MOUNTS_OK"
FAIL_ON='cat /proc/1/mountinfo'
capture_workers "$out"
[ -s "$d/agenova-pool-x-$WUID.environ" ] && [ ! -e "$d/agenova-pool-x-$WUID.mountinfo" ] && none_like "$d"/*.partial ||
  { echo "[fail] a failed read left a capture"; exit 1; }
FAIL_ON=''; WORKER_MOUNTINFO=''
capture_workers "$out"
[ ! -e "$d/agenova-pool-x-$WUID.mountinfo" ] && none_like "$d"/*.partial || { echo "[fail] an empty read left a capture"; exit 1; }
WORKER_MOUNTINFO="$MOUNTS_OK"
capture_workers "$out"
[ -s "$d/agenova-pool-x-$WUID.mountinfo" ] && [ "$(grep -c 'cat /proc/1/environ' "$LOG")" = 1 ] || { echo "[fail] a failed read was not retried alone"; exit 1; }
reset w3; out="$OUTPUT/attempt"; mkdir -p "$out"; FAIL_ON='sandbox-template-ref-hash -o json'
if capture_workers "$out"; then echo "[fail] a failed Pod list query counted"; exit 1; fi
none_like "$out"/worker-pods/* || { echo "[fail] a failed Pod list query left a file"; exit 1; }
echo '[pass] the worker watcher keeps each changed Pod list and reads each started Pod once, keeping only complete reads'

# captures <attempt-dir> [spec] [mountinfo] [uid]: the claimed worker's
# captures, as the watcher leaves them; each case below breaks one thing.
captures() {
  local out="$1" spec="${2:-$good_spec}" mounts="${3:-$MOUNTS_OK}" uid="${4:-$WUID}" d="$1/worker-pods"
  rm -rf "${out:?}"; mkdir -p "$d"
  printf '{"state":{"claim":{"backendIdentity":{"workerId":"agenova-pool-x"}}}}' >"$out/work-show.json"
  printf 'agenova-pool-x uid=%s restarts= container= imageID=\nagenova-pool-x uid=%s restarts=0 container=c imageID=good\n' "$WUID" "$WUID" >"$out/worker-identity.txt"
  worker_pod agenova-pool-x "$uid" running "$spec" >"$d/agenova-pool-x-$uid.json"
  printf 'PATH=/bin' >"$d/agenova-pool-x-$uid.environ"
  printf '%s\n' "$mounts" >"$d/agenova-pool-x-$uid.mountinfo"
  pod_list "$(worker_pod agenova-pool-x "$uid" running "$spec")" >"$d/pods-0001.json"
}
reset w4; a="$OUTPUT/a"
captures "$a"
verify_worker_captures "$a/work-show.json" "$a"
spec() { node -e 'const s=JSON.parse(process.argv[1]);'"$1"';process.stdout.write(JSON.stringify(s))' "$good_spec"; }
while IFS='|' read -r change want; do
  captures "$a" "$(spec "$change")"
  expect_fail "worker Pod: $change" "$want" verify_worker_captures "$a/work-show.json" "$a"
done <<'EOF'
s.containers[0].env.push({name:"TOKEN",valueFrom:{secretKeyRef:{name:"e16-mcp-token",key:"token"}}})|containers agent env TOKEN has valueFrom
s.containers[0].env.push({name:"POD",valueFrom:{fieldRef:{fieldPath:"metadata.name"}}})|containers agent env POD has valueFrom
s.containers[0].envFrom=[{secretRef:{name:"e16-mcp-token"}}]|containers agent has envFrom
s.volumes=[{name:"t",secret:{secretName:"e16-mcp-token"}}]|the Pod has volumes
s.containers[0].volumeMounts=[{name:"t",mountPath:"/t"}]|containers agent has volumeMounts
s.automountServiceAccountToken=true|automountServiceAccountToken is not false
delete s.automountServiceAccountToken|automountServiceAccountToken is not false
s.initContainers=[{name:"init",image:"x",volumeMounts:[{name:"t",mountPath:"/t"}]}]|initContainers init has volumeMounts
s.ephemeralContainers=[{name:"debug",image:"x",envFrom:[{secretRef:{name:"x"}}]}]|ephemeralContainers debug has envFrom
s.containers=[]|the started capture has no containers
EOF
# A later list entry for the same UID counts too, for example an ephemeral
# container added after the started capture.
captures "$a"
pod_list "$(worker_pod agenova-pool-x "$WUID" running "$(spec 's.ephemeralContainers=[{name:"debug",image:"x",env:[{name:"T",valueFrom:{secretKeyRef:{name:"x",key:"t"}}}]}]')")" >"$a/worker-pods/pods-0002.json"
expect_fail "a later list entry with a secret env" "pods-0002.json: ephemeralContainers debug env T has valueFrom" verify_worker_captures "$a/work-show.json" "$a"
for mount in '/var/run/secrets/kubernetes.io/serviceaccount' '/run/secrets/kubernetes.io/serviceaccount' '/var/run/secrets'; do
  captures "$a" "" "$MOUNTS_OK
2904 2900 0:304 / $mount ro,relatime - tmpfs tmpfs rw"
  expect_fail "a worker mount at $mount" "mounts under /var/run/secrets: $mount" verify_worker_captures "$a/work-show.json" "$a"
done
captures "$a" "" '2901 2900 0:301 / /proc rw - proc proc rw'
expect_fail "a mount table without a root mount" "has no root mount" verify_worker_captures "$a/work-show.json" "$a"
for ext in json environ mountinfo; do
  captures "$a"; rm "$a/worker-pods/agenova-pool-x-$WUID.$ext"
  expect_fail "a worker without its $ext capture" "worker agenova-pool-x (uid $WUID) has no $ext capture taken while it ran" verify_worker_captures "$a/work-show.json" "$a"
done
captures "$a"; : >"$a/worker-pods/agenova-pool-x-$WUID.environ"
expect_fail "an empty environment capture" "has no environ capture" verify_worker_captures "$a/work-show.json" "$a"
# Captures of another Pod UID never stand in for the running one.
captures "$a" "" "" "$OUID"
expect_fail "captures of another UID" "worker agenova-pool-x (uid $WUID) has no json capture taken while it ran" verify_worker_captures "$a/work-show.json" "$a"
captures "$a"; worker_pod agenova-pool-x "$OUID" running "$good_spec" >"$a/worker-pods/agenova-pool-x-$WUID.json"
expect_fail "a started capture of another Pod under the running UID" "the started capture is another Pod" verify_worker_captures "$a/work-show.json" "$a"
captures "$a"; printf 'agenova-pool-x uid=%s restarts=0 container=c2 imageID=good\n' "$OUID" >>"$a/worker-identity.txt"
expect_fail "a claimed worker that ran as two Pod UIDs" "worker agenova-pool-x ran as more than one Pod UID: $WUID $OUID" verify_worker_captures "$a/work-show.json" "$a"
captures "$a"; printf 'agenova-pool-x uid=%s restarts= container= imageID=\n' "$WUID" >"$a/worker-identity.txt"
expect_fail "a claimed worker never seen started" "worker agenova-pool-x was never captured with a started container" verify_worker_captures "$a/work-show.json" "$a"
captures "$a"; printf 'not json' >"$a/worker-pods/pods-0001.json"
expect_fail "an unreadable Pod list" "unreadable pods-0001.json" verify_worker_captures "$a/work-show.json" "$a"
echo '[pass] the claimed worker needs its started Pod JSON, environment and mount table for its running UID, with no token path in any of them'

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
# Every interop entry is on /mcp, and its receipts carry no Authorization header.
ev() {
  local auth=""
  case "$1" in *'"event":"receipt"'*) auth=',"auth":"missing"' ;; esac
  printf '{"time":"2026-10-03T01:00:00Z",%s,"correlation":"inv-42","path":"/mcp"%s%s}\n' "$1" "$auth" "${2:+,$2}"
}
# The token probe as the fixture logs it: a header-less initialize on
# /mcp-token, refused with 401 before any session.
m2_receipt='{"time":"2026-10-03T01:00:01Z","event":"receipt","httpMethod":"POST","rpcMethod":"initialize","rpcId":"1","correlation":"inv-m2","path":"/mcp-token","auth":"missing"}'
m2_response='{"time":"2026-10-03T01:00:01Z","event":"response","httpMethod":"POST","rpcMethod":"initialize","rpcId":"1","correlation":"inv-m2","path":"/mcp-token","status":401}'
good_m2() { printf '%s\n%s\n' "$m2_receipt" "$m2_response"; }
# The interop test's three calls as the fixture logs them (the shape of a
# real local fixture run): one session each, responses interleaved; then the
# token probe.
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
  good_m2
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
# Slice 4: the token probe and the interop calls' path and auth class.
good_read | grep -v '"correlation":"inv-m2"' >"$OUTPUT/nom2.jsonl"
good_read | sed '/"correlation":"inv-m2"/s/"status":401/"status":200/' >"$OUTPUT/m2status200.jsonl"
good_read | grep -v '"correlation":"inv-m2","path":"/mcp-token","status"' >"$OUTPUT/m2noresponse.jsonl"
{ good_read; good_m2; } >"$OUTPUT/m2twice.jsonl"
good_read | sed '/"correlation":"inv-m2"/s#"path":"/mcp-token"#"path":"/mcp"#' >"$OUTPUT/m2path.jsonl"
good_read | sed '/"correlation":"inv-m2"/s/"auth":"missing"/"auth":"invalid"/' >"$OUTPUT/m2auth.jsonl"
good_read | sed '/"correlation":"inv-m2"/s/"rpcMethod":"initialize"/"rpcMethod":"tools\/call"/' >"$OUTPUT/m2method.jsonl"
good_read | first_only '"auth":"missing"' '"auth":"ok"' >"$OUTPUT/authok.jsonl"
good_read | first_only '"auth":"missing"' '"auth":"invalid"' >"$OUTPUT/authinvalid.jsonl"
good_read | first_only '"path":"/mcp"' '"path":"/mcp-token"' >"$OUTPUT/interoppath.jsonl"
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
nom2|inv-m2 is not exactly one header-less POST initialize on /mcp-token, classed missing and answered 401 (0 entries)
m2status200|inv-m2 is not exactly one header-less POST initialize on /mcp-token, classed missing and answered 401 (2 entries)
m2noresponse|inv-m2 is not exactly one header-less POST initialize on /mcp-token, classed missing and answered 401 (1 entries)
m2twice|inv-m2 is not exactly one header-less POST initialize on /mcp-token, classed missing and answered 401 (4 entries)
m2path|inv-m2 is not exactly one header-less POST initialize on /mcp-token
m2auth|inv-m2 is not exactly one header-less POST initialize on /mcp-token
m2method|inv-m2 is not exactly one header-less POST initialize on /mcp-token
authok|inv-42 receipt with auth ok, want missing
authinvalid|inv-42 receipt with auth invalid, want missing
interoppath|inv-42 receipt entry on path /mcp-token, want /mcp
EOF
echo '[pass] the controlled read must show the three interop calls, each in its own complete session, with exact handler outcomes'
echo '[pass] the interop calls are all on /mcp without a header, and the token probe is one header-less initialize on /mcp-token answered 401'

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
host_go() { printf 'host_go %s\n' "$*" >>"$LOG"; printf -- '--- PASS: %s (0.10s)\n' "$INTEROP_TEST"; good_read >>"$OUTPUT/fixture-follow.jsonl"; }
controlled_read >/dev/null
[ -s "$OUTPUT/controlled-read/fixture-full.jsonl" ] && grep -q 'controlled-read pass' "$OUTPUT/campaign.log"
probe_line="$(grep 'curl .*/mcp-token$' "$LOG")"
[ "$(printf '%s\n' "$probe_line" | grep -c .)" = 1 ] && [[ "$probe_line" == *"-H X-Agenova-Correlation: inv-m2 "* ]] &&
  [[ "$probe_line" == *' http://127.0.0.1:43123/mcp-token' ]] && [[ "$probe_line" == *'"method":"initialize"'* ]] && [[ "$probe_line" != *Authorization* ]] &&
  [ "$(cat "$OUTPUT/controlled-read/token-probe-status.txt")" = 401 ] || { echo "[fail] the token probe was not one header-less initialize on /mcp-token: $probe_line"; exit 1; }
in_order 'host_go test ./internal/adapters/bundled -run' 'curl .*/mcp-token$'
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
for status in 200 500 ''; do
  cr_setup cr9; FIXTURE_LOG="$(good_read)"; TOKEN_PROBE_STATUS="$status"
  expect_fail "a token probe answered '$status'" "the header-less initialize on /mcp-token was not answered 401" controlled_read
  ! grep -q 'controlled-read pass' "$OUTPUT/campaign.log" 2>/dev/null || { echo "[fail] a failed token probe was recorded as a pass"; exit 1; }
  kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
done
cr_setup cr10; FIXTURE_LOG="$(good_read)"; FAIL_ON='/mcp-token'
expect_fail "a token probe that could not connect" "the header-less initialize on /mcp-token was not answered 401" controlled_read
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
cr_setup cr11; good_m2 >"$OUTPUT/fixture-follow.jsonl"
expect_fail "controlled read when the log already holds the token probe" "already holds inv-m2 entries" controlled_read
not_called 'go test'; not_called 'curl'
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
cr_setup cr12; FIXTURE_LOG="$(good_read | grep -v inv-m2)"
host_go() { printf -- '--- PASS: %s (0.10s)\n' "$INTEROP_TEST"; good_read | grep -v inv-m2 >>"$OUTPUT/fixture-follow.jsonl"; }
expect_fail "controlled read whose token probe the collector never saw" "did not capture the controlled read" controlled_read
kill "$fixture_pid"; wait "$fixture_pid" 2>/dev/null || true
echo '[pass] the controlled read sends one header-less initialize to /mcp-token and needs its 401 in the captured log'
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
recorded() { mkdir -p "$(attempt_dir "$1" "$2")"; } # case attempt: a Work attempt on record
# Polling loops need real (short) pauses here.
sleep() { command sleep 0.05; }
no_listeners() {
  ! port_listening "$API_PORT" && ! port_listening "$UI_PORT" || { echo "[fail] $1: a parity port still has a listener"; exit 1; }
}

reset pa1; fake_cli forward; recorded positive 1
parity positive >/dev/null
no_listeners "after a passing parity run"
grep -q "^Forwarding from 127.0.0.1:$API_PORT -> 8081\$" "$OUTPUT/work/positive/attempt-1/parity/api-connect.log"
called 'npx playwright test'
reset pa2; fake_cli forward; recorded positive 1
node -e "$LISTEN_JS" "$API_PORT" >/dev/null 2>&1 & blocker=$!
for _ in $(seq 1 50); do port_listening "$API_PORT" && break; sleep; done
expect_fail "parity while the API port is taken" "port $API_PORT is already in use" parity positive
not_called 'playwright'; [ ! -e "$OUTPUT/work/positive/attempt-1/parity" ]
kill "$blocker"; wait "$blocker" 2>/dev/null || true
for _ in $(seq 1 50); do port_listening "$API_PORT" || break; sleep; done
node -e "$LISTEN_JS" "$UI_PORT" >/dev/null 2>&1 & blocker=$!
for _ in $(seq 1 50); do port_listening "$UI_PORT" && break; sleep; done
expect_fail "parity while the UI port is taken" "port $UI_PORT is already in use" parity positive
kill "$blocker"; wait "$blocker" 2>/dev/null || true
for _ in $(seq 1 50); do port_listening "$UI_PORT" || break; sleep; done
reset pa3; fake_cli forward; recorded positive 1; FAIL_ON='playwright'
expect_fail "parity whose tests fail" "parity failed for positive" parity positive
no_listeners "after a failing parity run"
reset pa4; fake_cli exit; recorded positive 1
expect_fail "parity whose api connect never forwards" "did not forward 127.0.0.1:$API_PORT" parity positive
not_called 'playwright'
no_listeners "after api connect failed"
# SIGTERM while the tests run, after both servers are up, stops them too.
reset pa5; fake_cli forward; recorded positive 1; PLAYWRIGHT_BLOCK=31.7
( parity positive >/dev/null 2>&1 ) & runner=$!
for _ in $(seq 1 100); do called 'playwright test' && port_listening "$UI_PORT" && break; sleep; done
called 'playwright test' && port_listening "$API_PORT" && port_listening "$UI_PORT" || { echo "[fail] parity never reached its tests"; exit 1; }
kill -TERM "$runner"
set +e; wait "$runner"; rc=$?; set -e
[ "$rc" = 143 ] || { echo "[fail] parity stopped by SIGTERM exited $rc, want 143"; exit 1; }
no_listeners "after SIGTERM during the parity tests"
! pgrep -f "sleep 31.7" >/dev/null || { echo "[fail] the interrupted test run was left behind"; exit 1; }
# A passing run restores the traps it replaced.
reset pa6; fake_cli forward; recorded positive 1
trap on_exit EXIT
parity positive >/dev/null
[ "$(trap -p EXIT)" = "trap -- 'on_exit' EXIT" ] && [ -z "$(trap -p TERM)" ] || { echo "[fail] parity did not restore the traps: $(trap -p EXIT TERM)"; exit 1; }
no_listeners "after a passing run with traps"
reset pa7; fake_cli forward; recorded positive 1
expect_fail "parity for an attempt never recorded" "no recorded attempt 2 of positive" parity positive --attempt 2
not_called 'playwright'; [ ! -e "$OUTPUT/work/positive/attempt-2" ]
echo '[pass] parity needs both ports free, uses only its own tunnel and leaves nothing listening on any exit, including SIGTERM and kubectl below api connect'

# --- Work attempts (L9) and archiving after a failed check (L10) ---
# work runs with the real attempt handling, checks after work show, prior
# records and parity (stand-ins as above); cluster-facing checks are stubbed.
# A Work view as run --json and work show print it. The decision ID depends
# only on the request, so two submissions under one name share it; their
# facts differ.
work_view() { # file ref invocation-id
  printf '{"requestRef":"%s","request":{"metadata":{"name":"%s"}},"state":{"decision":{"id":"decision:%s","result":"Allow"}},"facts":[{"id":"fact:%s-1","kind":"ToolDecision","operation":"tool.invoke","invocationId":"%s","timestamp":"2026-10-04T01:00:00Z"},{"id":"fact:%s-2","kind":"ProviderAttempt","operation":"tool.invoke","invocationId":"%s","timestamp":"2026-10-04T01:00:01Z"},{"id":"fact:%s-3","kind":"ProviderOutcome","operation":"tool.invoke","invocationId":"%s","reasonCode":"configured-tool","timestamp":"2026-10-04T01:00:02Z"}],"outcome":{"status":"Succeeded"}}' \
    "$2" "$2" "$2" "$3" "$3" "$3" "$3" "$3" "$3" >"$1"
}
eval "real_$(declare -f capture_workers)"
eval "real_$(declare -f verify_worker_captures)"
stubbed_work() { # work arguments
  (
    require_collector() { :; }; verify_node_tag() { :; }; check_control_plane_identity() { :; }
    pod_identity() { [ -z "${WORKER_LINE:-}" ] || printf '%s\n' "$WORKER_LINE"; }; snapshot_fixture() { :; }
    # CAPTURES=real runs the real worker watcher pass and capture check.
    capture_workers() { [ "$CAPTURES" != real ] || real_capture_workers "$@"; }
    verify_worker_captures() { [ "$CAPTURES" != real ] || real_verify_worker_captures "$@"; }
    verify_claimed_worker() {
      case "$WORKER_FAIL" in
        "") ;;
        signal) sh -c 'kill -INT $$' ;;
        *) fail "worker agenova-pool-x was never captured while the Work ran" ;;
      esac
    }
    work "$@"
  )
}
last_failure() { printf '%s\n' "$1" | grep '^\[fail\] ' | tail -1; }
for args in "--attempt 0" "--attempt 01" "--attempt x" "--attempt" "2" "--attempt 2 extra"; do
  reset wa0
  # shellcheck disable=SC2086
  expect_fail "work positive $args" "attempt" stubbed_work positive $args
  [ ! -s "$LOG" ] && [ ! -e "$OUTPUT/work" ] || { echo "[fail] 'work positive $args' ran commands or created directories"; exit 1; }
done
reset wa1; fake_cli forward
WORK_SHOW="$TMP/view-a1.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-a1; RUN_JSON="$WORK_SHOW"
out="$( (stubbed_work positive) 2>&1 )" || { echo "[fail] attempt 1 of a passing Work failed: $out"; exit 1; }
a1="$OUTPUT/work/positive/attempt-1"
grep -qx '  name: e16-positive-a1' "$a1/work.yaml" && [ "$(diff "$SCRIPT_DIR/work-positive.yaml" "$a1/work.yaml" | grep -c '^[<>]')" = 2 ] ||
  { echo "[fail] attempt 1 was not submitted as e16-positive-a1 with nothing else changed"; exit 1; }
called "run -f $a1/work.yaml" && called "work show e16-positive-a1 --json" && called "evidence work -case positive -ref e16-positive-a1 -view $a1/work-show.json " &&
  called "playwright-env case=positive ref=e16-positive-a1" && [ -s "$a1/parity/playwright.json" ] ||
  { echo "[fail] the attempt's ref did not reach run, work show, the checker and parity"; exit 1; }
[ ! -s "$OUTPUT/prior-at-check.txt" ] && [ "$(cut -d' ' -f1 "$OUTPUT/work-invocations.txt")" = inv-a1 ] && grep -q 'work positive attempt 1 (e16-positive-a1) pass' "$OUTPUT/campaign.log" ||
  { echo "[fail] attempt 1 was not checked and recorded on its own"; exit 1; }
cp -R "$a1" "$TMP/a1-before"; : >"$LOG"
expect_fail "rerunning a recorded attempt" "each attempt is recorded once" stubbed_work positive --attempt 1
not_called 'run -f'
diff -r "$TMP/a1-before" "$a1" >/dev/null || { echo "[fail] a rerun changed a recorded attempt"; exit 1; }
WORK_SHOW="$TMP/view-a2.json"; work_view "$WORK_SHOW" e16-positive-a2 inv-a2; RUN_JSON="$WORK_SHOW"
out="$( (stubbed_work positive --attempt 2) 2>&1 )" || { echo "[fail] attempt 2 of a passing Work failed: $out"; exit 1; }
a2="$OUTPUT/work/positive/attempt-2"
grep -qx '  name: e16-positive-a2' "$a2/work.yaml" && called "run -f $a2/work.yaml" && called "work show e16-positive-a2 --json" &&
  called "evidence work -case positive -ref e16-positive-a2 " && called "playwright-env case=positive ref=e16-positive-a2" && [ -s "$a2/parity/playwright.json" ] ||
  { echo "[fail] attempt 2 did not carry its own name and ref throughout"; exit 1; }
[ "$(cut -d' ' -f1 "$OUTPUT/prior-at-check.txt")" = inv-a1 ] && [ "$(cut -d' ' -f1 "$OUTPUT/work-invocations.txt" | tr '\n' ' ')" = "inv-a1 inv-a2 " ] ||
  { echo "[fail] attempt 2 was not checked against attempt 1's records, or the records were not kept"; exit 1; }
diff -r "$TMP/a1-before" "$a1" >/dev/null || { echo "[fail] attempt 2 changed attempt 1"; exit 1; }
no_listeners "after two attempts"
echo '[pass] each attempt of a Work has its own request name, ref, directory and parity; a recorded attempt is never rerun or changed'

# A failed check after work show still archives the prior records and parity;
# the step fails with the attempt's first failure, reported last.
reset wf1; fake_cli forward; FAIL_ON='evidence work'
WORK_SHOW="$TMP/view-f1.json"; work_view "$WORK_SHOW" e16-n8-truncation-a1 inv-f1; RUN_JSON="$WORK_SHOW"
if out="$( (stubbed_work n8-truncation) 2>&1 )"; then echo "[fail] accepted a Work whose evidence check failed"; exit 1; fi
f1="$OUTPUT/work/n8-truncation/attempt-1"
[[ "$(last_failure "$out")" == "[fail] Work n8-truncation (e16-n8-truncation-a1) does not meet its acceptance; see $f1/evidence.txt" ]] ||
  { echo "[fail] the step did not fail with the evidence check: $out"; exit 1; }
[ -s "$f1/parity/playwright.json" ] && called "playwright-env case=n8-truncation ref=e16-n8-truncation-a1" && grep -q '^inv-f1 ' "$OUTPUT/work-invocations.txt" ||
  { echo "[fail] a failed evidence check stopped parity or the prior records"; exit 1; }
! grep -q 'work n8-truncation .* pass' "$OUTPUT/campaign.log" || { echo "[fail] a failed attempt was recorded as a pass"; exit 1; }
no_listeners "after a failed attempt"
reset wf2; fake_cli forward; FAIL_ON='evidence work'; WORKER_FAIL=1
WORK_SHOW="$TMP/view-f2.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-f2; RUN_JSON="$WORK_SHOW"
if out="$( (stubbed_work positive) 2>&1 )"; then echo "[fail] accepted a Work whose worker was never captured"; exit 1; fi
[ "$(last_failure "$out")" = "[fail] worker agenova-pool-x was never captured while the Work ran" ] && [ -e "$OUTPUT/work/positive/attempt-1/evidence.txt" ] ||
  { echo "[fail] the first failure was not kept, or the later checks did not run: $out"; exit 1; }
[ -s "$OUTPUT/work/positive/attempt-1/parity/playwright.json" ] && grep -q '^inv-f2 ' "$OUTPUT/work-invocations.txt" ||
  { echo "[fail] a failed worker check stopped parity or the prior records"; exit 1; }
reset wf3; fake_cli forward; FAIL_ON='playwright'; WORKER_FAIL=1
WORK_SHOW="$TMP/view-f3.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-f3; RUN_JSON="$WORK_SHOW"
if out="$( (stubbed_work positive) 2>&1 )"; then echo "[fail] accepted a Work whose worker and parity failed"; exit 1; fi
[[ "$out" == *"[fail] CLI, API and Portal parity failed for positive attempt 1"* ]] &&
  [ "$(last_failure "$out")" = "[fail] worker agenova-pool-x was never captured while the Work ran" ] && grep -q '^inv-f3 ' "$OUTPUT/work-invocations.txt" ||
  { echo "[fail] a parity failure replaced the attempt's own failure: $out"; exit 1; }
no_listeners "after a failed attempt and parity"
# A check stopped by a signal stops the runner instead of being archived.
reset wf5; fake_cli forward; WORKER_FAIL=signal
WORK_SHOW="$TMP/view-f5.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-f5; RUN_JSON="$WORK_SHOW"
set +e; stubbed_work positive >/dev/null 2>&1; rc=$?; set -e
[ "$rc" = 130 ] && not_called 'playwright' && [ ! -e "$OUTPUT/work-invocations.txt" ] ||
  { echo "[fail] an interrupted check exited $rc and the runner went on"; exit 1; }
reset wf4; fake_cli forward; FAIL_ON=' work show '
expect_fail "work show fails" "work show failed for e16-positive-a1" stubbed_work positive
not_called 'playwright'; [ ! -e "$OUTPUT/work-invocations.txt" ]
echo '[pass] once work show succeeds, a failed check still archives the prior records and parity, and the step fails with its first failure'

# work show must return the Work this invocation submitted (Codex review of
# the L9 fix). Each refusal stops before any check, record or parity.
not_this_submission() { # description
  expect_fail "$1" "did not return the Work this attempt submitted" stubbed_work "${2:-positive}" ${3:+--attempt "$3"}
  not_called 'evidence work'; not_called 'playwright'
  [ ! -e "$OUTPUT/work-invocations.txt" ] && ! grep -q ' pass$' "$OUTPUT/campaign.log" 2>/dev/null ||
    { echo "[fail] $1: the attempt was archived or recorded as a pass"; exit 1; }
}
# The service already holds a Deny under this attempt's name, for example
# from another output directory: the submission is refused with a conflict and
# prints no evidence, and work show returns the old Deny.
reset ws1; fake_cli forward; FAIL_ON=' run -f '
WORK_SHOW="$TMP/view-ws1.json"; work_view "$WORK_SHOW" e16-admission-deny-a2 inv-old
not_this_submission "a refused submission whose name already has a Deny" admission-deny 2
# Evidence under the same name with other facts is another submission.
reset ws2; fake_cli forward
WORK_SHOW="$TMP/view-ws2.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-old
RUN_JSON="$TMP/run-ws2.json"; work_view "$RUN_JSON" e16-positive-a1 inv-new
not_this_submission "a queried Work with other facts than this submission"
reset ws3; fake_cli forward
WORK_SHOW="$TMP/view-ws3.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-ws3
RUN_JSON="$TMP/run-ws3.json"; node -e 'const fs=require("fs");const v=JSON.parse(fs.readFileSync(process.argv[1]));v.outcome={status:"Failed"};fs.writeFileSync(process.argv[2],JSON.stringify(v))' "$WORK_SHOW" "$RUN_JSON"
not_this_submission "a queried Work with another outcome"
reset ws4; fake_cli forward
WORK_SHOW="$TMP/view-ws4.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-ws4
RUN_JSON="$TMP/run-ws4.json"; node -e 'const fs=require("fs");const v=JSON.parse(fs.readFileSync(process.argv[1]));v.facts=[];fs.writeFileSync(process.argv[2],JSON.stringify(v))' "$WORK_SHOW" "$RUN_JSON"
not_this_submission "submission evidence without facts"
# A legitimate Deny exits 1 and is accepted on its own evidence.
reset ws5; fake_cli forward; FAIL_ON=' run -f '
WORK_SHOW="$TMP/view-ws5.json"; work_view "$WORK_SHOW" e16-admission-deny-a1 inv-ws5; RUN_JSON="$WORK_SHOW"
out="$( (stubbed_work admission-deny) 2>&1 )" || { echo "[fail] a Deny on its own evidence was refused: $out"; exit 1; }
grep -qx 'run-exit 1' "$OUTPUT/work/admission-deny/attempt-1/exit-codes.txt" && grep -q 'work admission-deny attempt 1 (e16-admission-deny-a1) pass' "$OUTPUT/campaign.log" ||
  { echo "[fail] the Deny was not recorded with its nonzero run exit"; exit 1; }
# A submission that returned before the Work finished, and a Work with later
# facts, are still this submission.
reset ws6; fake_cli forward
WORK_SHOW="$TMP/view-ws6.json"; work_view "$WORK_SHOW" e16-positive-a1 inv-ws6
RUN_JSON="$TMP/run-ws6.json"; node -e 'const fs=require("fs");const v=JSON.parse(fs.readFileSync(process.argv[1]));v.facts=v.facts.slice(0,2);delete v.outcome;fs.writeFileSync(process.argv[2],JSON.stringify(v))' "$WORK_SHOW" "$RUN_JSON"
out="$( (stubbed_work positive) 2>&1 )" || { echo "[fail] a submission whose facts lead the queried Work was refused: $out"; exit 1; }
no_listeners "after the submission checks"
echo '[pass] an attempt is judged only on the Work it submitted: a refused submission, other facts or another outcome stop it; a Deny and later facts do not'

# --- Slice 4 Works: the missing token's absence, worker captures, parity ---
reset wt1; fake_cli forward
WORK_SHOW="$TMP/view-wt1.json"; work_view "$WORK_SHOW" e16-token-missing-a1 inv-wt1; RUN_JSON="$WORK_SHOW"
out="$( (stubbed_work token-missing) 2>&1 )" || { echo "[fail] a token-missing attempt with its Secret absent failed: $out"; exit 1; }
t1="$OUTPUT/work/token-missing/attempt-1"
grep -q '^secret agenova-e16-system/e16-mcp-token-absent absent ' "$t1/token-secret-absent.txt" || { echo "[fail] the absence was not archived in the attempt"; exit 1; }
in_order '-n agenova-e16-system get secret e16-mcp-token-absent -o name --ignore-not-found' " run -f $t1/work.yaml"
called 'evidence work -case token-missing -ref e16-token-missing-a1 ' && called 'playwright-env case=token-missing ref=e16-token-missing-a1' && [ -s "$t1/parity/playwright.json" ] ||
  { echo "[fail] the token-missing attempt was not checked or its parity not archived"; exit 1; }
reset wt2; fake_cli forward; seed_secret agenova-e16-system e16-mcp-token-absent "$TOKEN_W"
WORK_SHOW="$TMP/view-wt2.json"; work_view "$WORK_SHOW" e16-token-missing-a1 inv-wt2; RUN_JSON="$WORK_SHOW"
expect_fail "token-missing while its Secret exists" "Secret agenova-e16-system/e16-mcp-token-absent exists; the token-missing case needs it absent" stubbed_work token-missing
not_called ' run -f '; not_called 'playwright'
reset wt3; fake_cli forward; FAIL_ON='get secret e16-mcp-token-absent'
WORK_SHOW="$TMP/view-wt3.json"; work_view "$WORK_SHOW" e16-token-missing-a1 inv-wt3; RUN_JSON="$WORK_SHOW"
expect_fail "token-missing when the Secret query fails" "could not query Secret agenova-e16-system/e16-mcp-token-absent" stubbed_work token-missing
not_called ' run -f '
# The other token cases claim workers and archive parity like the positive one.
for case in token-valid token-wrong; do
  reset "wt-$case"; fake_cli forward
  WORK_SHOW="$TMP/view-$case.json"; work_view "$WORK_SHOW" "e16-$case-a1" "inv-$case"; RUN_JSON="$WORK_SHOW"
  out="$( (stubbed_work "$case") 2>&1 )" || { echo "[fail] a passing $case attempt failed: $out"; exit 1; }
  called "evidence work -case $case -ref e16-$case-a1 " && called "playwright-env case=$case ref=e16-$case-a1" &&
    [ -s "$OUTPUT/work/$case/attempt-1/parity/playwright.json" ] || { echo "[fail] $case was not checked or its parity not archived"; exit 1; }
  not_called 'e16-mcp-token-absent'
done
no_listeners "after the token Works"
echo '[pass] token-missing shows its Secret absent just before submitting; every token case runs its checks and parity'

# The capture check runs through work_check for every Work that claims a
# worker, token cases included: a failure is the attempt's and parity is
# still archived.
claimed_view() { # file ref invocation-id
  work_view "$1" "$2" "$3"
  node -e 'const fs=require("fs");const v=JSON.parse(fs.readFileSync(process.argv[1]));v.state.claim={backendIdentity:{workerId:"agenova-pool-x"}};fs.writeFileSync(process.argv[1],JSON.stringify(v))' "$1"
}
for case in token-valid token-missing token-wrong positive; do
  reset "wc-$case"; fake_cli forward; CAPTURES=real
  WORK_SHOW="$TMP/view-wc-$case.json"; claimed_view "$WORK_SHOW" "e16-$case-a1" "inv-wc-$case"; RUN_JSON="$WORK_SHOW"
  if out="$( (stubbed_work "$case") 2>&1 )"; then echo "[fail] accepted $case without captures of its worker"; exit 1; fi
  [ "$(last_failure "$out")" = "[fail] worker agenova-pool-x was never captured with a started container while the Work ran" ] ||
    { echo "[fail] $case did not fail with the missing capture: $out"; exit 1; }
  [ -s "$OUTPUT/work/$case/attempt-1/parity/playwright.json" ] && grep -q "^inv-wc-$case " "$OUTPUT/work-invocations.txt" &&
    ! grep -q " pass\$" "$OUTPUT/campaign.log" || { echo "[fail] a failed capture check stopped parity or the prior records for $case"; exit 1; }
done
# While the Work runs, the watcher captures the claimed worker from inside.
reset wc1; fake_cli forward; CAPTURES=real
WORKER_LINE="agenova-pool-x uid=$WUID restarts=0 container=c imageID=good"
WORKER_PODS="$(pod_list "$(worker_pod agenova-pool-x "$WUID" running "$good_spec")")" WORKER_ENVIRON='PATH=/bin' WORKER_MOUNTINFO="$MOUNTS_OK"
WAIT_FOR="$OUTPUT/work/token-valid/attempt-1/worker-pods/agenova-pool-x-$WUID.mountinfo"
WORK_SHOW="$TMP/view-wc1.json"; claimed_view "$WORK_SHOW" e16-token-valid-a1 inv-wc1; RUN_JSON="$WORK_SHOW"
out="$( (stubbed_work token-valid) 2>&1 )" || { echo "[fail] a Work whose worker was captured while it ran failed: $out"; exit 1; }
c1="$OUTPUT/work/token-valid/attempt-1/worker-pods"
[ -s "$c1/pods-0001.json" ] && [ -s "$c1/agenova-pool-x-$WUID.json" ] && [ -s "$c1/agenova-pool-x-$WUID.environ" ] ||
  { echo "[fail] the watcher did not capture the worker while the Work ran"; exit 1; }
WORKER_LINE="" WAIT_FOR=""
no_listeners "after the capture checks"
echo '[pass] every Work that claims a worker, token cases included, needs its captures; a missing one fails the attempt after parity'
sleep() { :; }; unset -f npm

# --- final scan (Slice 4) ---
start_stand_ins() {
  command sleep 30 & fixture_pid=$!
  command sleep 30 & cp_pid=$!
  echo "$fixture_pid" >"$OUTPUT/fixture-follow.pid"; echo "$cp_pid" >"$OUTPUT/control-plane-follow.pid"
}
stop_stand_ins() { kill "$fixture_pid" "$cp_pid" 2>/dev/null || true; wait "$fixture_pid" "$cp_pid" 2>/dev/null || true; }
# Live collectors whose logs the final snapshots contain, as after a campaign.
scan_logs() {
  start_stand_ins
  : >"$OUTPUT/fixture-identity-start.txt"
  printf '{"f":1}\n' >"$OUTPUT/fixture-follow.jsonl"; FIXTURE_LOG='{"f":1}'
  printf '{"cp":1}\n{"cp":2}\n' >"$OUTPUT/control-plane-follow.jsonl"; CP_LOG=$'{"cp":1}\n{"cp":2}\n{"cp":3}'
}
scan_setup() { # name
  reset "$1"; write_build testsha; mkdir -p "$STATE_DIR"
  seed_secret agenova-e16 e16-mcp-token "$TOKEN_F"
  seed_secret agenova-e16-system e16-mcp-token "$TOKEN_F"
  seed_secret agenova-e16-system e16-mcp-token-wrong "$TOKEN_W"
  record_secret agenova-e16 e16-mcp-token created
  record_secret agenova-e16-system e16-mcp-token created
  record_secret agenova-e16-system e16-mcp-token-wrong created
  scan_logs
  : >"$LOG"
}
scan_setup s1; mkdir -p "$TMP/extra-root"
scan "$TMP/extra-root" >/dev/null
printf 'valid-fixture %s\nvalid %s\nwrong %s\n' "$TOKEN_F" "$TOKEN_F" "$TOKEN_W" >"$TMP/want-scan"
cmp -s "$TMP/scan-stdin" "$TMP/want-scan" || { echo "[fail] the scan did not get exactly the three token lines on stdin"; exit 1; }
grep -qxF "go build -o $OUTPUT/bin/e16-evidence ./harness/integration/e16/evidence" "$LOG" && grep -qxF "go-env GOTOOLCHAIN=go1.22.12 GOFLAGS=$(host_goflags)" "$LOG" &&
  grep -qxF "$OUTPUT/bin/e16-evidence scan $(cd "$OUTPUT" && pwd -P) $(cd "$STATE_DIR" && pwd -P) $(cd "$TMP/extra-root" && pwd -P)" "$LOG" ||
  { echo "[fail] the scan did not build the checker with host_go or did not search the output, state and extra root"; exit 1; }
in_order '-n agenova-e16 get secret e16-mcp-token -o jsonpath=.*metadata' '-n agenova-e16-system get secret e16-mcp-token-wrong -o jsonpath=.*metadata' \
  'logs deployment/agenova-control-plane' 'logs deployment/e16-mcp' 'go build' '/bin/e16-evidence scan '
[ -s "$OUTPUT/scan/scan.txt" ] && [ -s "$OUTPUT/scan/control-plane-full.jsonl" ] && [ -s "$OUTPUT/scan/fixture-full.jsonl" ] && [ ! -e "$OUTPUT/scan.partial" ] && grep -q 'scan pass' "$OUTPUT/campaign.log" ||
  { echo "[fail] the scan did not keep its output, final logs and record"; exit 1; }
no_token "$TOKEN_F" "$TOKEN_W"
expect_fail "a second scan" "the final scan runs once per campaign" scan
stop_stand_ins
echo '[pass] the scan passes the three tokens on stdin only and searches the output, state and extra roots with the host-built checker'

scan_fails() { # description message
  expect_fail "$1" "$2" scan
  not_called '/bin/e16-evidence scan '
  ! grep -q 'scan pass' "$OUTPUT/campaign.log" 2>/dev/null || { echo "[fail] $1 was recorded as a pass"; exit 1; }
  stop_stand_ins
}
scan_setup s2; printf 'agenova-e16-system/e16-mcp-token-wrong uid=uid-seed-e16-mcp-token-wrong resourceVersion=8' >"$SECRETS/agenova-e16-system_e16-mcp-token-wrong.id"
scan_fails "a Secret updated since it was recorded" "Secret agenova-e16-system/e16-mcp-token-wrong changed since it was recorded"
scan_setup s3; printf 'agenova-e16/e16-mcp-token uid=uid-recreated resourceVersion=7' >"$SECRETS/agenova-e16_e16-mcp-token.id"
scan_fails "a Secret recreated since it was recorded" "Secret agenova-e16/e16-mcp-token changed since it was recorded"
scan_setup s4; rm "$OUTPUT/secrets.txt"
scan_fails "Secrets never recorded" "secrets.txt has no record of Secret agenova-e16/e16-mcp-token"
scan_setup s5; CP_LOG=$'{"cp":1}\n{"cp":3}'
scan_fails "a control-plane snapshot without a line the collector saw" "1 captured log lines are missing from the snapshot $OUTPUT/scan.partial/control-plane-full.jsonl"
scan_setup s6; kill "$cp_pid"; wait "$cp_pid" 2>/dev/null || true
scan_fails "a control-plane collector that stopped" "log collector control-plane is not running"
scan_setup s7; FIXTURE_LOG=''
scan_fails "a fixture snapshot without a line the collector saw" "captured log lines are missing from the snapshot $OUTPUT/scan.partial/fixture-full.jsonl"
for rc in 1 2; do
  scan_setup "s8-$rc"; SCAN_RC="$rc"
  expect_fail "a scan that exits $rc" "the token scan exited $rc" scan
  ! grep -q 'scan pass' "$OUTPUT/campaign.log" 2>/dev/null || { echo "[fail] a scan exiting $rc was recorded as a pass"; exit 1; }
  stop_stand_ins
done
# A token that cannot be read back exactly fails the scan, and it is never
# passed on.
scan_setup s9; printf '%s\n' "$TOKEN_W" >"$SECRETS/agenova-e16-system_e16-mcp-token-wrong"
expect_fail "a token read back with a trailing newline" "could not read the three tokens back from the cluster" scan
! grep -q '^wrong ' "$TMP/scan-stdin" || { echo "[fail] an invalid token was passed to the scan"; exit 1; }
stop_stand_ins
echo '[pass] the scan fails on a changed Secret, a broken log snapshot, an unreadable token or any non-zero checker exit'

# A scan stopped before the search can be repeated; once the checker has
# searched, pass or leak, it cannot. Roots are searched by real path, and an
# extra root must be absolute.
scan_setup s10; FAIL_ON='logs deployment/agenova-control-plane'
expect_fail "a control-plane log that cannot be read" "could not snapshot the control-plane log" scan
[ -d "$OUTPUT/scan.partial" ] && [ ! -e "$OUTPUT/scan" ] || { echo "[fail] a scan stopped before the search left a final scan behind"; exit 1; }
FAIL_ON=''; : >"$LOG"
scan >/dev/null
[ -s "$OUTPUT/scan/scan.txt" ] && [ ! -e "$OUTPUT/scan.partial" ] || { echo "[fail] the repeated scan did not replace its partial preparation"; exit 1; }
stop_stand_ins
scan_setup s11; SCAN_RC=1
expect_fail "a scan that found a token" "the token scan exited 1" scan
SCAN_RC=0
expect_fail "a scan repeated after its search" "the final scan runs once per campaign" scan
stop_stand_ins
scan_setup s12
expect_fail "a relative extra root" "scan root extra is not an absolute path" scan extra
not_called '/bin/e16-evidence scan '
expect_fail "a missing extra root" "is not a readable directory" scan "$TMP/no-such-root"
stop_stand_ins
scan_setup s13; mkdir -p "$TMP/real-state"; ln -s "$TMP/real-state" "$TMP/state-link"; STATE_DIR="$TMP/state-link"
scan >/dev/null
grep -qxF "$OUTPUT/bin/e16-evidence scan $(cd "$OUTPUT" && pwd -P) $(cd "$TMP/real-state" && pwd -P)" "$LOG" ||
  { echo "[fail] a linked state directory was not searched by its real path"; exit 1; }
stop_stand_ins; STATE_DIR="$TMP/s"
echo '[pass] a scan stopped before its search can be repeated, a searched one cannot, and roots are absolute and searched by real path'

# Tracing would print token values, so a traced run stops before any step.
# status is the one local, read-only step, so nothing here reaches a cluster.
for trace in '-x' 'SHELLOPTS'; do
  reset "tr-$trace"; echo 'status-marker' >"$OUTPUT/campaign.log"
  if [ "$trace" = -x ]; then out="$(bash -x "$SCRIPT" --context kind-x --install-namespace agenova-e16-system --state-dir "$TMP/s" --output "$OUTPUT" --model-profile coding-standard status 2>&1)" && rc=0 || rc=$?
  else out="$(env SHELLOPTS=xtrace bash "$SCRIPT" --context kind-x --install-namespace agenova-e16-system --state-dir "$TMP/s" --output "$OUTPUT" --model-profile coding-standard status 2>&1)" && rc=0 || rc=$?; fi
  [ "$rc" -ne 0 ] && [[ "$out" == *"shell tracing is on and would print token values"* ]] && [[ "$out" != *"status-marker"* ]] ||
    { echo "[fail] a run traced with $trace was not refused before its first step: $out"; exit 1; }
done
echo '[pass] a traced run is refused before any step'

# One campaign through fixture, install and scan: no generated token ever
# appears in a command line, campaign.log or any file under the output.
reset flow; write_build testsha; CP_POD='cp-1 uid=1 restarts=0 container=c imageID=good'
fixture_stubbed >/dev/null
install_stubbed >/dev/null
scan_logs
scan >/dev/null
printf 'valid-fixture %s\nvalid %s\nwrong %s\n' "$GEN_1" "$GEN_1" "$GEN_2" >"$TMP/want-scan"
cmp -s "$TMP/scan-stdin" "$TMP/want-scan" || { echo "[fail] the campaign scan did not get the generated tokens"; exit 1; }
[ "$(grep -c ' created ' "$OUTPUT/secrets.txt")" = 3 ] || { echo "[fail] the campaign did not record its three Secrets"; exit 1; }
no_token "$GEN_1" "$GEN_2" "$(printf '%s' "$GEN_1" | base64)" "$(printf '%s' "$GEN_2" | base64)"
stop_stand_ins
echo '[pass] across fixture, install and scan, no token appears in a command line, campaign.log or any output file'

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
COMPLETE=1
