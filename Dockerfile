FROM node:22-bookworm-slim AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.25-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/subagg ./cmd/subagg

FROM ubuntu:22.04
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
