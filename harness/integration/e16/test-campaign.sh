#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# Offline checks for campaign.sh: argument guards, mutation gates and probe
# output validation. No command reaches Docker, kind or a cluster.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT="$ROOT/harness/integration/e16/campaign.sh"
# shellcheck source=campaign.sh
source "$SCRIPT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
LOG="$TMP/calls.log"
run() { printf '%s\n' "$*" >>"$LOG"; }
other_installs() { printf 'Deployment agenova-system agenova-control-plane 1\n'; }
node_image_id() { printf 'sha256:recorded'; }

expect_fail() { # description command...
  local description="$1"; shift
  if ( "$@" ) >/dev/null 2>&1; then echo "[fail] accepted: $description"; exit 1; fi
}
assert_no_mutation() {
  ! grep -E 'scale|patch|ctr -n k8s.io images export|kind load|apply|docker build' "$LOG" >/dev/null ||
    { echo "[fail] unexpected mutation: $(cat "$LOG")"; exit 1; }
}

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

CONTEXT=kind-x INSTALL_NAMESPACE=agenova-e16-system STATE_DIR="$TMP/s" OUTPUT="$TMP/o" YES=0
mkdir -p "$OUTPUT"
: >"$LOG"
expect_fail "protect without --yes" protect
assert_no_mutation
echo '[pass] protect changes nothing without --yes'

: >"$LOG"
expect_fail "load before protect while another install shares the tags" load
assert_no_mutation
echo '[pass] load refuses while an unprotected install shares the fixed tags'

: >"$LOG"
YES=1
protect >/dev/null
grep -q '^image agenova-control-plane:0.1.0 sha256:recorded ' "$OUTPUT/protect-state.txt"
grep -q '^scale Deployment agenova-system agenova-control-plane 1$' "$OUTPUT/protect-state.txt"
# The state is written before the first mutation, so restore always has it.
first_mutation="$(grep -n -E 'ctr -n k8s.io|scale' "$LOG" | head -1 | cut -d: -f1)"
[ -n "$first_mutation" ] || { echo '[fail] protect did not export or scale'; exit 1; }
grep -q 'scale deployment agenova-control-plane --replicas=0' "$LOG"
expect_fail "protect twice" protect
echo '[pass] protect records state first and refuses to overwrite it'

: >"$LOG"
node_image_id() { printf 'sha256:other'; }
expect_fail "restore with a mismatched image" restore
node_image_id() { printf 'sha256:recorded'; }
: >"$LOG"
restore >/dev/null
grep -q 'kind load image-archive' "$LOG"
grep -q 'scale deployment agenova-control-plane --replicas=1' "$LOG"
[ ! -e "$OUTPUT/protect-state.txt" ]
restore >/dev/null
echo '[pass] restore verifies image IDs, restores replicas and is idempotent'

printf '=== RUN   TestE16Probes\n    skipped\n--- SKIP: TestE16Probes (0.00s)\nPASS\n' >"$TMP/skip.log"
expect_fail "skipped probe run" validate_probe_output "$TMP/skip.log"
printf '=== RUN   TestE16Probes\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/empty.log"
expect_fail "probe run without receipts" validate_probe_output "$TMP/empty.log"
printf 'E16_PROBE {}\n--- PASS: TestE16Probes (1.00s)\nPASS\n' >"$TMP/ok.log"
validate_probe_output "$TMP/ok.log"
echo '[pass] a probe Job that skipped or emitted nothing is rejected even with exit 0'
