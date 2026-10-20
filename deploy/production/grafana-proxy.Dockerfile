# syntax=docker/dockerfile:1
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY internal ./internal
COPY cmd/agenova-grafana-proxy ./cmd/agenova-grafana-proxy
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agenova-grafana-proxy ./cmd/agenova-grafana-proxy
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/agenova-grafana-proxy /agenova-grafana-proxy
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/agenova-grafana-proxy"]
