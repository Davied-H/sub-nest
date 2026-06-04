FROM node:22-bookworm-slim AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.25-bookworm AS go-builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/subagg ./cmd/subagg

FROM ubuntu:22.04 AS mihomo-core
ARG TARGETARCH
ARG MIHOMO_VERSION=v1.19.26
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl gzip \
    && rm -rf /var/lib/apt/lists/* \
    && case "$TARGETARCH" in \
        amd64) artifact="mihomo-linux-amd64-compatible-${MIHOMO_VERSION}.gz" ;; \
        arm64) artifact="mihomo-linux-arm64-${MIHOMO_VERSION}.gz" ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/mihomo.gz "https://github.com/MetaCubeX/mihomo/releases/download/${MIHOMO_VERSION}/${artifact}" \
    && mkdir -p /out \
    && gzip -dc /tmp/mihomo.gz > /out/mihomo \
    && chmod +x /out/mihomo

FROM ubuntu:22.04 AS slim
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=go-builder /out/subagg /usr/local/bin/subagg
COPY --from=web-builder /src/web/dist ./web/dist
ENV SUBAGG_ADDR=:8080 \
    SUBAGG_DATA=/data/config.json \
    SUBAGG_STATIC=/app/web/dist
EXPOSE 8080
VOLUME ["/data"]
CMD ["subagg"]

FROM slim
COPY --from=mihomo-core /out/mihomo /usr/local/bin/mihomo
ENV SUBAGG_MIHOMO_BIN=/usr/local/bin/mihomo
