# syntax=docker/dockerfile:1
# edp-manager: the API + the bundled dashboard (./ui), served at "/" via
# EDPM_UI_DIR. The manager IS the control-plane UI, so there's no headless image
# — the UI always ships. (EDPM_UI_DIR stays a knob so local dev can run API-only
# and serve the UI from Vite separately.)

# ---- ui build: compile the React SPA (in ./ui) to static files ----
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json* ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# ---- go build: static, CGO-free binary ----
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /edp-manager ./cmd/edp-manager

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /edp-manager /usr/local/bin/edp-manager
COPY --from=ui /ui/dist /usr/share/edp-manager-ui
ENV EDPM_ADDR=:9090 \
    EDPM_DATA_DIR=/data \
    EDPM_UI_DIR=/usr/share/edp-manager-ui
VOLUME ["/data"]
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/edp-manager"]
