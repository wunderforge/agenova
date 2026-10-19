# syntax=docker/dockerfile:1
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Build deploy/reference/Dockerfile first. Supply its immutable image reference.
ARG CONTROL_PLANE_IMAGE
FROM ${CONTROL_PLANE_IMAGE}
COPY ui/dist /opt/agenova/portal
