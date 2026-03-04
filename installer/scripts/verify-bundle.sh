#!/usr/bin/env bash
# =============================================================================
# verify-bundle.sh — Verify integrity of a Games Dashboard offline bundle.
#
# Usage:
#   installer/scripts/verify-bundle.sh <bundle.tar.gz>
#
# Exit codes:
#   0 — all checks passed
#   1 — archive checksum mismatch or artifact tampering detected
#   2 — usage error or missing dependencies
# =============================================================================
set -euo pipefail

# Colors
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'; BOLD='\033[1m'
pass()  { PASS=$((PASS+1)); echo -e "  ${GREEN}✓${NC} $*"; }
fail()  { FAIL=$((FAIL+1)); echo -e "  ${RED}✗${NC} $*" >&2; }
warn()  { echo -e "  ${YELLOW}!${NC} $*"; }
section() { echo -e "\n${BOLD}── $* ──${NC}"; }

PASS=0; FAIL=0

# =============================================================================
# Dependency check
# =============================================================================
check_deps() {
  for cmd in sha256sum tar jq; do
    if ! command -v "$cmd" &>/dev/null; then
      echo -e "${RED}ERROR${NC}: required command not found: $cmd" >&2
      exit 2
    fi
  done
}

# =============================================================================
# Usage
# =============================================================================
usage() {
  cat << EOF
Usage: verify-bundle.sh <bundle.tar.gz>

Verifies a Games Dashboard offline bundle:
  1. Outer archive SHA-256 against <bundle.tar.gz>.sha256 sidecar
  2. GPG signature against <bundle.tar.gz>.asc (if present)
  3. Each artifact's SHA-256 against manifest.json entries

EOF
}

# =============================================================================
# Main
# =============================================================================
main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 2
  fi

  local archive="$1"

  if [[ ! -f "$archive" ]]; then
    echo -e "${RED}ERROR${NC}: file not found: $archive" >&2
    exit 2
  fi

  check_deps

  echo -e "${BOLD}Games Dashboard — Bundle Verifier${NC}"
  echo "  Archive: $archive"
  echo ""

  # ── 1. Archive checksum ───────────────────────────────────────────────────
  section "Archive integrity"

  local sha256_file="${archive}.sha256"
  if [[ -f "$sha256_file" ]]; then
    if sha256sum --check "$sha256_file" --status 2>/dev/null; then
      pass "Archive SHA-256 matches sidecar file"
    else
      fail "Archive SHA-256 MISMATCH — archive may be corrupted or tampered"
      echo ""
      echo "  Expected: $(cat "$sha256_file")"
      echo "  Actual:   $(sha256sum "$archive")"
    fi
  else
    warn "No .sha256 sidecar found — skipping archive checksum"
  fi

  # ── 2. GPG signature ─────────────────────────────────────────────────────
  local asc_file="${archive}.asc"
  if [[ -f "$asc_file" ]]; then
    section "GPG signature"
    if command -v gpg &>/dev/null; then
      if gpg --verify "$asc_file" "$archive" 2>/dev/null; then
        pass "GPG signature valid"
      else
        fail "GPG signature INVALID"
      fi
    else
      warn "gpg not installed — skipping signature verification"
    fi
  fi

  # ── 3. Artifact checksums ─────────────────────────────────────────────────
  section "Artifact checksums"

  local extract_dir
  extract_dir=$(mktemp -d)
  trap 'rm -rf "$extract_dir"' EXIT

  log_info() { echo "  [info] $*"; }
  log_info "Extracting bundle to $extract_dir ..."
  tar -xzf "$archive" -C "$extract_dir" 2>/dev/null

  # Locate the extracted bundle root (single top-level directory)
  local bundle_root
  bundle_root=$(find "$extract_dir" -mindepth 1 -maxdepth 1 -type d | head -1)

  if [[ -z "$bundle_root" ]]; then
    fail "Bundle extraction yielded no directory"
    echo ""
    echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
    exit 1
  fi

  local manifest="$bundle_root/manifest.json"
  if [[ ! -f "$manifest" ]]; then
    fail "manifest.json not found in bundle root"
    echo ""
    echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
    exit 1
  fi

  pass "manifest.json found"

  # Validate manifest schema fields
  local version arch os_field created_at
  version=$(jq -r '.version // empty' "$manifest")
  arch=$(jq -r '.arch // empty' "$manifest")
  os_field=$(jq -r '.os // empty' "$manifest")
  created_at=$(jq -r '.created_at // empty' "$manifest")

  if [[ -n "$version" && -n "$arch" && -n "$os_field" && -n "$created_at" ]]; then
    pass "Manifest schema valid (version=$version arch=$arch os=$os_field)"
  else
    fail "Manifest missing required fields (version, arch, os, created_at)"
  fi

  # Check each artifact listed in manifest
  local total_artifacts=0 matched=0 mismatched=0 missing_files=0

  while IFS="=" read -r rel_path entry; do
    total_artifacts=$((total_artifacts+1))
    local expected_sha256
    expected_sha256=$(echo "$entry" | jq -r '.sha256 // empty')
    local artifact_path="$bundle_root/$rel_path"

    if [[ ! -f "$artifact_path" ]]; then
      fail "Missing artifact: $rel_path"
      missing_files=$((missing_files+1))
      continue
    fi

    if [[ -z "$expected_sha256" ]]; then
      warn "No checksum for $rel_path — skipping"
      continue
    fi

    local actual_sha256
    actual_sha256=$(sha256sum "$artifact_path" | cut -d' ' -f1)

    if [[ "$actual_sha256" == "$expected_sha256" ]]; then
      pass "$rel_path"
      matched=$((matched+1))
    else
      fail "$rel_path — SHA-256 MISMATCH"
      echo "    Expected: $expected_sha256"
      echo "    Actual:   $actual_sha256"
      mismatched=$((mismatched+1))
    fi
  done < <(jq -r '.artifacts | to_entries[] | "\(.key)=\(.value)"' "$manifest")

  # ── Results ───────────────────────────────────────────────────────────────
  echo ""
  echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
  echo "    Artifacts: ${total_artifacts} total — ${matched} verified, ${mismatched} mismatched, ${missing_files} missing"
  echo ""

  if [[ $FAIL -eq 0 ]]; then
    echo -e "${GREEN}Bundle OK — ready for offline installation.${NC}"
    exit 0
  else
    echo -e "${RED}Bundle FAILED verification — do not use this bundle.${NC}" >&2
    exit 1
  fi
}

main "$@"
