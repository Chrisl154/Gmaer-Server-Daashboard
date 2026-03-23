# ── Build stage ─────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/games-daemon \
    ./cmd/daemon

# ── Docker CLI stage — pull the static Docker CLI binary ──────────────────────
FROM docker:27-cli AS docker-cli

# ── Runtime stage ────────────────────────────────────────────────────────────
# Use debian-slim instead of distroless so we have sh + docker CLI available.
# The daemon shells out to `docker` for container lifecycle management.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

LABEL org.opencontainers.image.title="Games Dashboard Daemon"
LABEL org.opencontainers.image.description="Backend daemon for the Gaming Server Dashboard"
LABEL org.opencontainers.image.source="https://github.com/games-dashboard/games-dashboard"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=builder /out/games-daemon /games-daemon

# TLS certs and config are mounted at runtime
VOLUME ["/etc/games-dashboard", "/var/lib/games-dashboard"]

EXPOSE 8443

# Create nonroot user and add to docker group (GID 999) so it can reach the host socket.
# The docker group GID must match the host's /var/run/docker.sock group (typically 999).
RUN groupadd -g 999 docker || true && \
    groupadd -g 65532 nonroot && \
    useradd -u 65532 -g nonroot -G docker -s /bin/false nonroot && \
    mkdir -p /etc/games-dashboard /var/lib/games-dashboard && \
    chown -R nonroot:nonroot /etc/games-dashboard /var/lib/games-dashboard
USER nonroot

ENTRYPOINT ["/games-daemon"]
CMD ["--log-level", "info"]
