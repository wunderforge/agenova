#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# Offline checks for campaign.sh: argument guards, protect/load/restore gates
# and probe output validation. No command reaches Docker, kind or a cluster.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT="$ROOT/harness/integration/e16/campaign.sh"
# shellcheck source=campaign.sh
source "$SCRIPT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
LOG="$TMP/calls.log"
HEX="$(printf 'a%.0s' $(seq 1 64))"
RECORDED="sha256:$HEX"

# A fake image archive whose manifest names the recorded config.
mkdir -p "$TMP/fake"
printf '[{"Config":"blobs/sha256/%s"}]' "$HEX" >"$TMP/fake/manifest.json"
tar -cf "$TMP/fake.tar" -C "$TMP/fake" manifest.json

# Stubs: every external command is logged, nothing runs.
FAIL_ON=""
run() {
  printf '%s\n' "$*" >>"$LOG"
  case "$*" in
    *"cat /tmp/e16-protect.tar"*) cat "$TMP/fake.tar" ;;
    *"kind load image-archive"*) touch "$TMP/imported" ;;
    # Any local listener answers with valid, empty Work JSON.
    *"curl "*) printf '[]' ;;
  esac
  if [ -n "$FAIL_ON" ] && [[ "$*" == *"$FAIL_ON"* ]]; then return 1; fi
  return 0
}
git() { case "$*" in *"status --porcelain"*) : ;; *"rev-parse --short=12 HEAD"*) echo testsha12345 ;; *"rev-parse HEAD"*) echo testsha ;; esac; }
sleep() { :; }
OTHER_REPLICAS=1
other_installs() { printf 'Deployment agenova-system agenova-control-plane %s\n' "$OTHER_REPLICAS"; }
OTHER_PODS="" E16_PODS=""
fixed_tag_pods() { if [ "$1" = others ]; then printf '%s' "$OTHER_PODS"; else printf '%s' "$E16_PODS"; fi; }
WORK_JSON='[]' ARCHIVE_RC=0
eval "real_$(declare -f archive_other_work)"
archive_other_work() { printf '%s' "$WORK_JSON" >"$3"; return "$ARCHIVE_RC"; }
NODE_ID="$RECORDED"
# After an import the node reports the recorded image again.
node_image_id() { if [ -e "$TMP/imported" ]; then printf '%s' "$RECORDED"; else printf '%s' "$NODE_ID"; fi; }
wait_ready() { return 0; }

reset() {
  OUTPUT="$TMP/out-$1"; rm -rf "$OUTPUT" "$TMP/imported"; mkdir -p "$OUTPUT"; : >"$LOG"
  CONTEXT=kind-x INSTALL_NAMESPACE=agenova-e16-system STATE_DIR="$TMP/s" YES=1
  OTHER_REPLICAS=1 OTHER_PODS="" E16_PODS="" WORK_JSON='[]' ARCHIVE_RC=0 NODE_ID="$RECORDED" FAIL_ON=""
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

for args in \
  "preflight" \
  "--context kind-x --install-namespace e16 --state-dir $TMP/s preflight" \
  "--context minikube --install-namespace e16 --state-dir $TMP/s --output $TMP/o preflight" \
  "--context kind-x --install-namespace default --state-dir $TMP/s --output $TMP/o preflight" \
  "--context kind-x --install-namespace agenova-e16 --state-dir $TMP/s --output $TMP/o preflight" \
  "--context '  ' --install-namespace e16 --state-dir $TMP/s --output $TMP/o preflight"; do
  # shellcheck disable=SC2086
  if eval bash "$SCRIPT" $args >/dev/null 2>&1; then echo "[fail] accepted: $args"; exit 1; fi
done
echo '[pass] context, namespace, state and output arguments are required and checked'

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
expect_fail "protect twice" "run restore before protecting again" protect
echo '[pass] protect archives history, verifies exports, stops the old install and refuses to repeat'

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
write_build() { # source
  mkdir -p "$OUTPUT/images"
  printf 'source %s\n' "$1" >"$OUTPUT/build-identity.txt"
  local tag
  for tag in agenova-control-plane:0.1.0 agenova-testworker:kind agenova-e16-mcp:testsha12345 agenova-e16-probe:testsha12345; do
    cp "$TMP/fake.tar" "$(image_archive "$tag")"
    printf 'x tag=%s local-id=y config=%s archive-sha256=%s\n' "$tag" "$RECORDED" "$(shasum -a 256 "$TMP/fake.tar" | cut -d' ' -f1)" >>"$OUTPUT/build-identity.txt"
  done
}
protected_state() { printf 'image agenova-control-plane:0.1.0 %s x\ncomplete now\n' "$RECORDED" >"$OUTPUT/protect-state.txt"; }

reset l1; write_build testsha; printf 'image agenova-control-plane:0.1.0 %s x\n' "$RECORDED" >"$OUTPUT/protect-state.txt"
expect_fail "load after an incomplete protect" "missing or incomplete" load; not_called 'kind load'
reset l2; write_build testsha; protected_state
expect_fail "load while the old Deployment is scaled up" "protect no longer holds" load; not_called 'kind load'
reset l3; write_build testsha; protected_state; OTHER_REPLICAS=0; OTHER_PODS='agenova-system/old Running x'
expect_fail "load while old pods still run" "still run the fixed tags" load; not_called 'kind load'
echo '[pass] load requires a complete protect whose effect still holds'

reset l4; write_build testsha; protected_state; OTHER_REPLICAS=0
printf 'tampered' >>"$(image_archive agenova-testworker:kind)"
expect_fail "load a replaced archive" "does not match its recorded hash" load; not_called 'kind load'
reset l5; write_build othersha; protected_state; OTHER_REPLICAS=0
expect_fail "load archives built from another commit" "another commit" load; not_called 'kind load'
echo '[pass] load refuses archives that do not match the build record for this commit'

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

printf '=== RUN   TestE16Probes\n    skipped\n--- SKIP: TestE16Probes (0.00s)\nPASS\n' >"$TMP/skip.log"
expect_fail "skipped probe run" "did not pass" validate_probe_output "$TMP/skip.log"
printf '=== RUN   TestE16Probes\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/empty.log"
expect_fail "probe run without receipts" "no receipts" validate_probe_output "$TMP/empty.log"
printf 'E16_PROBE {}\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/ok.log"
validate_probe_output "$TMP/ok.log"
echo '[pass] a probe Job that skipped or emitted nothing is rejected even with exit 0'
