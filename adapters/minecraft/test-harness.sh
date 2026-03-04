#!/usr/bin/env bash
# Minecraft Adapter Test Harness
set -euo pipefail
PASS=0; FAIL=0

pass() { echo "✓ $*"; PASS=$((PASS+1)); }
fail() { echo "✗ $*"; FAIL=$((FAIL+1)); }

echo "=== Minecraft Adapter Test Harness ==="

# Test manifest
if python3 -c "import yaml; yaml.safe_load(open('$(dirname $0)/manifest.yaml'))" 2>/dev/null; then
  pass "manifest.yaml is valid YAML"
else
  fail "manifest.yaml invalid"
fi

# Test RCON port defined
if grep -q "25575" "$(dirname $0)/manifest.yaml"; then
  pass "RCON port 25575 defined"
else
  fail "RCON port missing"
fi

# Test mod sources defined
if grep -q "curseforge\|modrinth" "$(dirname $0)/manifest.yaml"; then
  pass "Mod sources defined"
else
  fail "No mod sources"
fi

echo "Passed: $PASS / $((PASS+FAIL))"
[[ $FAIL -eq 0 ]]
