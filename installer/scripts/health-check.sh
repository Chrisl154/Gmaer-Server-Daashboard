#!/usr/bin/env bash
# health-check.sh — Verify that the daemon is healthy after installation.
# Usage: health-check.sh [<daemon-url>]
set -euo pipefail

DAEMON_URL="${1:-https://localhost:8443}"
MAX_RETRIES="${MAX_RETRIES:-30}"
RETRY_SLEEP="${RETRY_SLEEP:-2}"

echo "[health-check] Waiting for daemon at $DAEMON_URL ..."

for i in $(seq 1 "$MAX_RETRIES"); do
  if curl -fsk --max-time 5 "$DAEMON_URL/healthz" > /tmp/health-response.json 2>/dev/null; then
    echo "[health-check] Daemon is healthy!"
    cat /tmp/health-response.json
    exit 0
  fi
  echo "[health-check] Attempt $i/$MAX_RETRIES — not ready yet..."
  sleep "$RETRY_SLEEP"
done

echo "[health-check] ERROR: Daemon did not become healthy after $MAX_RETRIES attempts." >&2
echo "[health-check] Check daemon logs: docker logs games-dashboard-daemon" >&2
exit 1
