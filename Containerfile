# Stage 1: Build web frontend
FROM docker.io/oven/bun:1-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json web/bun.lock web/svelte.config.js ./
RUN bun install --frozen-lockfile

COPY web/ .
RUN mkdir -p /src/pkg/webchat/static && bun run build

# Stage 2: Build Go binary
FROM docker.io/library/golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/pkg/webchat/static/ ./pkg/webchat/static/
RUN CGO_ENABLED=0 go build -v -ldflags "-X main.version=container" -o /src/build/localagent ./cmd

# Stage 3: Runtime
FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 localagent \
    && adduser -u 1000 -G localagent -D -h /home/localagent localagent

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:18790/health || exit 1

COPY --from=builder /src/build/localagent /usr/local/bin/localagent
COPY deploy/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER localagent

RUN /usr/local/bin/localagent onboard

EXPOSE 18790 18791

ENTRYPOINT ["entrypoint.sh"]
CMD ["gateway"]
