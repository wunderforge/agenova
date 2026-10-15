#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

mode="${1:-verify}"
platform_file="${AGENOVA_PLATFORM_FILE:-deploy/nonproduction/platform.yaml}"
state_dir="${AGENOVA_STATE_DIR:-.tmp/nonproduction-state}"
agenova_bin="${AGENOVA_BIN:-./.tmp/agenova}"
rendered_file="${AGENOVA_RENDERED_FILE:-.tmp/nonproduction-rendered.yaml}"

case "${mode}" in
  validate|render|prerequisites|plan|apply|verify) ;;
  *) echo "usage: $0 [validate|render|prerequisites|plan|apply|verify]" >&2; exit 2 ;;
esac

"${agenova_bin}" platform validate -f "${platform_file}" --json
if [[ "${mode}" == "validate" ]]; then
  exit 0
fi
if [[ "${mode}" == "render" ]]; then
  mkdir -p "$(dirname "${rendered_file}")"
  "${agenova_bin}" platform render -f "${platform_file}" > "${rendered_file}"
  printf '%s\n' "${rendered_file}"
  exit 0
fi

expected_context="${AGENOVA_EXPECTED_CONTEXT:?set AGENOVA_EXPECTED_CONTEXT to the exact non-production kube context}"
expected_namespace="${AGENOVA_EXPECTED_NAMESPACE:?set AGENOVA_EXPECTED_NAMESPACE to the dedicated namespace}"
if [[ "${expected_context}" =~ (^|[-_.])(prod|prd|production)($|[-_.]) ]]; then
  echo "refusing a context that looks like production: ${expected_context}" >&2
  exit 2
fi

actual_context="$(kubectl config current-context)"
if [[ "${actual_context}" != "${expected_context}" ]]; then
  echo "current context ${actual_context} does not match ${expected_context}" >&2
  exit 2
fi

require_api_resource() {
  local resources="$1"
  local required="$2"
  if ! grep -Fxq "${required}" <<<"${resources}"; then
    echo "required Agent Sandbox API resource is unavailable: ${required}" >&2
    exit 2
  fi
}

extension_resources="$(kubectl --context "${expected_context}" api-resources --api-group=extensions.agents.x-k8s.io -o name)"
core_resources="$(kubectl --context "${expected_context}" api-resources --api-group=agents.x-k8s.io -o name)"
require_api_resource "${extension_resources}" "sandboxclaims.extensions.agents.x-k8s.io"
require_api_resource "${extension_resources}" "sandboxtemplates.extensions.agents.x-k8s.io"
require_api_resource "${extension_resources}" "sandboxwarmpools.extensions.agents.x-k8s.io"
require_api_resource "${core_resources}" "sandboxes.agents.x-k8s.io"

controller_namespace="${AGENOVA_AGENT_SANDBOX_CONTROLLER_NAMESPACE:-agent-sandbox-system}"
controller_name="${AGENOVA_AGENT_SANDBOX_CONTROLLER_NAME:-agent-sandbox-controller}"
expected_controller_image="${AGENOVA_EXPECTED_AGENT_SANDBOX_CONTROLLER_IMAGE:?set AGENOVA_EXPECTED_AGENT_SANDBOX_CONTROLLER_IMAGE to the reviewed v0.4.6 controller image}"
actual_controller_images="$(kubectl --context "${expected_context}" --namespace "${controller_namespace}" get deployment "${controller_name}" -o jsonpath='{range .spec.template.spec.containers[*]}{.image}{"\n"}{end}')"
if ! grep -Fxq "${expected_controller_image}" <<<"${actual_controller_images}"; then
  echo "Agent Sandbox controller image does not match the reviewed v0.4.6 image" >&2
  exit 2
fi
available_replicas="$(kubectl --context "${expected_context}" --namespace "${controller_namespace}" get deployment "${controller_name}" -o jsonpath='{.status.availableReplicas}')"
if [[ -z "${available_replicas}" || "${available_replicas}" == "0" ]]; then
  echo "Agent Sandbox controller ${controller_namespace}/${controller_name} has no available replica" >&2
  exit 2
fi
if [[ "${mode}" == "prerequisites" ]]; then
  printf 'Agent Sandbox substrate ready: %s/%s (%s available)\n' "${controller_namespace}" "${controller_name}" "${available_replicas}"
  exit 0
fi

plan="$("${agenova_bin}" platform plan -f "${platform_file}" --state-dir "${state_dir}" --json)"
printf '%s\n' "${plan}"
if [[ "${plan}" != *"\"target\":\"${expected_context}/${expected_namespace}\""* ]]; then
  echo "Platform target does not match the explicitly selected non-production target" >&2
  exit 2
fi
if [[ "${mode}" == "plan" ]]; then
  exit 0
fi

if [[ "${mode}" == "apply" ]]; then
  "${agenova_bin}" platform apply -f "${platform_file}" --state-dir "${state_dir}" --yes --json
fi

kubectl --context "${expected_context}" --namespace "${expected_namespace}" rollout status deployment/agenova-control-plane --timeout=120s
kubectl --context "${expected_context}" --namespace "${expected_namespace}" get serviceaccount/agenova-control-plane role/agenova-control-plane-runtime rolebinding/agenova-control-plane-runtime service/agenova-control-plane
kubectl --context "${expected_context}" get --raw "/api/v1/namespaces/${expected_namespace}/services/http:agenova-control-plane:8080/proxy/v1/status"

"${agenova_bin}" platform status --state-dir "${state_dir}" --json
