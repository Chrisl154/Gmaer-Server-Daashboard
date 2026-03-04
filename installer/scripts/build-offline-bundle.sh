#!/usr/bin/env bash
# =============================================================================
# build-offline-bundle.sh — Build a self-contained offline installation bundle.
#
# Requires internet access and:
#   docker, helm, go (for CLI cross-compilation), sha256sum, curl, jq
#
# Usage:
#   VERSION=1.0.0 ARCH=amd64 installer/scripts/build-offline-bundle.sh \
#     --output /tmp/bundles
#
# Produces:
#   <output>/games-dashboard-offline-<VERSION>-<ARCH>.tar.gz
#   <output>/games-dashboard-offline-<VERSION>-<ARCH>.tar.gz.sha256
#   <output>/games-dashboard-offline-<VERSION>-<ARCH>.tar.gz.asc   (if GPG key set)
# =============================================================================
set -euo pipefail

VERSION="${VERSION:-1.0.0}"
ARCH="${ARCH:-amd64}"
OS="${OS:-linux}"
OUTPUT_DIR="${OUTPUT_DIR:-/tmp/games-dashboard-bundles}"
GPG_KEY_ID="${GPG_KEY_ID:-}"   # Optional: GPG key fingerprint for signing

BUNDLE_NAME="games-dashboard-offline-${VERSION}-${ARCH}"
WORK_DIR="/tmp/${BUNDLE_NAME}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# Image tags
DAEMON_IMAGE="ghcr.io/games-dashboard/daemon:${VERSION}"
UI_IMAGE="ghcr.io/games-dashboard/ui:${VERSION}"
PROMETHEUS_IMAGE="prom/prometheus:v2.49.1"
GRAFANA_IMAGE="grafana/grafana:10.3.1"

# Colors
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'; BOLD='\033[1m'
log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
log_step()  { echo -e "\n${BOLD}── $* ──${NC}"; }

# =============================================================================
# Dependency check
# =============================================================================
check_deps() {
  log_step "Checking build dependencies"
  local missing=()
  for cmd in docker helm go sha256sum curl jq tar; do
    if command -v "$cmd" &>/dev/null; then
      log_info "$cmd ✓"
    else
      log_error "$cmd not found"
      missing+=("$cmd")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "Missing dependencies: ${missing[*]}"
    exit 1
  fi
}

# =============================================================================
# Argument parsing
# =============================================================================
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --output) OUTPUT_DIR="$2"; shift 2 ;;
      --version) VERSION="$2"; BUNDLE_NAME="games-dashboard-offline-${VERSION}-${ARCH}"; shift 2 ;;
      --arch) ARCH="$2"; BUNDLE_NAME="games-dashboard-offline-${VERSION}-${ARCH}"; shift 2 ;;
      --gpg-key) GPG_KEY_ID="$2"; shift 2 ;;
      --help|-h)
        echo "Usage: VERSION=x.y.z ARCH=amd64 build-offline-bundle.sh [--output DIR] [--gpg-key KEYID]"
        exit 0 ;;
      *) log_error "Unknown option: $1"; exit 1 ;;
    esac
  done
}

# =============================================================================
# Setup working directory
# =============================================================================
setup_workdir() {
  log_step "Setting up work directory"
  rm -rf "$WORK_DIR"
  mkdir -p "$WORK_DIR"/{images,helm,cli,steamcmd,adapters}
  log_info "Work dir: $WORK_DIR"
}

# =============================================================================
# Pull and export Docker images
# =============================================================================
export_images() {
  log_step "Exporting Docker images"

  declare -A IMAGES=(
    ["daemon"]="$DAEMON_IMAGE"
    ["ui"]="$UI_IMAGE"
    ["prometheus"]="$PROMETHEUS_IMAGE"
    ["grafana"]="$GRAFANA_IMAGE"
  )

  for name in "${!IMAGES[@]}"; do
    local image="${IMAGES[$name]}"
    log_info "Pulling $image ..."
    docker pull --platform "linux/${ARCH}" "$image"

    log_info "Saving $image → images/${name}.tar"
    docker save "$image" -o "$WORK_DIR/images/${name}.tar"
    log_info "  Size: $(du -sh "$WORK_DIR/images/${name}.tar" | cut -f1)"
  done
}

# =============================================================================
# Package Helm charts
# =============================================================================
package_helm_charts() {
  log_step "Packaging Helm charts"

  for chart in games-dashboard game-instance; do
    local chart_path="$REPO_ROOT/helm/charts/$chart"
    if [[ -d "$chart_path" ]]; then
      helm package "$chart_path" --destination "$WORK_DIR/helm" --version "$VERSION"
      log_info "Packaged $chart"
    else
      log_warn "Chart not found: $chart_path"
    fi
  done
}

# =============================================================================
# Build CLI binaries
# =============================================================================
build_cli_binaries() {
  log_step "Building gdash CLI binaries"

  local cli_dir="$REPO_ROOT/cli"
  if [[ ! -f "$cli_dir/go.mod" ]]; then
    log_warn "cli/go.mod not found — skipping CLI build"
    return
  fi

  local targets=("amd64" "arm64")
  for target_arch in "${targets[@]}"; do
    local output="$WORK_DIR/cli/gdash-linux-${target_arch}"
    log_info "Building gdash-linux-${target_arch} ..."
    (
      cd "$cli_dir"
      GOOS=linux GOARCH="$target_arch" CGO_ENABLED=0 \
        go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
        -o "$output" ./cmd
    )
    chmod +x "$output"
    log_info "  Size: $(du -sh "$output" | cut -f1)"
  done
}

# =============================================================================
# Bundle SteamCMD
# =============================================================================
bundle_steamcmd() {
  log_step "Bundling SteamCMD"

  local steamcmd_url="https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz"
  local dest="$WORK_DIR/steamcmd/steamcmd.tar.gz"

  log_info "Downloading SteamCMD ..."
  curl -fsSL "$steamcmd_url" -o "$dest"
  log_info "  Size: $(du -sh "$dest" | cut -f1)"
}

# =============================================================================
# Copy adapter manifests
# =============================================================================
copy_adapters() {
  log_step "Copying adapter manifests"

  local adapters_dir="$REPO_ROOT/adapters"
  if [[ -d "$adapters_dir" ]]; then
    cp "$adapters_dir"/*.yaml "$WORK_DIR/adapters/" 2>/dev/null || true
    local count
    count=$(find "$WORK_DIR/adapters" -name "*.yaml" | wc -l)
    log_info "Copied $count adapter manifests"
  else
    log_warn "Adapters directory not found: $adapters_dir"
  fi
}

# =============================================================================
# Generate manifest.json with checksums
# =============================================================================
generate_manifest() {
  log_step "Generating manifest.json"

  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  local builder
  builder="${CI_JOB_NAME:-$(hostname)}"

  # Build artifacts JSON
  local artifacts_json="{}"
  while IFS= read -r -d '' file; do
    local rel="${file#$WORK_DIR/}"
    local size
    size=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null || echo 0)
    local checksum
    checksum=$(sha256sum "$file" | cut -d' ' -f1)
    artifacts_json=$(echo "$artifacts_json" | jq \
      --arg k "$rel" \
      --argjson s "$size" \
      --arg c "$checksum" \
      '.[$k] = {"size": $s, "sha256": $c}')
  done < <(find "$WORK_DIR" -type f -not -name "manifest.json" -print0 | sort -z)

  jq -n \
    --arg version "$VERSION" \
    --arg arch "$ARCH" \
    --arg os "$OS" \
    --arg ts "$timestamp" \
    --arg builder "$builder" \
    --argjson artifacts "$artifacts_json" \
    '{
      version: $version,
      arch: $arch,
      os: $os,
      created_at: $ts,
      builder: $builder,
      artifacts: $artifacts
    }' > "$WORK_DIR/manifest.json"

  log_info "manifest.json written ($(wc -l < "$WORK_DIR/manifest.json") lines)"
}

# =============================================================================
# Create archive and sidecar files
# =============================================================================
create_archive() {
  log_step "Creating archive"

  mkdir -p "$OUTPUT_DIR"
  local archive="$OUTPUT_DIR/${BUNDLE_NAME}.tar.gz"

  log_info "Compressing → $archive ..."
  tar -czf "$archive" -C "$(dirname "$WORK_DIR")" "$(basename "$WORK_DIR")"

  local size
  size=$(du -sh "$archive" | cut -f1)
  log_info "Archive size: $size"

  # SHA-256 sidecar
  local sha256_file="${archive}.sha256"
  sha256sum "$archive" > "$sha256_file"
  log_info "SHA-256:  $sha256_file"

  # Optional GPG signature
  if [[ -n "$GPG_KEY_ID" ]]; then
    gpg --default-key "$GPG_KEY_ID" --armor --detach-sign "$archive"
    log_info "GPG sig:  ${archive}.asc"
  fi

  echo ""
  log_info "${BOLD}Bundle ready:${NC}"
  log_info "  Archive:    $archive"
  log_info "  SHA-256:    $sha256_file"
  [[ -n "$GPG_KEY_ID" ]] && log_info "  GPG sig:    ${archive}.asc"
}

# =============================================================================
# Cleanup
# =============================================================================
cleanup() {
  log_info "Cleaning up work directory ..."
  rm -rf "$WORK_DIR"
}

# =============================================================================
# Main
# =============================================================================
main() {
  parse_args "$@"

  echo ""
  echo -e "${BOLD}Games Dashboard — Offline Bundle Builder${NC}"
  echo "  Version : $VERSION"
  echo "  Arch    : $ARCH"
  echo "  Output  : $OUTPUT_DIR"
  echo ""

  check_deps
  setup_workdir
  export_images
  package_helm_charts
  build_cli_binaries
  bundle_steamcmd
  copy_adapters
  generate_manifest
  create_archive
  cleanup

  echo ""
  log_info "Build complete."
}

main "$@"
