import axios from 'axios';

const DAEMON_URL = import.meta.env.VITE_DAEMON_URL ?? 'https://localhost:8443';

export const api = axios.create({
  baseURL: DAEMON_URL,
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

export function getWsUrl(path: string): string {
  const base = DAEMON_URL.replace(/^http/, 'ws').replace(/^https/, 'wss');
  const stored = localStorage.getItem('games-dashboard-auth');
  let token = '';
  if (stored) {
    try { token = JSON.parse(stored)?.state?.token ?? ''; } catch {}
  }
  return `${base}${path}${token ? `?token=${token}` : ''}`;
}
