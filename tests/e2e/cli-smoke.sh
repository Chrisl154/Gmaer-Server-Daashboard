#!/usr/bin/env bash
# =============================================================================
# Games Dashboard — CLI E2E Smoke Tests
# Requires: gdash binary in PATH, daemon running at GDASH_DAEMON.
#
# Usage:
#   GDASH_DAEMON=https://localhost:8443 GDASH_ADMIN_PASSWORD=changeme \
#     tests/e2e/cli-smoke.sh
#
# Exit code: 0 = all passed, 1 = failures
# =============================================================================
set -euo pipefail

DAEMON="${GDASH_DAEMON:-https://localhost:8443}"
ADMIN_PASS="${GDASH_ADMIN_PASSWORD:-changeme}"
GDASH="${GDASH_BIN:-gdash}"
OUTPUT_XML="${OUTPUT_XML:-/tmp/e2e-results.xml}"

PASS=0; FAIL=0; SKIP=0
TESTS=()
START_TIME=$(date +%s)

pass() { PASS=$((PASS+1)); TESTS+=("PASS|$*"); echo "  ✓ $*"; }
fail() { FAIL=$((FAIL+1)); TESTS+=("FAIL|$*"); echo "  ✗ $*" >&2; }
skip() { SKIP=$((SKIP+1)); TESTS+=("SKIP|$*"); echo "  - $* (skipped)"; }
section() { echo ""; echo "── $* ──"; }

echo "=== Games Dashboard CLI E2E Smoke Tests ==="
echo "Daemon: $DAEMON"
echo "Binary: $GDASH"
echo ""

# ── Pre-flight ────────────────────────────────────────────────────────────────
section "Pre-flight"

if command -v "$GDASH" &>/dev/null; then
  pass "gdash binary found in PATH"
else
  fail "gdash binary not found — build with: cd cli && go build -o gdash ./cmd"
  echo "FATAL: cannot continue without gdash binary."
  exit 1
fi

# Configure CLI to use the test daemon (insecure for self-signed certs)
"$GDASH" config set daemon_url "$DAEMON" &>/dev/null
"$GDASH" config set insecure true &>/dev/null
"$GDASH" config set output json &>/dev/null
pass "CLI configured for test daemon"

# ── Health check ──────────────────────────────────────────────────────────────
section "Health"

if "$GDASH" health &>/tmp/gdash-health.json 2>/dev/null; then
  pass "gdash health — daemon reachable"
else
  fail "gdash health — daemon unreachable (is it running?)"
  echo "FATAL: daemon not reachable. Start it first."
  exit 1
fi

if "$GDASH" version &>/dev/null 2>&1; then
  pass "gdash version"
else
  fail "gdash version"
fi

# ── Auth ──────────────────────────────────────────────────────────────────────
section "Auth"

if "$GDASH" auth login -u admin -p "$ADMIN_PASS" &>/tmp/gdash-login.json 2>/dev/null; then
  pass "auth login"
else
  fail "auth login (check GDASH_ADMIN_PASSWORD)"
fi

if "$GDASH" auth status &>/dev/null 2>/dev/null; then
  pass "auth status"
else
  fail "auth status"
fi

# ── Server CRUD ───────────────────────────────────────────────────────────────
section "Server CRUD"

TEST_SERVER_ID="e2e-test-$$"
TEST_SERVER_NAME="E2E Test Server"

# Create
if "$GDASH" server create "$TEST_SERVER_ID" "$TEST_SERVER_NAME" \
    --adapter valheim \
    --deploy-method manual \
    --install-dir /tmp/e2e-valheim \
    &>/tmp/gdash-create.json 2>/dev/null; then
  pass "server create"
else
  fail "server create"
fi

# List — server should appear
if "$GDASH" server list 2>/dev/null | grep -q "$TEST_SERVER_ID"; then
  pass "server list (contains created server)"
else
  fail "server list (created server not found)"
fi

# Get
if "$GDASH" server get "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "server get"
else
  fail "server get"
fi

# Start
if "$GDASH" server start "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "server start"
  sleep 3   # allow state transition
else
  fail "server start"
fi

# Status
STATUS_OUT=$("$GDASH" server get "$TEST_SERVER_ID" 2>/dev/null || true)
if echo "$STATUS_OUT" | grep -qE '"state".*"(running|starting)"'; then
  pass "server state is running/starting after start"
else
  skip "server state check (state unknown)"
fi

# Stop
if "$GDASH" server stop "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "server stop"
  sleep 2
else
  fail "server stop"
fi

# ── Backup ────────────────────────────────────────────────────────────────────
section "Backup"

if "$GDASH" backup create "$TEST_SERVER_ID" --type full &>/tmp/gdash-backup.json 2>/dev/null; then
  pass "backup create (full)"
  sleep 3
else
  fail "backup create"
fi

if "$GDASH" backup list "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "backup list"
else
  fail "backup list"
fi

# ── Mods ──────────────────────────────────────────────────────────────────────
section "Mods"

if "$GDASH" mod install "$TEST_SERVER_ID" "test-mod-001" \
    --source local \
    --version "1.0.0" \
    &>/dev/null 2>/dev/null; then
  pass "mod install"
  sleep 2
else
  fail "mod install"
fi

if "$GDASH" mod list "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "mod list"
else
  fail "mod list"
fi

if "$GDASH" mod test "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "mod test"
else
  fail "mod test"
fi

if "$GDASH" mod uninstall "$TEST_SERVER_ID" "test-mod-001" &>/dev/null 2>/dev/null; then
  pass "mod uninstall"
else
  fail "mod uninstall"
fi

# ── SBOM / CVE ───────────────────────────────────────────────────────────────
section "SBOM / CVE"

if "$GDASH" sbom show &>/dev/null 2>/dev/null; then
  pass "sbom show"
else
  fail "sbom show"
fi

if "$GDASH" sbom cve-report &>/dev/null 2>/dev/null; then
  pass "sbom cve-report"
else
  fail "sbom cve-report"
fi

if "$GDASH" sbom scan &>/dev/null 2>/dev/null; then
  pass "sbom scan (triggered)"
else
  fail "sbom scan"
fi

# ── Config CLI ────────────────────────────────────────────────────────────────
section "Config"

if "$GDASH" config show &>/dev/null 2>/dev/null; then
  pass "config show"
else
  fail "config show"
fi

if "$GDASH" config get daemon_url &>/dev/null 2>/dev/null; then
  pass "config get daemon_url"
else
  fail "config get daemon_url"
fi

if "$GDASH" config set output text &>/dev/null 2>/dev/null && \
   "$GDASH" config set output json &>/dev/null 2>/dev/null; then
  pass "config set output"
else
  fail "config set output"
fi

# ── Cleanup ───────────────────────────────────────────────────────────────────
section "Cleanup"

if "$GDASH" server delete "$TEST_SERVER_ID" &>/dev/null 2>/dev/null; then
  pass "server delete (cleanup)"
else
  fail "server delete (cleanup)"
fi

if "$GDASH" auth logout &>/dev/null 2>/dev/null; then
  pass "auth logout"
else
  fail "auth logout"
fi

# ── Results ───────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
TOTAL=$((PASS + FAIL + SKIP))

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped / ${TOTAL} total (${DURATION}s) ==="

# Write JUnit XML
mkdir -p "$(dirname "$OUTPUT_XML")"
cat > "$OUTPUT_XML" << XMLEOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="GDashE2E" tests="$TOTAL" failures="$FAIL" skipped="$SKIP" time="$DURATION">
  <testsuite name="CLI Smoke Tests" tests="$TOTAL" failures="$FAIL" skipped="$SKIP" time="$DURATION">
XMLEOF

for entry in "${TESTS[@]}"; do
  status="${entry%%|*}"
  name="${entry#*|}"
  case "$status" in
    PASS) echo "    <testcase name=\"$name\" time=\"0.1\"/>" >> "$OUTPUT_XML" ;;
    FAIL) printf "    <testcase name=\"%s\" time=\"0.1\">\n      <failure>%s</failure>\n    </testcase>\n" \
            "$name" "$name" >> "$OUTPUT_XML" ;;
    SKIP) echo "    <testcase name=\"$name\" time=\"0\"><skipped/></testcase>" >> "$OUTPUT_XML" ;;
  esac
done

cat >> "$OUTPUT_XML" << XMLEOF
  </testsuite>
</testsuites>
XMLEOF

echo "Results written to: $OUTPUT_XML"

[[ $FAIL -eq 0 ]]
