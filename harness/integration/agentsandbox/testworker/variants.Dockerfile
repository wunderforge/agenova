# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Local regression artifacts only, not published by platform CI/CD.
ARG BASE_IMAGE=agenova-worker-240:a
FROM ${BASE_IMAGE} AS variant
LABEL io.agenova.test-variant="independent-application-image"

FROM alpine:3.20 AS incompatible
USER 65532:65532
ENTRYPOINT ["/bin/sh", "-c", "sleep 600"]
