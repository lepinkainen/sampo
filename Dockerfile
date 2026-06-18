# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS go-builder
RUN apk add --no-cache ca-certificates gcc git musl-dev tzdata
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

FROM scratch AS binary-export
COPY --from=go-builder /out/sampo /sampo

FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg tzdata wget \
    && adduser -D -u 1000 sampo
WORKDIR /app
COPY --from=go-builder /out/sampo ./sampo
COPY --from=frontend-builder /app/frontend/build ./frontend/build
COPY config.docker.yaml ./config.yaml
RUN mkdir -p /cache /data /app/models \
    && chown -R sampo:sampo /app /cache /data
USER sampo
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
CMD ["./sampo"]
