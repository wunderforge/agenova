#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Isolated command doubles: no Docker, Kubernetes, downloads or user kubeconfig.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${HERE}/reproduce.sh"
mkdir -p "${ROOT}/.tmp"
TEST_ROOT="$(mktemp -d "${ROOT}/.tmp/substrate-tests.XXXXXX")"

fake_kind() {
  case "$*" in
    'get clusters')
      [ "${LIST_ERROR:-false}" = false ] || return 1
      if [ -f "${CASE_DIR}/cluster" ]; then printf '%s\n' "$CLUSTER_NAME"; fi ;;
    'create cluster '*)
      printf 'kind create\n' >> "${CASE_DIR}/mutations"
      touch "${CASE_DIR}/cluster" ;;
    'delete cluster '*)
      printf 'kind delete\n' >> "${CASE_DIR}/mutations"
      rm "${CASE_DIR}/cluster" ;;
    version) printf 'kind fake\n' ;;
    *) return 1 ;;
  esac
}
docker() {
  case "$1" in
    info) [ "${DOCKER_ERROR:-false}" = false ] ;;
    inspect) printf '%s\n' "${NODE_ID:-node-original}" ;;
    *) return 1 ;;
  esac
}
curl() {
  printf '%s\n' "$*" >> "${CASE_DIR}/downloads"
  local dest=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then dest="$2"; shift; fi
    shift
  done
  if [ -n "$dest" ]; then printf 'fixture\n' > "$dest";
  else sha256_of "${CASE_DIR}/fixture"; fi
}
sleep() { :; }
fake_kubectl() {
  if [ "$1" = --context ]; then
    [ "$2" = "$CONTEXT" ] || return 90
    shift 2
  fi
  local args="$*"
  case "$args" in
    'config current-context') printf '%s\n' "${ACTIVE_CONTEXT:-$CONTEXT}" ;;
    'get namespace kube-system '*) printf 'cluster-uid\n' ;;
    "get namespace $SMOKE_NAMESPACE --ignore-not-found -o name")
      [ "${NS_READ_ERROR:-false}" = false ] || return 1
      if [ -f "${CASE_DIR}/namespace" ]; then printf 'namespace/%s\n' "$SMOKE_NAMESPACE"; fi ;;
    "get namespace $SMOKE_NAMESPACE -o jsonpath="*)
      cat "${CASE_DIR}/namespace" ;;
    'create -f -')
      [ ! -f "${CASE_DIR}/namespace" ] || return 1
      local manifest
      manifest="$(cat)"
      [[ "$manifest" == *'agenova.io/substrate-owner: cluster-uid'* ]] || return 1
      printf 'namespace create\n' >> "${CASE_DIR}/mutations"
      printf 'cluster-uid\n' > "${CASE_DIR}/namespace" ;;
    'apply -f '*|*' delete sandbox'* ) printf '%s\n' "$args" >> "${CASE_DIR}/mutations" ;;
    "delete namespace $SMOKE_NAMESPACE "*)
      printf 'namespace delete\n' >> "${CASE_DIR}/mutations"
      [ "${DELETE_TIMEOUT:-false}" = false ] || return 1
      rm "${CASE_DIR}/namespace" ;;
    *' get pods -o name')
      [ "${POD_READ_ERROR:-false}" = false ] || return 1
      if [ "${PODS_REMAIN:-false}" = true ]; then printf 'pod/stale\n'; fi ;;
    *'get sandboxclaim smoke-claim -o jsonpath='*) printf '%s\n' "${READY:-True}" ;;
    'get crd '*jsonpath*) printf 'v1alpha1 served=true storage=true\n' ;;
    version*|'get crd '*|*'rollout status '*|*' get '*|*' logs '*) printf 'fake observation\n' ;;
    *) printf 'unhandled fake kubectl: %s\n' "$args" >&2; return 1 ;;
  esac
}
setup() {
  CASE_DIR="${TEST_ROOT}/$1"
  mkdir -p "${CASE_DIR}/state"
  STATE_DIR="${CASE_DIR}/state"
  OWNER_FILE="${STATE_DIR}/identity"
  DOWNLOAD_DIR="${CASE_DIR}/manifests"
  TOOLS_DIR="${CASE_DIR}/tools"
  : > "${CASE_DIR}/mutations"
  : > "${CASE_DIR}/downloads"
  printf 'fixture\n' > "${CASE_DIR}/fixture"
  resolve_tools() { KIND_BIN=fake_kind; KUBECTL_BIN=fake_kubectl; KIND_SOURCE=mock; KUBECTL_SOURCE=mock; }
}
owned() {
  touch "${CASE_DIR}/cluster"
  printf 'node-original\ncluster-uid\n' > "${OWNER_FILE}"
}
owned_namespace() { owned; printf 'cluster-uid\n' > "${CASE_DIR}/namespace"; }
expect_failure() {
  local pattern="$1"; shift
  if ( "$@" ) > "${CASE_DIR}/failure.txt" 2>&1; then
    printf 'unexpected success: %s\n' "$*"; exit 1
  fi
  grep -q "$pattern" "${CASE_DIR}/failure.txt"
}
no_mutations() { [ ! -s "${CASE_DIR}/mutations" ]; }
run_case() {
  local name="$1"
  ( setup "$name"; "$name" ) > "${TEST_ROOT}/${name}.txt" 2>&1
  printf '[pass] %s\n' "$name"
}

foreign_cluster() {
  touch "${CASE_DIR}/cluster"
  expect_failure 'no ownership receipt' cmd_up
  expect_failure 'no ownership receipt' cmd_down
  no_mutations
}
replaced_cluster() {
  owned; NODE_ID=node-replacement
  expect_failure 'fingerprint changed' cmd_up
  expect_failure 'fingerprint changed' cmd_down
  no_mutations
}
wrong_context() {
  owned_namespace; ACTIVE_CONTEXT=unrelated-context
  for phase in cmd_up cmd_smoke cmd_teardown cmd_down; do
    expect_failure 'resolved kube context' "$phase"
  done
  no_mutations
}
foreign_namespace() {
  owned_namespace; printf 'someone-else\n' > "${CASE_DIR}/namespace"
  expect_failure 'not owned' cmd_smoke
  expect_failure 'not owned' cmd_teardown
  no_mutations
}
namespace_read_failure() {
  owned; NS_READ_ERROR=true
  expect_failure 'could not inspect smoke namespace' cmd_smoke
  expect_failure 'could not inspect smoke namespace' cmd_teardown
  no_mutations
}
cluster_list_failure() {
  LIST_ERROR=true
  expect_failure 'could not enumerate' cmd_up
  expect_failure 'could not enumerate' cmd_down
  no_mutations
}
docker_unavailable() {
  DOCKER_ERROR=true
  expect_failure 'docker daemon is not reachable' cmd_up
  no_mutations; [ ! -s "${CASE_DIR}/downloads" ]
}
readiness_timeout() {
  owned; READY=False
  expect_failure 'never reached Ready=True' cmd_smoke
  ! grep -q 'delete' "${CASE_DIR}/mutations"
}
pod_read_failure() {
  owned_namespace; POD_READ_ERROR=true
  expect_failure 'cannot observe pod cleanup' cmd_teardown
  ! grep -q 'namespace delete' "${CASE_DIR}/mutations"
}
pod_cleanup_timeout() {
  owned_namespace; PODS_REMAIN=true
  expect_failure 'did not terminate' cmd_teardown
  ! grep -q 'namespace delete' "${CASE_DIR}/mutations"
}
namespace_cleanup_timeout() {
  owned_namespace; DELETE_TIMEOUT=true
  expect_failure '' cmd_teardown
  [ -f "${CASE_DIR}/namespace" ]
}
owned_reruns() {
  main all
  main all
  [ ! -f "${CASE_DIR}/cluster" ] && [ ! -f "${CASE_DIR}/namespace" ]
  [ ! -f "${OWNER_FILE}" ]
  [ "$(grep -c '^kind create$' "${CASE_DIR}/mutations")" = 2 ]
  [ "$(grep -c '^kind delete$' "${CASE_DIR}/mutations")" = 2 ]
}
owned_reentry() {
  owned_namespace
  cmd_up; cmd_smoke; cmd_smoke; cmd_teardown; cmd_teardown; cmd_down; cmd_down
  ! grep -q '^kind create$' "${CASE_DIR}/mutations"
}
kind_requires_package_manager() {
  # kind is never downloaded: a missing kind is a loud prerequisite failure
  # pointing at the supported package managers, on every platform.
  source "${HERE}/reproduce.sh"
  TOOLS_DIR="${CASE_DIR}/tools"; DOWNLOAD_DIR="${CASE_DIR}/manifests"
  command() { if [ "${2:-}" = kind ]; then return 1; fi; builtin command "$@"; }
  expect_failure 'package manager' require_kind
  expect_failure 'package manager' cmd_up
  expect_failure 'no auto-install rule for kind' download_pinned_tool kind v0.33.0
  [ ! -s "${CASE_DIR}/downloads" ]
}
diagnostics_and_scope() {
  owned
  local evidence="${ROOT}/docs/evidence/E8-T3/agent-sandbox-substrate"
  local before
  before="$(sha256_of "${evidence}/summary.md") $(sha256_of "${evidence}/output.txt")"
  main tools; main status
  [ "$before" = "$(sha256_of "${evidence}/summary.md") $(sha256_of "${evidence}/output.txt")" ]
  [ ! -s "${CASE_DIR}/downloads" ]; no_mutations
  expect_failure 'unknown subcommand: compare' main compare
  expect_failure 'only available for mutating' main tools --capture
  # Restore the real resolver and make absence deterministic using command().
  source "${HERE}/reproduce.sh"
  command() { if [ "${2:-}" = kind ]; then return 1; fi; builtin command "$@"; }
  expect_failure 'package manager' cmd_tools
  [ ! -s "${CASE_DIR}/downloads" ]
}
for scenario in foreign_cluster replaced_cluster wrong_context foreign_namespace \
  namespace_read_failure cluster_list_failure docker_unavailable readiness_timeout \
  pod_read_failure pod_cleanup_timeout namespace_cleanup_timeout owned_reruns \
  owned_reentry kind_requires_package_manager diagnostics_and_scope; do
  run_case "$scenario"
done
printf '[pass] 15 isolated regression scenarios (command doubles only)\n'
printf '[info] Logs: %s\n' "$TEST_ROOT"
