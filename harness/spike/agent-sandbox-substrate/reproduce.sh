#!/usr/bin/env bash
# E8-T3 (#50): reproduce the upstream Agent Sandbox test substrate on a
# disposable local kind cluster.
#
# This script proves the PINNED UPSTREAM lifecycle only. It never touches
# Agenova claim types, the RuntimeBackend contract, or
# internal/runtime/agentsandbox. The Agenova adapter proof is E8-T4 (#51).
#
# See RUNBOOK.md for usage and troubleshooting.
set -euo pipefail

# --- pinned versions / names -------------------------------------------------
# These pins are the single source of truth for the substrate. The RUNBOOK
# "Pinned versions" table documents them and when to bump each one.
#
#   AGENT_SANDBOX_VERSION    installed on every run (the thing this spike proves).
#   KIND_FALLBACK_VERSION    used ONLY when `kind` is not already on PATH. An
#                            existing `kind` is used as-is at whatever version it is.
#   KUBECTL_FALLBACK_VERSION used ONLY when `kubectl` is not already on PATH.
#
# Bump AGENT_SANDBOX_VERSION when the team adopts a newer upstream release.
# Bump the *_FALLBACK_VERSION pins when a clean machine should bootstrap a
# newer kind/kubectl; verified working values as of 2026-08-31 below.
#
# NOTE: this substrate is deliberately pinned to v0.4.6 to match the rest of
# the codebase (the internal/runtime/agentsandbox adapter, docs/backends,
# THIRD_PARTY_NOTICES.md, and the repository.ps1 check are all on v0.4.6 /
# extensions.agents.x-k8s.io/v1alpha1). Moving to v1.0.0 / v1beta1 is a
# separate E8 change tracked by the #66 mapping spike.
CLUSTER_NAME="agenova-k8s-lab"
CONTEXT="kind-${CLUSTER_NAME}"
AGENT_SANDBOX_VERSION="v0.4.6"      # matches the codebase adapter (v1alpha1 APIs)
KIND_FALLBACK_VERSION="v0.33.0"     # kind latest as of 2026-08-31
KUBECTL_FALLBACK_VERSION="v1.34.0"  # tracks the kind v0.33.0 default node (k8s v1.34)

CONTROLLER_NAMESPACE="agent-sandbox-system"
CONTROLLER_DEPLOY="agent-sandbox-controller"
SMOKE_NAMESPACE="agent-sandbox-smoke"
SMOKE_TIMEOUT_SECONDS=180

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
MANIFEST_DIR="${SCRIPT_DIR}/manifests"
TOOLS_DIR="${ROOT}/.tmp/${CLUSTER_NAME}-tools"
DOWNLOAD_DIR="${ROOT}/.tmp/agent-sandbox-${AGENT_SANDBOX_VERSION}"
EVIDENCE_DIR="${ROOT}/docs/evidence/E8-T3/agent-sandbox-substrate"
OUTPUT_FILE="${EVIDENCE_DIR}/output.txt"
SUMMARY_FILE="${EVIDENCE_DIR}/summary.md"

RELEASE_BASE="https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${AGENT_SANDBOX_VERSION}"

KIND_BIN=""
KUBECTL_BIN=""
KIND_SOURCE=""
KUBECTL_SOURCE=""

# --- logging (all to stderr so $(resolve_tool ...) stays clean) -------------
info() { printf '[info] %s\n' "$1" >&2; }
pass() { printf '[pass] %s\n' "$1" >&2; }
fail() { printf '[fail] %s\n' "$1" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage: reproduce.sh <status|tools|up|smoke|down|all>

  status  Read-only: kube context, cluster existence, CRDs, controller readiness.
  tools   Read-only: resolve kind/kubectl (reuse existing or fetch pinned) and print versions.
  up      Create the kind cluster (if missing) and install pinned Agent Sandbox.
  smoke   Create/observe/terminate/clean up one minimal upstream SandboxClaim.
  down    Delete the kind cluster created by this script. Nothing else is touched.
  all     up + smoke + down (default).
EOF
}

# --- prerequisites ---------------------------------------------------------
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing prerequisite: $1 not found on PATH"
}

require_docker() {
  docker info >/dev/null 2>&1 \
    || fail "docker daemon is not reachable (start Docker Desktop / your engine and retry)"
  pass "docker daemon reachable"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# Normalised host OS: linux | darwin | windows (windows covers Git Bash / MSYS /
# Cygwin, where this script runs under bash; native PowerShell/cmd cannot).
host_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux*) printf 'linux' ;;
    darwin*) printf 'darwin' ;;
    mingw*|msys*|cygwin*|windows*) printf 'windows' ;;
    *) fail "unsupported OS $(uname -s) for tool auto-install (install kind/kubectl manually)" ;;
  esac
}
host_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture $(uname -m) for tool auto-install" ;;
  esac
}
exe_suffix() { [ "$(host_os)" = "windows" ] && printf '.exe' || true; }

# download_pinned_tool <name> <pinned-version>
# Downloads a checksum-verified pinned binary into .tmp/ and prints its path.
# Only called when <name> is not already on PATH.
download_pinned_tool() {
  local name="$1" version="$2"
  local os arch ext dest url sha_url expected got
  os="$(host_os)"
  arch="$(host_arch)"
  ext="$(exe_suffix)"
  mkdir -p "${TOOLS_DIR}"
  dest="${TOOLS_DIR}/${name}${ext}"

  if [ -x "${dest}" ]; then
    printf '%s\n' "${dest}"
    return 0
  fi

  case "${name}" in
    kind)
      url="https://github.com/kubernetes-sigs/kind/releases/download/${version}/kind-${os}-${arch}${ext}"
      sha_url="${url}.sha256sum"
      ;;
    kubectl)
      url="https://dl.k8s.io/release/${version}/bin/${os}/${arch}/kubectl${ext}"
      sha_url="${url}.sha256"
      ;;
    *)
      fail "no auto-install rule for ${name}"
      ;;
  esac

  info "downloading pinned ${name} ${version} for ${os}/${arch}"
  curl -fsSL -o "${dest}.part" "${url}" || fail "download failed: ${url}"
  expected="$(curl -fsSL "${sha_url}" | awk '{print $1}')" \
    || fail "checksum fetch failed: ${sha_url}"
  got="$(sha256_of "${dest}.part")"
  if [ -z "${expected}" ] || [ "${expected}" != "${got}" ]; then
    rm -f "${dest}.part"
    fail "${name} checksum mismatch: expected '${expected:-<none>}', got '${got}'"
  fi
  chmod +x "${dest}.part"
  mv "${dest}.part" "${dest}"
  info "installed pinned ${name} at ${dest}"
  printf '%s\n' "${dest}"
}

resolve_tools() {
  require_cmd curl
  if command -v kind >/dev/null 2>&1; then
    KIND_BIN="$(command -v kind)"; KIND_SOURCE="existing"
  else
    KIND_BIN="$(download_pinned_tool kind "${KIND_FALLBACK_VERSION}")"; KIND_SOURCE="pinned:${KIND_FALLBACK_VERSION}"
  fi
  if command -v kubectl >/dev/null 2>&1; then
    KUBECTL_BIN="$(command -v kubectl)"; KUBECTL_SOURCE="existing"
  else
    KUBECTL_BIN="$(download_pinned_tool kubectl "${KUBECTL_FALLBACK_VERSION}")"; KUBECTL_SOURCE="pinned:${KUBECTL_FALLBACK_VERSION}"
  fi
  pass "kind: ${KIND_BIN} (${KIND_SOURCE})"
  pass "kubectl: ${KUBECTL_BIN} (${KUBECTL_SOURCE})"
}

kc() { "${KUBECTL_BIN}" --context "${CONTEXT}" "$@"; }

# --- context safety ------------------------------------------------------
current_context() { "${KUBECTL_BIN}" config current-context 2>/dev/null || true; }

cluster_exists() {
  "${KIND_BIN}" get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"
}

require_context() {
  local ctx
  ctx="$(current_context)"
  if [ "${ctx}" != "${CONTEXT}" ]; then
    fail "resolved kube context is '${ctx:-<none>}', expected '${CONTEXT}'; refusing to mutate or clean up. Run '${KUBECTL_BIN} config use-context ${CONTEXT}' first."
  fi
  pass "resolved context matches expected '${CONTEXT}'"
}

record_versions() {
  info "kind binary: ${KIND_BIN} (${KIND_SOURCE})"
  info "kind version: $("${KIND_BIN}" version)"
  info "kubectl binary: ${KUBECTL_BIN} (${KUBECTL_SOURCE})"
  info "kubectl / server version:"
  kc version || true
  info "installed Agent Sandbox CRDs:"
  kc get crd -o name | grep -E 'agents\.x-k8s\.io' || fail "expected agent-sandbox CRDs not found"
  info "controller deployment:"
  kc -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o wide || fail "controller deployment not found"
  info "controller image: $(kc -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o jsonpath='{.spec.template.spec.containers[0].image}')"
  info "pinned Agent Sandbox version: ${AGENT_SANDBOX_VERSION}"
}

# --- subcommands --------------------------------------------------------
cmd_tools() {
  resolve_tools
  info "kind version: $("${KIND_BIN}" version)"
  info "kubectl client version: $("${KUBECTL_BIN}" version --client 2>/dev/null | head -1)"
}

cmd_status() {
  resolve_tools
  local ctx
  ctx="$(current_context)"
  info "expected context: ${CONTEXT}"
  info "resolved context: ${ctx:-<none>}"
  if docker info >/dev/null 2>&1; then
    info "docker daemon: reachable"
  else
    info "docker daemon: NOT reachable (up/smoke/down will fail until it is started)"
  fi
  if ! cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' does not exist"
    return 0
  fi
  pass "kind cluster '${CLUSTER_NAME}' exists"
  if [ "${ctx}" != "${CONTEXT}" ]; then
    info "WARNING: current context is not the cluster this script manages; skipping version dump"
    return 0
  fi
  record_versions
}

cmd_up() {
  require_docker
  resolve_tools

  if cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' already exists, skipping creation"
    require_context
  else
    info "creating kind cluster '${CLUSTER_NAME}'"
    "${KIND_BIN}" create cluster --name "${CLUSTER_NAME}" --wait 90s
    require_context
  fi

  mkdir -p "${DOWNLOAD_DIR}"
  info "downloading pinned Agent Sandbox ${AGENT_SANDBOX_VERSION} manifests"
  curl -fsSL -o "${DOWNLOAD_DIR}/manifest.yaml" "${RELEASE_BASE}/manifest.yaml" \
    || fail "could not download core manifest (manifest.yaml) for ${AGENT_SANDBOX_VERSION}"
  curl -fsSL -o "${DOWNLOAD_DIR}/extensions.yaml" "${RELEASE_BASE}/extensions.yaml" \
    || fail "could not download extensions manifest for ${AGENT_SANDBOX_VERSION}"
  pass "downloaded pinned manifests to ${DOWNLOAD_DIR}"

  info "applying core manifest (namespace, RBAC, Sandbox CRD, controller)"
  kc apply -f "${DOWNLOAD_DIR}/manifest.yaml"
  kc -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after core manifest apply"

  info "applying extensions manifest (Template/WarmPool/Claim CRDs)"
  kc apply -f "${DOWNLOAD_DIR}/extensions.yaml"
  kc -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after extensions manifest apply"

  pass "Agent Sandbox ${AGENT_SANDBOX_VERSION} installed and controller ready"
  record_versions
}

cmd_smoke() {
  resolve_tools
  require_context
  kc get crd sandboxclaims.extensions.agents.x-k8s.io >/dev/null 2>&1 \
    || fail "extension CRDs not installed; run 'reproduce.sh up' first"

  info "creating namespace '${SMOKE_NAMESPACE}' (idempotent)"
  kc create namespace "${SMOKE_NAMESPACE}" --dry-run=client -o yaml | kc apply -f -

  info "applying minimal upstream-native SandboxTemplate/WarmPool/Claim"
  kc apply -f "${MANIFEST_DIR}/00-template.yaml"
  kc apply -f "${MANIFEST_DIR}/01-warmpool.yaml"
  kc apply -f "${MANIFEST_DIR}/02-claim.yaml"

  info "waiting up to ${SMOKE_TIMEOUT_SECONDS}s for SandboxClaim/smoke-claim Ready=True"
  local elapsed=0 ready=""
  while [ "${elapsed}" -lt "${SMOKE_TIMEOUT_SECONDS}" ]; do
    ready="$(kc -n "${SMOKE_NAMESPACE}" get sandboxclaim smoke-claim \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
    [ "${ready}" = "True" ] && break
    sleep 5
    elapsed=$((elapsed + 5))
  done

  if [ "${ready}" != "True" ]; then
    printf '[fail] SandboxClaim never reached Ready=True within %ss; reporting explicitly, not a pass.\n' "${SMOKE_TIMEOUT_SECONDS}" >&2
    kc -n "${SMOKE_NAMESPACE}" get sandboxtemplate,sandboxwarmpool,sandboxclaim,sandbox,pod -o wide || true
    kc -n "${SMOKE_NAMESPACE}" get events --sort-by=.lastTimestamp || true
    kc -n "${CONTROLLER_NAMESPACE}" logs "deploy/${CONTROLLER_DEPLOY}" --tail=50 || true
    exit 1
  fi
  pass "SandboxClaim/smoke-claim observed Ready=True"

  info "observed state:"
  kc -n "${SMOKE_NAMESPACE}" get sandboxtemplate,sandboxwarmpool,sandboxclaim,sandbox,pod -o wide
  info "claim status.sandbox: $(kc -n "${SMOKE_NAMESPACE}" get sandboxclaim smoke-claim -o jsonpath='{.status.sandbox}' 2>/dev/null || true)"

  # Observed upstream behavior (v0.4.6): deleting a warm-pool-backed claim
  # RECYCLES its Sandbox back into the pool rather than terminating the pod;
  # the sandbox only actually goes away once the pool and template are also
  # deleted. The teardown below accounts for that and still asserts pods -> 0.
  info "terminating: deleting SandboxClaim/smoke-claim"
  kc -n "${SMOKE_NAMESPACE}" delete sandboxclaim smoke-claim --ignore-not-found=true --timeout=60s

  info "tearing down warm pool and template"
  kc -n "${SMOKE_NAMESPACE}" delete sandboxwarmpool smoke-pool --ignore-not-found=true --timeout=60s
  kc -n "${SMOKE_NAMESPACE}" delete sandboxtemplate smoke-template --ignore-not-found=true --timeout=60s

  info "waiting up to ${SMOKE_TIMEOUT_SECONDS}s for all sandbox pods to terminate"
  elapsed=0
  local remaining="unknown"
  while [ "${elapsed}" -lt "${SMOKE_TIMEOUT_SECONDS}" ]; do
    remaining="$(kc -n "${SMOKE_NAMESPACE}" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ' || true)"
    [ "${remaining}" = "0" ] && break
    sleep 5
    elapsed=$((elapsed + 5))
  done
  if [ "${remaining}" != "0" ]; then
    printf '[fail] %s sandbox pod(s) still present %ss after claim/pool/template teardown; reporting explicitly.\n' "${remaining}" "${SMOKE_TIMEOUT_SECONDS}" >&2
    kc -n "${SMOKE_NAMESPACE}" get pods -o wide || true
    exit 1
  fi
  pass "sandbox terminated and cleaned up after claim/pool/template teardown"

  info "cleaning up scoped smoke namespace '${SMOKE_NAMESPACE}'"
  # --wait=false: request deletion but do not block on finalization. The
  # meaningful cleanup (sandbox pods -> 0) is already asserted above, and a
  # blocking wait here is prone to a dropped watch on kind. A follow-up
  # 'down' deletes the whole cluster regardless.
  kc delete namespace "${SMOKE_NAMESPACE}" --ignore-not-found=true --wait=false
  elapsed=0
  local ns_state="present"
  while [ "${elapsed}" -lt 60 ]; do
    kc get namespace "${SMOKE_NAMESPACE}" >/dev/null 2>&1 || { ns_state="gone"; break; }
    sleep 5
    elapsed=$((elapsed + 5))
  done
  if [ "${ns_state}" = "gone" ]; then
    pass "smoke namespace and resources removed"
  else
    info "smoke namespace still finalizing after 60s; deletion is requested and 'down' will remove the cluster"
  fi
}

cmd_down() {
  resolve_tools
  if ! cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' does not exist; nothing to clean up"
    return 0
  fi
  require_context
  info "deleting kind cluster '${CLUSTER_NAME}' (only this cluster)"
  "${KIND_BIN}" delete cluster --name "${CLUSTER_NAME}"
  pass "kind cluster '${CLUSTER_NAME}' deleted"
}

write_summary() {
  local exit_code="$1" result="pass"
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
- kind: ${KIND_BIN:-unresolved} (${KIND_SOURCE:-n/a})
- kubectl: ${KUBECTL_BIN:-unresolved} (${KUBECTL_SOURCE:-n/a})
- kind cluster / context: ${CLUSTER_NAME} / ${CONTEXT}
- Result: ${result}

## Notes

- Scope: the pinned upstream Agent Sandbox lifecycle only. No Agenova
  \`ClaimRequest\`/\`SandboxClaim\`, \`RuntimeBackend\` adapter, or contract is
  exercised here (that is E8-T4 / #51).
- Reruns are idempotent: cluster and namespace creation are safe to repeat,
  and teardown deletes only the \`${CLUSTER_NAME}\` kind cluster and the
  \`${SMOKE_NAMESPACE}\` namespace created by this script.
- Not isolated here: claim-only deletion vs warm-pool "recycle" behaviour
  (see #48). This harness deletes the claim, pool, and template together and
  asserts the sandbox pod count reaches zero.

Raw output: \`output.txt\`
EOF
}

main() {
  local subcommand="${1:-all}"
  SUBCOMMAND="${subcommand}"

  case "${subcommand}" in
    -h|--help) usage; exit 0 ;;
    status|tools|up|smoke|down|all) ;;
    *) usage; fail "unknown subcommand: ${subcommand}" ;;
  esac

  mkdir -p "${EVIDENCE_DIR}"
  : > "${OUTPUT_FILE}"
  exec > >(tee -a "${OUTPUT_FILE}") 2>&1
  trap 'write_summary $?' EXIT

  case "${subcommand}" in
    status) cmd_status ;;
    tools) cmd_tools ;;
    up) cmd_up ;;
    smoke) cmd_smoke ;;
    down) cmd_down ;;
    all) cmd_up; cmd_smoke; cmd_down ;;
  esac
}

main "$@"
