#!/usr/bin/env bash
# smoke.sh — live API smoke test for the games-dashboard daemon
#
# Usage:
#   ./test/smoke.sh                         # uses defaults below
#   BASE_URL=http://localhost:9090 ./test/smoke.sh
#   ADMIN_PASS=mypass ./test/smoke.sh
#
# Requires: curl, python3

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin123}"

PASS=0
FAIL=0

# ── helpers ──────────────────────────────────────────────────────────────────

green() { printf '\033[32m✓  %s\033[0m\n' "$*"; }
red()   { printf '\033[31m✗  %s\033[0m\n' "$*"; }

pass() { green "$1"; PASS=$((PASS + 1)); }
fail() { red   "$1"; FAIL=$((FAIL + 1)); }

# curl wrapper: method url [extra curl args...]
api() {
  local method="$1" url="$2"; shift 2
  curl -s -X "$method" "${BASE_URL}${url}" "$@"
}

jq_field() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)" 2>/dev/null; }

assert_field() {
  local label="$1" resp="$2" field="$3" expected="$4"
  local actual
  actual=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d$field)" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$actual" == "$expected" ]]; then
    pass "$label"
  else
    fail "$label (got: $actual, expected: $expected)"
  fi
}

assert_key() {
  local label="$1" resp="$2" field="$3"
  local actual
  # Use 'is not None' so count:0, empty string, false are still "present"
  actual=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
try:
    v=d$field
    print('True')
except (KeyError,IndexError,TypeError):
    print('False')
" 2>/dev/null || echo "False")
  if [[ "$actual" == "True" ]]; then
    pass "$label"
  else
    fail "$label (field $field missing in: ${resp:0:120})"
  fi
}

assert_http() {
  local label="$1" code="$2" resp="$3"
  local err
  err=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('error',''))" 2>/dev/null || echo "")
  if [[ "$err" == *"$code"* ]]; then
    pass "$label"
  else
    fail "$label (response: ${resp:0:120})"
  fi
}

echo ""
echo "══════════════════════════════════════════════════"
echo "  Games Dashboard Daemon — Smoke Test"
echo "  ${BASE_URL}"
echo "══════════════════════════════════════════════════"
echo ""

# ── 1. Health ─────────────────────────────────────────────────────────────────
echo "── Health ──────────────────────────────────────────"

resp=$(api GET /healthz)
assert_field "GET /healthz returns healthy:true" "$resp" "['healthy']" "True"

# ── 2. Auth — Login ───────────────────────────────────────────────────────────
echo ""
echo "── Auth: Login ─────────────────────────────────────"

resp=$(api POST /api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}")
assert_key "POST /auth/login returns token" "$resp" "['token']"
TOKEN=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)

if [[ -z "$TOKEN" ]]; then
  red "Cannot continue — no token obtained. Is the daemon running at ${BASE_URL}?"
  exit 1
fi

AUTH=(-H "Authorization: Bearer ${TOKEN}")

# Wrong password rejected
resp=$(api POST /api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"WRONG\"}")
assert_field "Wrong password → invalid credentials" "$resp" "['error']" "invalid credentials"

# No token rejected
resp=$(api GET /api/v1/servers)
assert_field "No token → missing token" "$resp" "['error']" "missing token"

# ── 3. Auth — Profile ─────────────────────────────────────────────────────────
echo ""
echo "── Auth: Profile ───────────────────────────────────"

resp=$(api GET /api/v1/users/me "${AUTH[@]}")
assert_field "GET /users/me → correct username" "$resp" "['username']" "${ADMIN_USER}"
assert_field "GET /users/me → has admin role" "$resp" "['roles'][0]" "admin"

# ── 4. Servers ────────────────────────────────────────────────────────────────
echo ""
echo "── Servers ─────────────────────────────────────────"

resp=$(api GET /api/v1/servers "${AUTH[@]}")
assert_key "GET /servers → has count field" "$resp" "['count']"

# ── 5. User Management ────────────────────────────────────────────────────────
echo ""
echo "── User Management ─────────────────────────────────"

TEST_USER="smoketest_$(date +%s)"
TEST_PASS="Smoke1234!"

# Create viewer user
resp=$(api POST /api/v1/admin/users "${AUTH[@]}" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${TEST_USER}\",\"password\":\"${TEST_PASS}\",\"roles\":[\"viewer\"]}")
assert_field "Create viewer user" "$resp" "['username']" "${TEST_USER}"
TEST_USER_ID=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

# List users — admin sees them all
resp=$(api GET /api/v1/admin/users "${AUTH[@]}")
list_has_user=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
names=[u['username'] for u in d.get('users',[])]
print('True' if '${TEST_USER}' in names else 'False')
" 2>/dev/null)
[[ "$list_has_user" == "True" ]] && pass "List users includes new user" || fail "List users — new user not found"

# Viewer cannot create users (RBAC)
resp=$(api POST /api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${TEST_USER}\",\"password\":\"${TEST_PASS}\"}")
VIEWER_TOKEN=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
VIEWER_AUTH=(-H "Authorization: Bearer ${VIEWER_TOKEN}")

resp=$(api POST /api/v1/admin/users "${VIEWER_AUTH[@]}" \
  -H "Content-Type: application/json" \
  -d '{"username":"hacker","password":"h4ck!","roles":["admin"]}')
assert_field "Viewer cannot create users (RBAC)" "$resp" "['error']" "insufficient permissions"

# Delete test user
if [[ -n "$TEST_USER_ID" ]]; then
  resp=$(api DELETE "/api/v1/admin/users/${TEST_USER_ID}" "${AUTH[@]}")
  pass "Delete test user (id=${TEST_USER_ID})"
fi

# ── 6. API Keys ───────────────────────────────────────────────────────────────
echo ""
echo "── API Keys ────────────────────────────────────────"

resp=$(api POST /api/v1/auth/api-keys "${AUTH[@]}" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-test-key","roles":["viewer"]}')
assert_key "Create API key → token returned" "$resp" "['token']"
assert_key "Create API key → prefix returned" "$resp" "['key']['prefix']"
API_KEY_TOKEN=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
API_KEY_ID=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['key']['id'])" 2>/dev/null)

# Use API key to call authenticated endpoint
if [[ -n "$API_KEY_TOKEN" ]]; then
  resp=$(api GET /api/v1/servers -H "Authorization: Bearer ${API_KEY_TOKEN}")
  assert_key "API key auth → can list servers" "$resp" "['count']"
fi

# Revoke API key
if [[ -n "$API_KEY_ID" ]]; then
  api DELETE "/api/v1/auth/api-keys/${API_KEY_ID}" "${AUTH[@]}" > /dev/null
  pass "Revoke API key"
fi

# ── 7. Persistence check ──────────────────────────────────────────────────────
echo ""
echo "── Persistence ─────────────────────────────────────"

DATA_DIR="${GDASH_DATA_DIR:-/tmp/gdash-test/data}"

if [[ -f "${DATA_DIR}/users.json" ]]; then
  user_count=$(python3 -c "import json; d=json.load(open('${DATA_DIR}/users.json')); print(len(d))" 2>/dev/null || echo "0")
  [[ "$user_count" -ge 1 ]] && pass "users.json exists with ${user_count} user(s)" || fail "users.json empty"
else
  fail "users.json not found at ${DATA_DIR}/users.json (set GDASH_DATA_DIR if different)"
fi

if [[ -f "${DATA_DIR}/audit.log" ]]; then
  audit_count=$(wc -l < "${DATA_DIR}/audit.log" | tr -d ' ')
  [[ "$audit_count" -ge 1 ]] && pass "audit.log exists with ${audit_count} entr(ies)" || fail "audit.log empty"
else
  fail "audit.log not found at ${DATA_DIR}/audit.log"
fi

# ── 8. Version ────────────────────────────────────────────────────────────────
echo ""
echo "── Version ─────────────────────────────────────────"

resp=$(api GET /api/v1/version)
assert_key "GET /version returns version" "$resp" "['version']"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
if [[ $FAIL -eq 0 ]]; then
  green "All ${TOTAL} tests passed"
else
  red "${FAIL}/${TOTAL} tests FAILED"
fi
echo "══════════════════════════════════════════════════"
echo ""

[[ $FAIL -eq 0 ]]
