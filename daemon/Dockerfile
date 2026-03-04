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

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Games Dashboard Daemon"
LABEL org.opencontainers.image.description="Backend daemon for the Gaming Server Dashboard"
LABEL org.opencontainers.image.source="https://github.com/games-dashboard/games-dashboard"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=builder /out/games-daemon /games-daemon

# TLS certs and config are mounted at runtime
VOLUME ["/etc/games-dashboard", "/var/lib/games-dashboard"]

EXPOSE 8443

USER nonroot:nonroot
ENTRYPOINT ["/games-daemon"]
CMD ["--log-level", "info"]
