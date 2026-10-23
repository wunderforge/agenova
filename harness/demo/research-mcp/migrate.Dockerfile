# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Trusted operator job, never an Agent or control-plane runtime image.
FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY internal ./internal
COPY tools/product-release-migrate ./tools/product-release-migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /migrate ./tools/product-release-migrate
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /migrate /migrate
USER 65532:65532
ENTRYPOINT ["/migrate"]
