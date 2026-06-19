# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
# Branding lives once in assets/branding; mirror task sync-branding here since
# the generated static copies are gitignored and absent from the build context.
COPY assets/branding/sampo-favicon.svg ./static/favicon.svg
COPY assets/branding/sampo-banner.svg ./static/sampo-banner.svg
RUN pnpm run build

FROM golang:1.26-bookworm AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/build ./frontend/build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.gitHash=${GIT_HASH} -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /out/sampo \
    ./cmd/sampo

# Download a pinned ONNX Runtime shared library for the target arch.
FROM debian:bookworm-slim AS ort-downloader
ARG TARGETARCH
ARG ORT_VERSION=1.26.0
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl tar \
    && rm -rf /var/lib/apt/lists/*
RUN case "${TARGETARCH}" in \
        amd64) ORT_ARCH=x64 ;; \
        arm64) ORT_ARCH=aarch64 ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}.tgz" \
        -o /tmp/ort.tgz \
    && mkdir -p /tmp/ort \
    && tar -xzf /tmp/ort.tgz -C /tmp/ort --strip-components=1 \
    && mkdir -p /opt/onnxruntime \
    && cp -a /tmp/ort/lib/. /opt/onnxruntime/

FROM scratch AS binary-export
COPY --from=go-builder /out/sampo /sampo

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates ffmpeg poppler-utils tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -u 1000 -m -d /home/sampo -s /usr/sbin/nologin sampo
WORKDIR /app

# ONNX Runtime shared library (resolved via ORT_LIB_PATH below).
COPY --from=ort-downloader /opt/onnxruntime/ /opt/onnxruntime/
ENV ORT_LIB_PATH=/opt/onnxruntime/libonnxruntime.so

# Least-volatile layers first so backend/frontend rebuilds don't bust the
# big model layer. Order: models (rarely change) -> config -> frontend -> binary.
# Bake ML models into the image so it runs without external mounts.
COPY --chown=sampo:sampo models/ ./models/
COPY --chown=sampo:sampo config.docker.yaml ./config.yaml
COPY --chown=sampo:sampo --from=frontend-builder /app/frontend/build ./frontend/build
COPY --chown=sampo:sampo --from=go-builder /out/sampo ./sampo
RUN mkdir -p /cache /data \
    && chown sampo:sampo /app /cache /data
USER sampo
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
CMD ["./sampo"]
