# Test-only derivative, not a required Agent base image or protocol change.
# Build from this directory with --build-arg WORKER_IMAGE=<reviewed repo@sha256>.
ARG WORKER_IMAGE
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
ARG WORKER_IMAGE
ARG TARGETARCH
WORKDIR /src
COPY main.go ./
RUN digest="${WORKER_IMAGE##*@sha256:}"; test "${#digest}" -eq 64 && \
    case "$digest" in *[!0-9a-f]*) exit 1;; esac && \
    case "$WORKER_IMAGE" in *@sha256:*) ;; *) exit 1;; esac && \
    CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH:-amd64}" go build -trimpath -ldflags="-s -w" -o /out/agenova-netprobe main.go

FROM ${WORKER_IMAGE}
COPY --from=build /out/agenova-netprobe /agenova-netprobe
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# Inherit the reviewed Worker's USER; do not change its tmp/socket permissions.
ENTRYPOINT ["/agenova-netprobe"]
