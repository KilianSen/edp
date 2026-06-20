# syntax=docker/dockerfile:1
# Headless edp image. Built from the MONOREPO ROOT as context (the edp service
# lives in ./edp), e.g. from the repo root:
#   docker build -f edp/Dockerfile -t edp .

# ---- build stage: static, CGO-free binary ----
# Pin to BUILDPLATFORM so the compiler always runs natively on the runner;
# cross-compile to the target arch via GOARCH instead of emulating under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY edp/go.mod edp/go.sum ./
RUN go mod download
COPY edp/ ./
# TARGETOS/TARGETARCH are provided automatically by buildx per target platform.
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /edp ./cmd/edp

# ---- runtime stage ----
# We shell out to docker/compose and git, so they must be present at runtime.
FROM alpine:3.20
# docker/compose + git for deploys; python3 runs lifecycle hooks.
RUN apk add --no-cache docker-cli docker-cli-compose git python3 ca-certificates tzdata
COPY --from=build /edp /usr/local/bin/edp

ENV EDP_ADDR=:8080 \
    EDP_DATA_DIR=/data \
    EDP_WORKSPACE_DIR=/workspace
VOLUME ["/data", "/workspace"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/edp"]
