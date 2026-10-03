# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# E16 acceptance probe image. Build from the repository root:
#   docker build -f harness/integration/e16/probe.Dockerfile -t agenova-e16-probe:<source-sha> .
# The test binary is compiled with the agenovaprobe tag; the installed control
# plane image never is. Bases are pinned by digest.
FROM golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY api ./api
COPY internal ./internal
COPY cmd/agenova-control-plane ./cmd/agenova-control-plane
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=local go test -c -tags agenovaprobe -trimpath -o /out/e16-probe.test ./cmd/agenova-control-plane

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/e16-probe.test /e16-probe.test
USER nonroot:nonroot
ENTRYPOINT ["/e16-probe.test", "-test.run", "^TestE16Probes$", "-test.v", "-test.count=1"]
