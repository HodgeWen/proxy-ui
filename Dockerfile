# Stage 1: Frontend build
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN corepack enable && corepack prepare bun@latest --activate && bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Stage 2: Backend build
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY --from=frontend /app/web/dist ./web/dist
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /s-ui ./cmd/server

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates

# sing-box: fixed version for reproducibility; override via build-arg
ARG SINGBOX_VERSION=1.12.21
ARG TARGETARCH
RUN case "$TARGETARCH" in \
      amd64) ARCH=amd64 ;; \
      arm64) ARCH=arm64 ;; \
      *) echo "Unsupported arch: $TARGETARCH" && exit 1 ;; \
    esac && \
    wget -qO- "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${ARCH}.tar.gz" | tar xz -C /tmp && \
    mv /tmp/sing-box-*/sing-box /usr/local/bin/sing-box && \
    chmod +x /usr/local/bin/sing-box && \
    rm -rf /tmp/sing-box-*

COPY --from=backend /s-ui /s-ui
COPY docker-entrypoint.sh /
ENTRYPOINT ["/docker-entrypoint.sh"]

WORKDIR /data
ENV CONFIG_PATH=/data/config.json DATA_DIR=/data SINGBOX_BINARY_PATH=/usr/local/bin/sing-box
