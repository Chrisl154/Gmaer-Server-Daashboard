/// <reference types="vite/client" />
import axios from 'axios';

// Empty string → same-origin (nginx proxies /api/, /healthz to daemon).
// Set VITE_DAEMON_URL at build time only for Docker/Helm where the UI is
// served from a different origin than the daemon.
const DAEMON_URL = import.meta.env.VITE_DAEMON_URL ?? '';

export const api = axios.create({
  baseURL: DAEMON_URL || undefined,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
});

// Request interceptor: inject stored token
api.interceptors.request.use(config => {
  const stored = localStorage.getItem('games-dashboard-auth');
  if (stored) {
    try {
      const { state } = JSON.parse(stored);
      if (state?.token) {
        config.headers.Authorization = `Bearer ${state.token}`;
      }
    } catch {}
  }
  return config;
});

// Response interceptor: handle 401
api.interceptors.response.use(
  response => response,
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('games-dashboard-auth');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

/**
 * Get WebSocket URL for authenticated connections.
 * Browsers cannot set custom headers on WebSocket upgrades, so the JWT is
 * appended as a ?token= query parameter which streamConsole validates.
 */
export function getWsUrl(path: string): string {
  // When no explicit daemon URL, derive WebSocket host from the page origin so
  // the connection goes through the same nginx reverse-proxy as the REST calls.
  const base = DAEMON_URL
    ? DAEMON_URL.replace(/^http/, 'ws').replace(/^https/, 'wss')
    : `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}`;
  let token = '';
  try {
    const stored = localStorage.getItem('games-dashboard-auth');
    if (stored) {
      const { state } = JSON.parse(stored);
      token = state?.token ?? '';
    }
  } catch {}
  const sep = path.includes('?') ? '&' : '?';
  return `${base}${path}${token ? `${sep}token=${encodeURIComponent(token)}` : ''}`;
}
