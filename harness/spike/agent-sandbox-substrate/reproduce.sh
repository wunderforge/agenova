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
#   KUBECTL_FALLBACK_VERSION used ONLY when `kubectl` is not already on PATH.
#
# `kind` is NOT installed by this script. It must already be on PATH, installed
# with a supported package manager per the upstream quick-start
# (https://kind.sigs.k8s.io/docs/user/quick-start/#installing-with-a-package-manager):
# macOS `brew install kind` or `sudo port install kind`; Windows `choco install kind`.
# An existing `kind` is used as-is at whatever version it is; a missing `kind`
# is a loud prerequisite failure, never a download. Owner decision 2026-09-10.
#
# Bump AGENT_SANDBOX_VERSION when the team adopts a newer upstream release.
# Bump KUBECTL_FALLBACK_VERSION when a clean machine should bootstrap a newer
# kubectl; verified working value as of 2026-08-31 below.
#
# NOTE: this substrate is deliberately pinned to v0.4.6 to match the rest of
# the codebase (the internal/runtime/agentsandbox adapter, docs/backends,
# THIRD_PARTY_NOTICES.md, and the repository.ps1 check are all on v0.4.6 /
# extensions.agents.x-k8s.io/v1alpha1). Adopting a newer upstream release
# across E8 is a separate change tracked by the #66 mapping spike.
CLUSTER_NAME="agenova-k8s-lab"
CONTEXT="kind-${CLUSTER_NAME}"
AGENT_SANDBOX_VERSION="v0.4.6"      # matches the codebase adapter (v1alpha1 APIs)
KUBECTL_FALLBACK_VERSION="v1.34.0"  # fallback client only; server version is recorded separately
KIND_INSTALL_DOC="https://kind.sigs.k8s.io/docs/user/quick-start/#installing-with-a-package-manager"

CONTROLLER_NAMESPACE="agent-sandbox-system"
CONTROLLER_DEPLOY="agent-sandbox-controller"
SMOKE_NAMESPACE="agent-sandbox-smoke"
SMOKE_TIMEOUT_SECONDS=180

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
MANIFEST_DIR="${SCRIPT_DIR}/manifests"
TOOLS_DIR="${ROOT}/.tmp/${CLUSTER_NAME}-tools"
DOWNLOAD_DIR="${ROOT}/.tmp/agent-sandbox-${AGENT_SANDBOX_VERSION}"
STATE_DIR="${ROOT}/.tmp/${CLUSTER_NAME}-owner"
OWNER_FILE="${STATE_DIR}/identity"
OWNER_LABEL="agenova.io/substrate-owner"
OWNER_UID=""
EVIDENCE_DIR=""
ALLOW_DOWNLOADS=true

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
Usage: reproduce.sh <status|tools|up|smoke|teardown|down|all> [--capture]

  status    Read-only: inspect installed tools and cluster (no downloads/files).
  tools     Read-only: inspect existing tools (no downloads/files).
  up        Create a disposable cluster or reuse this checkout's owned cluster.
  smoke     Create fixtures in an owned namespace and observe Ready.
  teardown  Delete owned fixtures, wait for zero pods and namespace deletion.
  down      Delete only the cluster matching this checkout's saved fingerprint.
  all       up + smoke + teardown + down (default).
  --capture Explicitly capture a mutating phase under a new .tmp directory.
            Captures never overwrite committed evidence.

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
    *) fail "unsupported OS $(uname -s) for kubectl auto-install (install kubectl manually)" ;;
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
# Only called for `kubectl` when it is not already on PATH. `kind` is never
# downloaded (install it with a package manager, see KIND_INSTALL_DOC).
download_pinned_tool() {
  local name="$1" version="$2"
  local os arch ext dest url sha_url expected got
  os="$(host_os)"
  arch="$(host_arch)"
  ext="$(exe_suffix)"
  mkdir -p "${TOOLS_DIR}"
  dest="${TOOLS_DIR}/${name}-${version}-${os}-${arch}${ext}"

  if [ -x "${dest}" ]; then
    printf '%s\n' "${dest}"
    return 0
  fi

  case "${name}" in
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

cached_tool() {
  local path="${TOOLS_DIR}/$1-$2-$(host_os)-$(host_arch)$(exe_suffix)"
  [ -x "${path}" ] || fail "$1 is unavailable; diagnostics do not download tools (run up)"
  printf '%s\n' "${path}"
}

require_kind() {
  command -v kind >/dev/null 2>&1 || fail "kind not found on PATH. This harness does not install kind; add it with a supported package manager (${KIND_INSTALL_DOC}): macOS 'brew install kind' or 'sudo port install kind'; Windows 'choco install kind'."
  KIND_BIN="$(command -v kind)"; KIND_SOURCE="existing"
}

resolve_tools() {
  require_kind
  if command -v kubectl >/dev/null 2>&1; then
    KUBECTL_BIN="$(command -v kubectl)"; KUBECTL_SOURCE="existing"
  elif [ "${ALLOW_DOWNLOADS}" = false ]; then
    KUBECTL_BIN="$(cached_tool kubectl "${KUBECTL_FALLBACK_VERSION}")"; KUBECTL_SOURCE="cached:${KUBECTL_FALLBACK_VERSION}"
  else
    require_cmd curl
    KUBECTL_BIN="$(download_pinned_tool kubectl "${KUBECTL_FALLBACK_VERSION}")"; KUBECTL_SOURCE="pinned:${KUBECTL_FALLBACK_VERSION}"
  fi
  pass "kind: ${KIND_BIN} (${KIND_SOURCE})"
  pass "kubectl: ${KUBECTL_BIN} (${KUBECTL_SOURCE})"
}

kc() { "${KUBECTL_BIN}" --context "${CONTEXT}" "$@"; }

# --- context safety ------------------------------------------------------
current_context() { "${KUBECTL_BIN}" config current-context 2>/dev/null || true; }

cluster_exists() {
  local clusters
  clusters="$("${KIND_BIN}" get clusters)" || fail "could not enumerate kind clusters"
  printf '%s\n' "${clusters}" | grep -qx "${CLUSTER_NAME}"
}

require_context() {
  local ctx
  ctx="$(current_context)"
  if [ "${ctx}" != "${CONTEXT}" ]; then
    fail "resolved kube context is '${ctx:-<none>}', expected '${CONTEXT}'; refusing to mutate or clean up. Run '${KUBECTL_BIN} config use-context ${CONTEXT}' first."
  fi
  pass "resolved context matches expected '${CONTEXT}'"
}

# A name/context alone is not proof of ownership. Bind the local receipt to
# both the Docker control-plane container and the API server's kube-system UID.
cluster_fingerprint() {
  local node_id kube_uid
  node_id="$(docker inspect --format '{{.Id}}' "${CLUSTER_NAME}-control-plane")" \
    || fail "cannot inspect control-plane container"
  kube_uid="$(kc get namespace kube-system -o jsonpath='{.metadata.uid}')" \
    || fail "cannot read cluster identity"
  [ -n "${node_id}" ] && [ -n "${kube_uid}" ] || fail "empty cluster identity"
  printf '%s\n%s\n' "${node_id}" "${kube_uid}"
}

require_owned_cluster() {
  require_context
  [ -f "${OWNER_FILE}" ] || fail "cluster has no ownership receipt in this checkout; refusing to adopt or delete it"
  local actual expected
  actual="$(cluster_fingerprint)" || fail "cannot verify cluster fingerprint"
  expected="$(cat "${OWNER_FILE}")"
  [ "${actual}" = "${expected}" ] || fail "cluster fingerprint changed; refusing mutation or deletion"
  OWNER_UID="$(printf '%s\n' "${actual}" | tail -1)"
}

# All kubectl mutations revalidate ownership and pin the explicit context.
mutate() { require_owned_cluster; kc "$@"; }

namespace_name() {
  kc get namespace "${SMOKE_NAMESPACE}" --ignore-not-found -o name
}

require_owned_namespace() {
  require_owned_cluster
  local label
  label="$(kc get namespace "${SMOKE_NAMESPACE}" -o "jsonpath={.metadata.labels.agenova\\.io/substrate-owner}")" \
    || fail "cannot read smoke namespace ownership"
  [ "${label}" = "${OWNER_UID}" ] || fail "smoke namespace is not owned by this harness"
}

record_versions() {
  info "kind binary: ${KIND_BIN} (${KIND_SOURCE})"
  "${KIND_BIN}" version
  info "kubectl binary: ${KUBECTL_BIN} (${KUBECTL_SOURCE})"
  kc version
  local crd
  for crd in sandboxes.agents.x-k8s.io \
    sandboxtemplates.extensions.agents.x-k8s.io \
    sandboxwarmpools.extensions.agents.x-k8s.io \
    sandboxclaims.extensions.agents.x-k8s.io; do
    info "installed CRD ${crd}: version served storage"
    kc get crd "${crd}" -o jsonpath='{range .spec.versions[*]}{.name}{" served="}{.served}{" storage="}{.storage}{"\n"}{end}'
  done
  info "controller deployment:"
  kc -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o wide
  info "controller image:"
  kc -n "${CONTROLLER_NAMESPACE}" get deploy "${CONTROLLER_DEPLOY}" -o jsonpath='{.spec.template.spec.containers[*].image}{"\n"}'
  info "pinned Agent Sandbox version: ${AGENT_SANDBOX_VERSION}"
}

# --- subcommands --------------------------------------------------------
cmd_tools() {
  ALLOW_DOWNLOADS=false
  resolve_tools
  "${KIND_BIN}" version
  "${KUBECTL_BIN}" version --client
}

cmd_status() {
  ALLOW_DOWNLOADS=false
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
  require_cmd curl
  resolve_tools

  if cluster_exists; then
    require_owned_cluster
    info "reusing cluster with matching ownership fingerprint"
  else
    info "creating kind cluster '${CLUSTER_NAME}'"
    "${KIND_BIN}" create cluster --name "${CLUSTER_NAME}" --wait 90s
    require_context
    local identity
    identity="$(cluster_fingerprint)" || fail "cluster created but fingerprint unavailable; manual inspection required"
    mkdir -p "${STATE_DIR}"
    printf '%s\n' "${identity}" > "${OWNER_FILE}"
    require_owned_cluster
  fi

  mkdir -p "${DOWNLOAD_DIR}"
  info "downloading pinned Agent Sandbox ${AGENT_SANDBOX_VERSION} manifests"
  curl -fsSL -o "${DOWNLOAD_DIR}/manifest.yaml" "${RELEASE_BASE}/manifest.yaml" \
    || fail "could not download core manifest (manifest.yaml) for ${AGENT_SANDBOX_VERSION}"
  curl -fsSL -o "${DOWNLOAD_DIR}/extensions.yaml" "${RELEASE_BASE}/extensions.yaml" \
    || fail "could not download extensions manifest for ${AGENT_SANDBOX_VERSION}"
  pass "downloaded pinned manifests to ${DOWNLOAD_DIR}"

  info "applying core manifest (namespace, RBAC, Sandbox CRD, controller)"
  mutate apply -f "${DOWNLOAD_DIR}/manifest.yaml"
  kc -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after core manifest apply"

  info "applying extensions manifest (Template/WarmPool/Claim CRDs)"
  mutate apply -f "${DOWNLOAD_DIR}/extensions.yaml"
  kc -n "${CONTROLLER_NAMESPACE}" rollout status "deploy/${CONTROLLER_DEPLOY}" --timeout=180s \
    || fail "controller did not become ready after extensions manifest apply"

  pass "Agent Sandbox ${AGENT_SANDBOX_VERSION} installed and controller ready"
  record_versions
}

cmd_smoke() {
  resolve_tools
  require_owned_cluster
  kc get crd sandboxclaims.extensions.agents.x-k8s.io >/dev/null 2>&1 \
    || fail "extension CRDs not installed; run 'reproduce.sh up' first"

  local existing
  existing="$(namespace_name)" || fail "could not inspect smoke namespace"
  if [ -n "${existing}" ]; then
    require_owned_namespace
  else
    # create (not apply) refuses a namespace that appeared after the read.
    cat <<EOF | mutate create -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${SMOKE_NAMESPACE}
  labels:
    ${OWNER_LABEL}: ${OWNER_UID}
EOF
  fi
  require_owned_namespace

  info "applying minimal upstream-native SandboxTemplate/WarmPool/Claim"
  require_owned_namespace
  mutate apply -f "${MANIFEST_DIR}/00-template.yaml"
  require_owned_namespace
  mutate apply -f "${MANIFEST_DIR}/01-warmpool.yaml"
  require_owned_namespace
  mutate apply -f "${MANIFEST_DIR}/02-claim.yaml"

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

  info "smoke fixtures left in place in namespace '${SMOKE_NAMESPACE}'; run 'reproduce.sh teardown' to delete them and assert pods -> 0, or 'reproduce.sh down' to remove the whole cluster"
}

cmd_teardown() {
  resolve_tools
  require_owned_cluster
  local existing elapsed=0 remaining
  existing="$(namespace_name)" || fail "could not inspect smoke namespace"
  [ -n "${existing}" ] || { pass "smoke namespace already absent"; return 0; }
  require_owned_namespace

  # Removing claim, pool AND template prevents warm-pool replenishment.
  local resource
  for resource in sandboxclaim/smoke-claim sandboxwarmpool/smoke-pool sandboxtemplate/smoke-template; do
    require_owned_namespace
    mutate -n "${SMOKE_NAMESPACE}" delete "${resource}" --ignore-not-found=true --timeout=60s
  done
  while [ "${elapsed}" -lt "${SMOKE_TIMEOUT_SECONDS}" ]; do
    remaining="$(kc -n "${SMOKE_NAMESPACE}" get pods -o name)" \
      || fail "cannot observe pod cleanup"
    [ -z "${remaining}" ] && break
    sleep 5
    elapsed=$((elapsed + 5))
  done
  [ -z "${remaining}" ] || fail "sandbox pods did not terminate within ${SMOKE_TIMEOUT_SECONDS}s"
  pass "sandbox pods removed after claim/pool/template teardown"

  require_owned_namespace
  mutate delete namespace "${SMOKE_NAMESPACE}" --wait=true --timeout=60s
  existing="$(namespace_name)" || fail "could not verify namespace removal"
  [ -z "${existing}" ] || fail "smoke namespace remains after deletion"
  pass "owned smoke namespace removed"
}

cmd_down() {
  resolve_tools
  if ! cluster_exists; then
    info "kind cluster '${CLUSTER_NAME}' does not exist; nothing to clean up"
    return 0
  fi
  require_owned_cluster
  info "deleting kind cluster '${CLUSTER_NAME}' (only this cluster)"
  "${KIND_BIN}" delete cluster --name "${CLUSTER_NAME}"
  if cluster_exists; then fail "kind cluster remains after deletion"; fi
  rm -f "${OWNER_FILE}"
  pass "owned kind cluster '${CLUSTER_NAME}' deleted"
}

write_summary() {
  local exit_code="$1" result="pass" commit branch dirty
  [ "${exit_code}" -eq 0 ] || result="fail"
  commit="$(git -C "${ROOT}" rev-parse HEAD)"
  branch="$(git -C "${ROOT}" rev-parse --abbrev-ref HEAD)"
  dirty="$(git -C "${ROOT}" status --porcelain)"
  cat > "${EVIDENCE_DIR}/summary.md" <<EOF
# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Branch / commit: ${branch} / ${commit}
- Working tree: ${dirty:-clean}
- Script SHA256: $(sha256_of "${SCRIPT_DIR}/reproduce.sh")
- Command: bash harness/spike/agent-sandbox-substrate/reproduce.sh ${SUBCOMMAND} --capture
- Agent Sandbox: ${AGENT_SANDBOX_VERSION}
- kind / kubectl: ${KIND_BIN:-unresolved} / ${KUBECTL_BIN:-unresolved}
- Cluster / context: ${CLUSTER_NAME} / ${CONTEXT}
- Result: ${result}

Raw output: output.txt. A phase pass proves only that phase; completion requires
two real all runs from the committed final script. No Agenova adapter is tested.
EOF
}

main() {
  SUBCOMMAND="${1:-all}"
  local capture=false
  [ "$#" -le 2 ] || fail "too many arguments"
  case "${SUBCOMMAND}" in
    -h|--help) usage; return 0 ;;
    status|tools|up|smoke|teardown|down|all) ;;
    *) usage; fail "unknown subcommand: ${SUBCOMMAND}" ;;
  esac
  if [ "$#" -eq 2 ]; then
    [ "$2" = --capture ] || fail "unknown option: $2"
    case "${SUBCOMMAND}" in
      tools|status) fail "--capture is only available for mutating phases" ;;
    esac
    capture=true
  fi
  if [ "${capture}" = true ]; then
    mkdir -p "${ROOT}/.tmp"
    EVIDENCE_DIR="$(mktemp -d "${ROOT}/.tmp/agent-sandbox-evidence.XXXXXX")"
    info "capturing transient evidence in ${EVIDENCE_DIR}"
    exec > >(tee "${EVIDENCE_DIR}/output.txt") 2>&1
    trap 'write_summary $?' EXIT
  fi
  case "${SUBCOMMAND}" in
    status) cmd_status ;;
    tools) cmd_tools ;;
    up) cmd_up ;;
    smoke) cmd_smoke ;;
    teardown) cmd_teardown ;;
    down) cmd_down ;;
    all) cmd_up; cmd_smoke; cmd_teardown; cmd_down ;;
  esac
}

# Sourcing exposes functions to the isolated harness regression tests.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
