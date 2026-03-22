#!/usr/bin/env bash
# Garry's Mod Adapter Test Harness
set -euo pipefail

ADAPTER_ID="garrys-mod"
TEST_CONTAINER="gmod-test-$(date +%s)"
TEST_DATA_DIR="/tmp/gmod-test-data"
RESULTS_FILE="/tmp/gmod-test-results.json"
PASS=0
FAIL=0

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
pass() { log "✓ PASS: $*"; PASS=$((PASS+1)); }
fail() { log "✗ FAIL: $*"; FAIL=$((FAIL+1)); }

cleanup() {
  log "Cleaning up test environment..."
  docker rm -f "$TEST_CONTAINER" 2>/dev/null || true
  rm -rf "$TEST_DATA_DIR"
}
trap cleanup EXIT

log "=== Garry's Mod Adapter Test Harness ==="
log "Container: $TEST_CONTAINER"

mkdir -p "$TEST_DATA_DIR/data"

# Test 1: Container startup
log "Test 1: Container startup..."
docker run -d \
  --name "$TEST_CONTAINER" \
  --memory=2g \
  -e SRCDS_RCONPW="testpass" \
  -e SRCDS_PORT=27015 \
  -e SRCDS_MAXPLAYERS=16 \
  -e SRCDS_STARTMAP="gm_construct" \
  -e SRCDS_GAMEMODE="sandbox" \
  -v "$TEST_DATA_DIR/data:/opt/gmod/garrysmod/data" \
  cm2network/garrysmod &>/dev/null

sleep 5

if docker inspect "$TEST_CONTAINER" --format='{{.State.Status}}' 2>/dev/null | grep -q "running"; then
  pass "Container started"
else
  fail "Container failed to start"
fi

# Test 2: Process check
log "Test 2: Server process running..."
if docker exec "$TEST_CONTAINER" pgrep -f srcds_run &>/dev/null; then
  pass "srcds_run process found"
else
  fail "srcds_run process not found"
fi

# Test 3: Log output
log "Test 3: Server log output..."
logs=$(docker logs "$TEST_CONTAINER" 2>&1 | head -50)
if echo "$logs" | grep -qi "garrysmod\|starting\|loading"; then
  pass "Server logs contain expected output"
else
  fail "Server logs missing expected output"
fi

# Test 4: Console attach (non-interactive check)
log "Test 4: Console attach capability..."
if docker exec "$TEST_CONTAINER" sh -c "echo 'console test'" &>/dev/null; then
  pass "Console access available"
else
  fail "Console access failed"
fi

# Test 5: Backup paths exist
log "Test 5: Backup paths..."
if [[ -d "$TEST_DATA_DIR/data" ]]; then
  pass "Backup path exists: $TEST_DATA_DIR/data"
else
  fail "Backup path missing: $TEST_DATA_DIR/data"
fi

# Test 6: Stop server
log "Test 6: Stop server..."
docker stop "$TEST_CONTAINER" &>/dev/null
sleep 2
status=$(docker inspect "$TEST_CONTAINER" --format='{{.State.Status}}' 2>/dev/null)
if [[ "$status" == "exited" ]]; then
  pass "Container stopped cleanly"
else
  fail "Container stop failed (status: $status)"
fi

# Test 7: Backup simulation
log "Test 7: Backup simulation..."
touch "$TEST_DATA_DIR/data/save.dat"
backup_file="/tmp/gmod-backup-test.tar.gz"
tar -czf "$backup_file" -C "$TEST_DATA_DIR" . 2>/dev/null
if [[ -f "$backup_file" ]]; then
  pass "Backup created: $backup_file"
else
  fail "Backup creation failed"
fi

# Test 8: Restore simulation
log "Test 8: Restore simulation..."
restore_dir="/tmp/gmod-restore-test"
mkdir -p "$restore_dir"
tar -xzf "$backup_file" -C "$restore_dir" 2>/dev/null
if [[ -f "$restore_dir/data/save.dat" ]]; then
  pass "Restore successful"
else
  fail "Restore failed"
fi
rm -rf "$restore_dir" "$backup_file"

cat > "$RESULTS_FILE" << EOF
{
  "adapter": "$ADAPTER_ID",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "passed": $PASS,
  "failed": $FAIL,
  "total": $((PASS+FAIL)),
  "success": $([[ $FAIL -eq 0 ]] && echo "true" || echo "false")
}
EOF

log "=== Results ==="
log "Passed: $PASS / $((PASS+FAIL))"
log "Failed: $FAIL / $((PASS+FAIL))"
log "Results: $RESULTS_FILE"

[[ $FAIL -eq 0 ]]
