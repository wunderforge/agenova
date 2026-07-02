# Deployment Model

This document records the target deployment and release model for Agenova. It is product-level context, not a current phase evidence gate.

## Target User Path

Agenova Runtime should be consumed as a Kubernetes-native runtime package, not as source code alone.

Recommended path for a Kubernetes operator:

1. Prepare a Kubernetes cluster and the selected execution backend, such as Kubernetes SIG Apps Agent Sandbox when using the Agent Sandbox adapter.
2. Install Agenova Runtime with a Helm chart or operator bundle.
3. Create reusable agent role and scoped assignment resources through YAML, `agenovactl`, or later product APIs.
4. Observe assignment status, claim facts, gateway decisions, and runtime evidence through Agenova surfaces.

## Release Artifacts

A production release should publish these artifacts together:

- GitHub source code for development, audit, and contribution.
- Go binaries for local tools and release assets, including `agenovactl`.
- Container images for in-cluster components such as the operator and gateways.
- CRD YAML generated from Agenova API types.
- Helm chart or operator bundle for Kubernetes installation.
- Example YAML for common roles, assignments, runtime profiles, and policies.
- Install and upgrade documentation.

## Kubernetes-Native Runtime Package

The Kubernetes-native `Agenova Runtime` distribution should install:

- Agenova CRDs.
- Controller manager / operator deployment.
- RuntimeBackend adapter configuration.
- Tool Gateway and Model Gateway services when enabled by the release phase.
- RBAC, service accounts, config maps, and optional webhooks.
- Sample resources for smoke testing.

The user-facing install command should eventually look like:

```bash
helm install agenova agenova/agenova-runtime \
  --namespace agenova-system \
  --create-namespace
```

The chart may assume that the selected execution backend is already installed, or it may expose explicit values for backend integration.

## How Runtime Components Relate

```text
api/v1alpha1/*.go
  -> generated CRD YAML
  -> installed by Helm/operator bundle
  -> used by operators and automation to create Agenova resources

cmd/agenova-operator
  -> controller image
  -> watches Agenova resources
  -> reconciles assignments through RuntimeBackend

internal/runtime/agentsandbox
  -> compiled into the operator or adapter component
  -> maps Agenova runtime semantics to Kubernetes Agent Sandbox resources

cmd/agenova-tool-gateway and cmd/agenova-model-gateway
  -> gateway images
  -> mediate claim-scoped tool/model access
  -> record facts under the active claim

cmd/agenovactl
  -> CLI binary
  -> creates, inspects, and debugs Agenova resources
```

## Current Repository State

The repository may contain scaffolds before this deployment model is fully implemented. Do not treat this document as proof that Helm charts, CRD generation, Docker images, or operator bundles already exist.

Implementation of packaging, publishing, and install smoke tests must remain phase-scoped and evidence-backed.
