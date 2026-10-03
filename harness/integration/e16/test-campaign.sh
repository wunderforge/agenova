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
    *"logs deployment/e16-mcp"*) printf '%s' "${FIXTURE_LOG:-}" ;;
    *"get deployment agenova-control-plane -o name"*) [ -z "${E16_DEPLOYED:-}" ] || echo deployment.apps/agenova-control-plane ;;
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
  OTHER_REPLICAS=1 OTHER_NONE="" OTHER_FAIL="" PODS_FAIL="" OTHER_PODS="" E16_PODS="" WORK_JSON='[]' ARCHIVE_RC=0 NODE_ID="$RECORDED" FAIL_ON=""
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
echo '[pass] each Work needs live collectors and its claimed worker on the recorded image'

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

reset c5
printf '{"stats":{"expected":2,"unexpected":0,"skipped":0,"flaky":0}}' >"$OUTPUT/r.json"; parity_report_ok "$OUTPUT/r.json"
for bad in '{"stats":{"expected":1,"unexpected":0,"skipped":1,"flaky":0}}' '{"stats":{"expected":6,"unexpected":0,"skipped":0,"flaky":0}}' '{"stats":{"expected":2,"unexpected":1,"skipped":0,"flaky":0}}' 'not json'; do
  printf '%s' "$bad" >"$OUTPUT/r.json"
  if parity_report_ok "$OUTPUT/r.json"; then echo "[fail] accepted parity report: $bad"; exit 1; fi
done
echo '[pass] parity counts only a run of exactly the setup and case tests'

printf '=== RUN   TestE16Probes\n    skipped\n--- SKIP: TestE16Probes (0.00s)\nPASS\n' >"$TMP/skip.log"
expect_fail "skipped probe run" "did not pass" validate_probe_output "$TMP/skip.log"
printf '=== RUN   TestE16Probes\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/empty.log"
expect_fail "probe run without receipts" "no receipts" validate_probe_output "$TMP/empty.log"
printf 'E16_PROBE {}\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/ok.log"
validate_probe_output "$TMP/ok.log"
echo '[pass] a probe Job that skipped or emitted nothing is rejected even with exit 0'
