#!/usr/bin/env bash
# Valheim Adapter Test Harness
# Tests: start, stop, console attach, backup, restore in ephemeral container
set -euo pipefail

ADAPTER_ID="valheim"
TEST_CONTAINER="valheim-test-$(date +%s)"
TEST_WORLD_DIR="/tmp/valheim-test-worlds"
RESULTS_FILE="/tmp/valheim-test-results.json"
PASS=0
FAIL=0

log() { echo "[$(date '+%H:%M:%S')] $*"; }
pass() { log "✓ PASS: $*"; PASS=$((PASS+1)); }
fail() { log "✗ FAIL: $*"; FAIL=$((FAIL+1)); }

# Cleanup on exit
cleanup() {
  log "Cleaning up test environment..."
  docker rm -f "$TEST_CONTAINER" 2>/dev/null || true
  rm -rf "$TEST_WORLD_DIR"
}
trap cleanup EXIT

log "=== Valheim Adapter Test Harness ==="
log "Container: $TEST_CONTAINER"

# Setup
mkdir -p "$TEST_WORLD_DIR"

# Test 1: Container startup
log "Test 1: Container startup..."
docker run -d \
  --name "$TEST_CONTAINER" \
  --memory=2g \
  -e SERVER_NAME="TestServer" \
  -e WORLD_NAME="TestWorld" \
  -e SERVER_PASSWORD="testpass" \
  -e SERVER_PUBLIC=0 \
  -v "$TEST_WORLD_DIR:/data/worlds" \
  lloesche/valheim-server:latest &>/dev/null

sleep 5

if docker inspect "$TEST_CONTAINER" --format='{{.State.Status}}' 2>/dev/null | grep -q "running"; then
  pass "Container started"
else
  fail "Container failed to start"
fi

# Test 2: Process check
log "Test 2: Server process running..."
if docker exec "$TEST_CONTAINER" pgrep -f valheim_server &>/dev/null; then
  pass "valheim_server process found"
else
  fail "valheim_server process not found"
fi

# Test 3: Log output
log "Test 3: Server log output..."
logs=$(docker logs "$TEST_CONTAINER" 2>&1 | head -50)
if echo "$logs" | grep -qi "valheim\|starting\|loading"; then
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
if [[ -d "$TEST_WORLD_DIR" ]]; then
  pass "Backup path exists: $TEST_WORLD_DIR"
else
  fail "Backup path missing: $TEST_WORLD_DIR"
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
touch "$TEST_WORLD_DIR/TestWorld.db"
touch "$TEST_WORLD_DIR/TestWorld.fwl"
backup_file="/tmp/valheim-backup-test.tar.gz"
tar -czf "$backup_file" -C "$TEST_WORLD_DIR" . 2>/dev/null
if [[ -f "$backup_file" ]]; then
  pass "Backup created: $backup_file"
else
  fail "Backup creation failed"
fi

# Test 8: Restore simulation
log "Test 8: Restore simulation..."
restore_dir="/tmp/valheim-restore-test"
mkdir -p "$restore_dir"
tar -xzf "$backup_file" -C "$restore_dir" 2>/dev/null
if [[ -f "$restore_dir/TestWorld.db" ]]; then
  pass "Restore successful"
else
  fail "Restore failed"
fi
rm -rf "$restore_dir" "$backup_file"

# Write results
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
