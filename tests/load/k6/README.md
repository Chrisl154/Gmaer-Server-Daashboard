# k6 Load Tests

## Scripts

| Script | Purpose | VUs | Duration |
|---|---|---|---|
| `smoke.js` | CI baseline — verify response times after every build | 5 | 30s |
| `load.js` | Pre-release load test — sustained concurrent load | 0→50 | ~3 min |

## Quick start

```bash
# Install k6 (Linux)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# Run smoke test against a local daemon
k6 run smoke.js

# Run against a specific host
k6 run --env BASE_URL=https://my-server:8443 --env ADMIN_PASS=changeme smoke.js

# Run full load test (takes ~3 minutes)
k6 run load.js
```

## Metrics captured

- `http_req_duration` — overall request latency (p50/p90/p95/p99)
- `http_req_failed` — HTTP error rate
- `auth_login_duration` — login endpoint latency
- `api_request_duration` — authenticated API latency
- `server_list_requests` — total `/servers` calls made
- `errors` — custom error rate (failed checks)

## CI behaviour

`smoke.js` runs in CI as a **non-gating** step (`continue-on-error: true`).
The JSON summary is uploaded as the `load-test-results` artifact so you can
track trends across commits without blocking deployments on flaky CI runners.

Raise the thresholds in `smoke.js` once you have a few runs of baseline data.
