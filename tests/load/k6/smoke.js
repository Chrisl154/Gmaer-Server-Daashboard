/**
 * k6 Smoke Test — Games Dashboard API
 *
 * Purpose: Establish a performance baseline. Run against a live daemon to
 * verify response times and error rates are within acceptable bounds.
 *
 * Usage:
 *   k6 run smoke.js
 *   k6 run --env BASE_URL=https://localhost:8443 --env ADMIN_PASS=secret smoke.js
 *
 * In CI this is non-gating — results are uploaded as an artifact for trending.
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// ── Custom metrics ────────────────────────────────────────────────────────────
const errorRate   = new Rate('errors');
const authLatency = new Trend('auth_login_duration', true);
const apiLatency  = new Trend('api_request_duration', true);
const serverOps   = new Counter('server_list_requests');

// ── Config ────────────────────────────────────────────────────────────────────
const BASE_URL   = __ENV.BASE_URL   || 'https://localhost:8443';
const ADMIN_USER = __ENV.ADMIN_USER || 'admin';
const ADMIN_PASS = __ENV.ADMIN_PASS || 'TestPassword123!';

// ── Thresholds (non-gating in CI — tracked for trending) ─────────────────────
export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: 5,
      duration: '30s',
    },
  },
  thresholds: {
    // These are soft targets — CI uses continue-on-error so they won't block
    http_req_duration:      ['p(95)<1000'], // 95th pct under 1s on CI runners
    http_req_failed:        ['rate<0.05'],  // <5% errors
    auth_login_duration:    ['p(95)<2000'],
    api_request_duration:   ['p(95)<1000'],
  },
  // Accept self-signed TLS in test environments
  insecureSkipTLSVerify: true,
  // Suppress per-request output for cleaner CI logs
  summaryTrendStats: ['min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

// ── Setup: obtain a JWT ───────────────────────────────────────────────────────
export function setup() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ username: ADMIN_USER, password: ADMIN_PASS }),
    { headers: { 'Content-Type': 'application/json' }, insecureSkipTLSVerify: true },
  );

  check(res, { 'setup: login 200': r => r.status === 200 });

  const body = res.json();
  if (!body || !body.token) {
    throw new Error(`Setup login failed: ${res.status} ${res.body}`);
  }
  return { token: body.token };
}

// ── Main VU loop ──────────────────────────────────────────────────────────────
export default function (data) {
  const authHeaders = {
    headers: {
      Authorization: `Bearer ${data.token}`,
      'Content-Type': 'application/json',
    },
    insecureSkipTLSVerify: true,
  };

  // ── Group 1: Public / health endpoints ────────────────────────────────────
  group('public endpoints', () => {
    const health = http.get(`${BASE_URL}/healthz`, { insecureSkipTLSVerify: true });
    const ok = check(health, {
      'healthz 200': r => r.status === 200,
      'healthz fast': r => r.timings.duration < 200,
    });
    errorRate.add(!ok);
    apiLatency.add(health.timings.duration);

    const version = http.get(`${BASE_URL}/api/v1/version`, { insecureSkipTLSVerify: true });
    const vOk = check(version, { 'version 200': r => r.status === 200 });
    errorRate.add(!vOk);
    apiLatency.add(version.timings.duration);
  });

  sleep(0.2);

  // ── Group 2: Authenticated API reads ──────────────────────────────────────
  group('authenticated reads', () => {
    const servers = http.get(`${BASE_URL}/api/v1/servers`, authHeaders);
    const sOk = check(servers, {
      'servers 200': r => r.status === 200,
      'servers is array': r => {
        try { return Array.isArray(JSON.parse(r.body)); } catch { return false; }
      },
    });
    errorRate.add(!sOk);
    apiLatency.add(servers.timings.duration);
    serverOps.add(1);

    const resources = http.get(`${BASE_URL}/api/v1/system/resources`, authHeaders);
    const rOk = check(resources, { 'resources 200': r => r.status === 200 });
    errorRate.add(!rOk);
    apiLatency.add(resources.timings.duration);

    const metrics = http.get(`${BASE_URL}/metrics`, { insecureSkipTLSVerify: true });
    check(metrics, { 'metrics 200': r => r.status === 200 });
    apiLatency.add(metrics.timings.duration);
  });

  sleep(0.5);

  // ── Group 3: Auth token refresh (re-login) ─────────────────────────────────
  // Only one in every 10 iterations to avoid hammering auth
  if (__ITER % 10 === 0) {
    group('auth login', () => {
      const start = Date.now();
      const res = http.post(
        `${BASE_URL}/api/v1/auth/login`,
        JSON.stringify({ username: ADMIN_USER, password: ADMIN_PASS }),
        { headers: { 'Content-Type': 'application/json' }, insecureSkipTLSVerify: true },
      );
      authLatency.add(Date.now() - start);
      const ok = check(res, { 'login 200': r => r.status === 200 });
      errorRate.add(!ok);
    });
  }

  sleep(0.3);
}

// ── Teardown: print summary note ──────────────────────────────────────────────
export function teardown() {
  console.log('Smoke test complete. Results uploaded as CI artifact for trending.');
}
