#!/usr/bin/env bash
# generate-tls.sh — Generate a self-signed TLS certificate for the daemon.
# Usage: generate-tls.sh <output-dir> [<hostname>]
set -euo pipefail

OUTPUT_DIR="${1:-/etc/games-dashboard/tls}"
HOSTNAME="${2:-localhost}"
DAYS="${TLS_DAYS:-3650}"   # 10 years default

mkdir -p "$OUTPUT_DIR"

echo "[generate-tls] Generating self-signed certificate for '$HOSTNAME' (${DAYS} days)..."

openssl req -x509 \
  -newkey rsa:4096 \
  -keyout "$OUTPUT_DIR/server.key" \
  -out    "$OUTPUT_DIR/server.crt" \
  -sha256 \
  -days   "$DAYS" \
  -nodes \
  -subj   "/CN=${HOSTNAME}/O=Games Dashboard/OU=Self-Signed" \
  -addext "subjectAltName=DNS:${HOSTNAME},DNS:localhost,IP:127.0.0.1"

chmod 600 "$OUTPUT_DIR/server.key"
chmod 644 "$OUTPUT_DIR/server.crt"

echo "[generate-tls] Certificate written to:"
echo "  Cert: $OUTPUT_DIR/server.crt"
echo "  Key:  $OUTPUT_DIR/server.key"
echo ""
echo "Certificate fingerprint:"
openssl x509 -in "$OUTPUT_DIR/server.crt" -fingerprint -noout -sha256
