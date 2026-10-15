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
  validate|render|plan|apply|verify) ;;
  *) echo "usage: $0 [validate|render|plan|apply|verify]" >&2; exit 2 ;;
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
