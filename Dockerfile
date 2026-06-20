# syntax=docker/dockerfile:1

# ---- build stage: static, CGO-free binary ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /edp ./cmd/edp

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
