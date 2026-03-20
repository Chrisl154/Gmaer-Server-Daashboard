/**
 * k6 Load Test — Games Dashboard API
 *
 * Purpose: Validate the API holds up under sustained concurrent load.
 * Run this manually before a release or after significant backend changes.
 * NOT run in CI automatically — it takes ~3 minutes and needs a real deployment.
 *
 * Usage:
 *   k6 run load.js
 *   k6 run --env BASE_URL=https://my-server:8443 --env ADMIN_PASS=secret load.js
 *
 * Stages:
 *   0–30s  ramp from 0 → 25 VUs
 *   30–90s hold at 25 VUs
 *   90–120s hold at 50 VUs (peak)
 *   120–150s ramp back to 0
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const apiLatency = new Trend('api_request_duration', true);

const BASE_URL   = __ENV.BASE_URL   || 'https://localhost:8443';
const ADMIN_USER = __ENV.ADMIN_USER || 'admin';
const ADMIN_PASS = __ENV.ADMIN_PASS || 'TestPassword123!';

export const options = {
  stages: [
    { duration: '30s', target: 25 },
    { duration: '60s', target: 25 },
    { duration: '30s', target: 50 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration:    ['p(95)<2000', 'p(99)<5000'],
    http_req_failed:      ['rate<0.02'],  // <2% errors at load
    api_request_duration: ['p(95)<2000'],
    errors:               ['rate<0.02'],
  },
  insecureSkipTLSVerify: true,
  summaryTrendStats: ['min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

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

export default function (data) {
  const authHeaders = {
    headers: {
      Authorization: `Bearer ${data.token}`,
      'Content-Type': 'application/json',
    },
    insecureSkipTLSVerify: true,
  };

  group('health + version', () => {
    const h = http.get(`${BASE_URL}/healthz`, { insecureSkipTLSVerify: true });
    errorRate.add(!check(h, { 'healthz 200': r => r.status === 200 }));
    apiLatency.add(h.timings.duration);
  });

  sleep(0.1);

  group('server list', () => {
    const s = http.get(`${BASE_URL}/api/v1/servers`, authHeaders);
    errorRate.add(!check(s, { 'servers 200': r => r.status === 200 }));
    apiLatency.add(s.timings.duration);
  });

  sleep(0.1);

  group('system resources', () => {
    const r = http.get(`${BASE_URL}/api/v1/system/resources`, authHeaders);
    errorRate.add(!check(r, { 'resources 200': r => r.status === 200 }));
    apiLatency.add(r.timings.duration);
  });

  sleep(0.3);
}
