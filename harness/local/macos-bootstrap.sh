#!/usr/bin/env bash
# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLUSTER_NAME="agenova-k8s-lab"
CLI="${ROOT}/.tmp/agenova"
SUBSTRATE="${ROOT}/harness/spike/agent-sandbox-substrate/reproduce.sh"

info() { printf '[info] %s\n' "$*"; }
pass() { printf '[pass] %s\n' "$*"; }
fail() { printf '[fail] %s\n' "$*" >&2; exit 1; }
run() { "$@"; }

usage() {
  cat <<'EOF'
Usage: bash harness/local/macos-bootstrap.sh <doctor|dry-run|setup|verify|down>

  doctor   Check the macOS host and installed prerequisites without mutation.
  dry-run  Print every setup stage without mutating Docker or Kubernetes.
  setup    Prepare kind/Agent Sandbox, build and load images, install Agenova,
           and register the reference Policy and AgentTemplate.
  verify   Run setup, require Ollama and llama3.1:latest, then run sample Work.
  down     Delete only the kind cluster owned by this checkout's substrate receipt.
EOF
}

host_os() { uname -s | tr '[:upper:]' '[:lower:]'; }
host_arch() {
  case "$(uname -m)" in
    arm64|aarch64) printf 'arm64' ;;
    x86_64|amd64) printf 'amd64' ;;
    *) fail "unsupported macOS architecture: $(uname -m)" ;;
  esac
}
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing prerequisite: $1. Install it, then rerun doctor"
}
require_macos() {
  [ "$(host_os)" = darwin ] || fail "this entry point supports macOS only; use the manual reference runbook on $(host_os)"
}
check_commands() {
  local cmd
  for cmd in bash curl docker go kind kubectl pwsh; do require_cmd "$cmd"; done
}
check_docker() {
  run docker info >/dev/null 2>&1 || fail "Docker daemon is unreachable; start Docker Desktop and rerun doctor"
}
check_ollama() {
  require_cmd ollama
  run ollama list >/dev/null 2>&1 || fail "Ollama is unreachable; run 'ollama serve' and ensure llama3.1:latest is installed"
  run ollama show llama3.1:latest >/dev/null 2>&1 || fail "missing Ollama model llama3.1:latest; run 'ollama pull llama3.1:latest'"
}

doctor() {
  require_macos
  host_arch >/dev/null
  check_commands
  check_docker
  info "host: macOS/$(host_arch)"
  run docker version --format 'docker client={{.Client.Version}} server={{.Server.Version}}'
  run kind version
  run kubectl version --client=true
  run go version
  run pwsh --version
  if command -v ollama >/dev/null 2>&1 && run ollama list >/dev/null 2>&1; then
    pass "Ollama reachable (required only by verify)"
  else
    info "Ollama not reachable; setup remains available, verify will fail with remediation"
  fi
  pass "macOS bootstrap prerequisites are ready"
}

print_plan() {
  cat <<EOF
[plan] bash ${SUBSTRATE#${ROOT}/} up
[plan] docker build worker image agenova-testworker:kind
[plan] docker build control-plane image agenova-control-plane:0.1.0
[plan] kind load both images into ${CLUSTER_NAME}
[plan] go build ${CLI#${ROOT}/}
[plan] agenova platform validate, plan, apply --yes, status
[plan] agenova policy apply and agent-template apply
[plan] verification Work runs only with the verify subcommand
EOF
}

setup() {
  doctor
  run bash "$SUBSTRATE" up
  run docker build -f "$ROOT/harness/integration/agentsandbox/testworker/Dockerfile" -t agenova-testworker:kind "$ROOT"
  run docker build -f "$ROOT/deploy/reference/Dockerfile" -t agenova-control-plane:0.1.0 "$ROOT"
  run kind load docker-image agenova-testworker:kind --name "$CLUSTER_NAME"
  run kind load docker-image agenova-control-plane:0.1.0 --name "$CLUSTER_NAME"
  run mkdir -p "$ROOT/.tmp"
  run go build -o "$CLI" ./cmd/agenova
  run "$CLI" platform validate -f "$ROOT/deploy/reference/platform.kind.yaml"
  run "$CLI" platform plan -f "$ROOT/deploy/reference/platform.kind.yaml"
  run "$CLI" platform apply -f "$ROOT/deploy/reference/platform.kind.yaml" --yes
  run "$CLI" platform status
  run "$CLI" policy apply -f "$ROOT/deploy/reference/demo/policy.yaml"
  run "$CLI" agent-template apply -f "$ROOT/deploy/reference/demo/engineer.yaml"
  pass "reference Platform, Policy, and AgentTemplate are ready"
  info "run '$0 verify' for the real Ollama Work, or '$0 down' for owned-cluster cleanup"
}

verify() {
  check_ollama
  setup
  run "$CLI" run -f "$ROOT/deploy/reference/demo/work.yaml"
  pass "reference Ollama Work completed"
}

main() {
  [ "$#" -eq 1 ] || { usage; exit 2; }
  cd "$ROOT"
  case "$1" in
    doctor) doctor ;;
    dry-run) require_macos; host_arch >/dev/null; print_plan ;;
    setup) setup ;;
    verify) verify ;;
    down) require_macos; run bash "$SUBSTRATE" down ;;
    -h|--help) usage ;;
    *) usage; fail "unknown subcommand: $1" ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
