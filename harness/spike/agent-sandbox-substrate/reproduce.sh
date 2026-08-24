#!/usr/bin/env bash
# E8-T3 (#50): reproduce the upstream Agent Sandbox test substrate on a
# disposable local kind cluster. This script never touches Agenova claim
# types, the RuntimeBackend contract, or internal/runtime/agentsandbox — it
# proves the pinned upstream lifecycle only. See RUNBOOK.md for usage.
set -euo pipefail

CLUSTER_NAME="agenova-k8s-lab"
CONTEXT="kind-${CLUSTER_NAME}"
AGENT_SANDBOX_VERSION="v0.4.6"
CONTROLLER_NAMESPACE="agent-sandbox-system"
CONTROLLER_DEPLOY="agent-sandbox-controller"
SMOKE_NAMESPACE="agent-sandbox-smoke"
SMOKE_TIMEOUT_SECONDS=90

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
MANIFEST_DIR="${SCRIPT_DIR}/manifests"
DOWNLOAD_DIR="${ROOT}/.tmp/agent-sandbox-${AGENT_SANDBOX_VERSION}"
EVIDENCE_DIR="${ROOT}/docs/evidence/E8-T3/agent-sandbox-substrate"
OUTPUT_FILE="${EVIDENCE_DIR}/output.txt"
SUMMARY_FILE="${EVIDENCE_DIR}/summary.md"

RELEASE_BASE="https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${AGENT_SANDBOX_VERSION}"

pass() { printf '[pass] %s\n' "$1"; }
fail() { printf '[fail] %s\n' "$1" >&2; exit 1; }
info() { printf '[info] %s\n' "$1"; }

usage() {
  cat <<'EOF'
Usage: reproduce.sh <status|up|smoke|down|all>

  status  Read-only: report kube context, cluster existence, CRDs, controller readiness.
  up      Create the kind cluster (if missing) and install pinned Agent Sandbox.
  smoke   Create/observe/terminate/clean up one minimal upstream SandboxClaim.
  down    Delete the kind cluster created by this script. Nothing else is touched.
  all     up + smoke + down (default).
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing prerequisite: $1 not found on PATH"
}

check_prereqs() {
  require_cmd kind
  require_cmd kubectl
  require_cmd curl
  docker info >/dev/null 2>&1 || fail "docker daemon is not reachable (start Docker Desktop and retry)"
  pass "prerequisites present: kind, kubectl, curl, docker daemon reachable"
}

cluster_exists() {
  kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"
}

current_context() {
  kubectl config current-context 2>/dev/null || true
}

require_context() {
  local ctx
  ctx="$(current_context)"
  if [ "${ctx}" != "${CONTEXT}" ]; then
    fail "resolved kube context is '${ctx:-<none>}', expected '${CONTEXT}'; refusing to mutate or clean up. Run 'kubectl config use-context ${CONTEXT}' first."
  fi
  pass "resolved context matches expected '${CONTEXT}'"
}

record_versions() {
  info "kind version: $(kind version)"
  info "kubectl client/server version:"
  kubectl --context "${CONTEXT}" version || true
  info "installed Agent Sandbox CRDs:"
  kubectl --context "${CONTEXT}" get crd -o name | grep -E 'agents\.x-k8s\.io' || fail "expected agent-sandbox CRDs not found"
  info "controller deployment:"
  kubectl --context "${CONTEXT}" -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o wide || fail "controller deployment not found"
  info "controller image: $(kubectl --context "${CONTEXT}" -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o jsonpath='{.spec.template.spec.containers[0].image}')"
  info "pinned Agent Sandbox version: ${AGENT_SANDBOX_VERSION}"
}

cmd_status() {
  local ctx
  ctx="$(current_context)"
  info "expected context: ${CONTEXT}"
  info "resolved context: ${ctx:-<none>}"
  if ! cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' does not exist"
    return 0
  fi
  pass "kind cluster '${CLUSTER_NAME}' exists"
  if [ "${ctx}" != "${CONTEXT}" ]; then
    info "WARNING: current context does not match the cluster created by this script"
    return 0
  fi
  record_versions
}

cmd_up() {
  check_prereqs
  if cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' already exists, skipping creation"
    require_context
  else
    info "creating kind cluster '${CLUSTER_NAME}'"
    kind create cluster --name "${CLUSTER_NAME}"
    require_context
  fi

  mkdir -p "${DOWNLOAD_DIR}"
  info "downloading pinned Agent Sandbox ${AGENT_SANDBOX_VERSION} manifests"
  curl -fsSL -o "${DOWNLOAD_DIR}/manifest.yaml" "${RELEASE_BASE}/manifest.yaml" \
    || fail "could not download core manifest for ${AGENT_SANDBOX_VERSION}"
  curl -fsSL -o "${DOWNLOAD_DIR}/extensions.yaml" "${RELEASE_BASE}/extensions.yaml" \
    || fail "could not download extensions manifest for ${AGENT_SANDBOX_VERSION}"
  pass "downloaded pinned manifests to ${DOWNLOAD_DIR}"

  info "applying core manifest (namespace, RBAC, Sandbox CRD, controller)"
  kubectl --context "${CONTEXT}" apply -f "${DOWNLOAD_DIR}/manifest.yaml"
  kubectl --context "${CONTEXT}" -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after core manifest apply"

  info "applying extensions manifest (Template/WarmPool/Claim CRDs, --extensions)"
  kubectl --context "${CONTEXT}" apply -f "${DOWNLOAD_DIR}/extensions.yaml"
  kubectl --context "${CONTEXT}" -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after extensions manifest apply"

  pass "Agent Sandbox ${AGENT_SANDBOX_VERSION} installed and controller ready"
  record_versions
}

cmd_smoke() {
  require_context
  kubectl --context "${CONTEXT}" get crd sandboxclaims.extensions.agents.x-k8s.io >/dev/null 2>&1 \
    || fail "extension CRDs not installed; run 'reproduce.sh up' first"

  info "creating namespace '${SMOKE_NAMESPACE}' (idempotent)"
  kubectl --context "${CONTEXT}" create namespace "${SMOKE_NAMESPACE}" --dry-run=client -o yaml \
    | kubectl --context "${CONTEXT}" apply -f -

  info "applying minimal upstream-native SandboxTemplate/WarmPool/Claim"
  kubectl --context "${CONTEXT}" apply -f "${MANIFEST_DIR}/00-template.yaml"
  kubectl --context "${CONTEXT}" apply -f "${MANIFEST_DIR}/01-warmpool.yaml"
  kubectl --context "${CONTEXT}" apply -f "${MANIFEST_DIR}/02-claim.yaml"

  info "waiting up to ${SMOKE_TIMEOUT_SECONDS}s for SandboxClaim/smoke-claim Ready=True"
  local elapsed=0 ready=""
  while [ "${elapsed}" -lt "${SMOKE_TIMEOUT_SECONDS}" ]; do
    ready="$(kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get sandboxclaim smoke-claim \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
    [ "${ready}" = "True" ] && break
    sleep 3
    elapsed=$((elapsed + 3))
  done

  if [ "${ready}" != "True" ]; then
    printf '[fail] SandboxClaim never reached Ready=True within %ss; reporting explicitly, not a pass.\n' "${SMOKE_TIMEOUT_SECONDS}" >&2
    kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get sandboxtemplate,sandboxwarmpool,sandboxclaim,sandbox,pod -o wide || true
    kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get events --sort-by=.lastTimestamp || true
    exit 1
  fi
  pass "SandboxClaim/smoke-claim observed Ready=True"

  info "observed state:"
  kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get sandboxtemplate,sandboxwarmpool,sandboxclaim,sandbox,pod -o wide

  info "terminating: deleting SandboxClaim/smoke-claim"
  kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" delete sandboxclaim smoke-claim --ignore-not-found=true
  # Observed upstream behavior (v0.4.6): deleting a warm-pool-backed claim
  # RECYCLES the underlying Sandbox back into the pool for reuse; it does not
  # terminate the pod. The sandbox is only actually torn down once its
  # SandboxWarmPool (and thus its SandboxTemplate) is deleted below. This is
  # a real substrate finding, not a script defect — record it for #48.

  info "tearing down warm pool and template to fully terminate the recycled sandbox"
  kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" delete sandboxwarmpool smoke-pool --ignore-not-found=true
  kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" delete sandboxtemplate smoke-template --ignore-not-found=true

  elapsed=0
  local remaining="unknown"
  while [ "${elapsed}" -lt "${SMOKE_TIMEOUT_SECONDS}" ]; do
    remaining="$(kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ' || true)"
    [ "${remaining}" = "0" ] && break
    sleep 3
    elapsed=$((elapsed + 3))
  done
  if [ "${remaining}" != "0" ]; then
    printf '[fail] sandbox pod(s) still present %ss after pool/template teardown; reporting explicitly.\n' "${SMOKE_TIMEOUT_SECONDS}" >&2
    kubectl --context "${CONTEXT}" -n "${SMOKE_NAMESPACE}" get pods -o wide || true
    exit 1
  fi
  pass "sandbox terminated and cleaned up after claim/pool/template teardown"

  info "cleaning up scoped smoke resources (namespace ${SMOKE_NAMESPACE})"
  kubectl --context "${CONTEXT}" delete namespace "${SMOKE_NAMESPACE}" --ignore-not-found=true
  pass "smoke namespace and resources removed"
}

cmd_down() {
  if ! cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' does not exist; nothing to clean up"
    return 0
  fi
  require_context
  info "deleting kind cluster '${CLUSTER_NAME}' (only this cluster)"
  kind delete cluster --name "${CLUSTER_NAME}"
  pass "kind cluster '${CLUSTER_NAME}' deleted"
}

write_summary() {
  local exit_code="$1" result
  result="pass"
  [ "${exit_code}" -ne 0 ] && result="fail"
  mkdir -p "${EVIDENCE_DIR}"
  local branch commit
  branch="$(git -C "${ROOT}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
  commit="$(git -C "${ROOT}" rev-parse HEAD 2>/dev/null || echo unknown)"
  cat > "${SUMMARY_FILE}" <<EOF
# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Branch: ${branch}
- Commit: ${commit}
- Command: reproduce.sh ${SUBCOMMAND:-all}
- Pinned Agent Sandbox version: ${AGENT_SANDBOX_VERSION}
- kind cluster/context: ${CLUSTER_NAME} / ${CONTEXT}
- Result: ${result}

Raw output: \`output.txt\`
EOF
}

main() {
  local subcommand="${1:-all}"
  SUBCOMMAND="${subcommand}"

  case "${subcommand}" in
    -h|--help) usage; exit 0 ;;
    status|up|smoke|down|all) ;;
    *) usage; fail "unknown subcommand: ${subcommand}" ;;
  esac

  mkdir -p "${EVIDENCE_DIR}"
  : > "${OUTPUT_FILE}"
  exec > >(tee -a "${OUTPUT_FILE}") 2>&1

  case "${subcommand}" in
    status) cmd_status ;;
    up) cmd_up ;;
    smoke) cmd_smoke ;;
    down) cmd_down ;;
    all)
      cmd_up
      cmd_smoke
      cmd_down
      ;;
  esac
}

trap 'write_summary $?' EXIT
main "$@"
