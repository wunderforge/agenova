#!/usr/bin/env bash
# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=macos-bootstrap.sh
source "$ROOT/harness/local/macos-bootstrap.sh"

LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT
host_os() { printf darwin; }
host_arch() { printf arm64; }
require_cmd() { :; }
run() { printf '%s\n' "$*" >>"$LOG"; }

assert_contains() { grep -F "$1" "$LOG" >/dev/null || { echo "missing command: $1"; exit 1; }; }
assert_absent() { ! grep -E "$1" "$LOG" >/dev/null || { echo "unexpected mutation: $1"; exit 1; }; }

: >"$LOG"
main dry-run >/dev/null
assert_absent 'docker build|kind load|kubectl (apply|create|delete)|platform apply'
echo '[pass] dry-run emits no mutating command'

: >"$LOG"
check_docker() { run docker info; }
doctor >/dev/null
assert_contains 'docker info'
assert_absent 'docker build|kind load|kubectl (apply|create|delete)|platform apply'
echo '[pass] doctor performs read-only checks'

: >"$LOG"
doctor() { run doctor; }
setup >/dev/null
assert_contains 'doctor'
assert_contains "bash $SUBSTRATE up"
assert_contains 'docker build'
assert_contains 'kind load docker-image agenova-testworker:kind'
assert_contains "$CLI platform apply"
assert_contains "$CLI policy apply"
assert_contains "$CLI agent-template apply"
first="$(grep -n "bash $SUBSTRATE up" "$LOG" | cut -d: -f1)"
apply="$(grep -n "$CLI platform apply" "$LOG" | cut -d: -f1)"
[ "$first" -lt "$apply" ] || { echo 'substrate must precede Platform apply'; exit 1; }
echo '[pass] setup preserves substrate-to-declarative-install order'

: >"$LOG"
check_ollama() { run ollama-ready; }
setup() { run setup; }
verify >/dev/null
assert_contains 'ollama-ready'
assert_contains 'setup'
assert_contains "$CLI run"
echo '[pass] verification is explicit and follows setup'

: >"$LOG"
fail() { printf '%s\n' "$*" >>"$LOG"; return 1; }
host_os() { printf linux; }
if require_macos; then echo 'unsupported host unexpectedly accepted'; exit 1; fi
assert_contains 'supports macOS only'
echo '[pass] unsupported hosts fail before mutation'

grep -F '& $pwsh -NoProfile -Command' "$ROOT/scripts/evidence.ps1" >/dev/null
! grep -E '^[[:space:]]*powershell -NoProfile' "$ROOT/scripts/evidence.ps1" >/dev/null
echo '[pass] evidence capture reuses the cross-platform PowerShell host'
