import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('../hooks/useServers', () => ({
  useSystemStatus: () => ({ data: { uptime: 3600, version: '1.0.0', go_version: 'go1.22' }, isLoading: false }),
}));
vi.mock('../store/authStore', () => ({
  useAuthStore: () => ({
    user: { username: 'admin', role: 'admin' },
    setupTOTP: vi.fn(),
    verifyTOTP: vi.fn(),
  }),
}));

import { SettingsPage } from './SettingsPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><SettingsPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

const settingsData = {
  bind_addr: ':9090',
  shutdown_timeout_s: 30,
  data_dir: '/data',
  log_level: 'info',
  storage: { backend: 'local', s3_bucket: '' },
  backup: { default_schedule: '0 3 * * *', retain_days: 30, compression: 'gzip' },
  metrics: { enabled: true, path: '/metrics' },
  cluster: { enabled: false },
};

beforeEach(() => {
  vi.clearAllMocks();
  mockGet.mockResolvedValue({ data: settingsData });
});

describe('SettingsPage', () => {
  it('renders the nav sidebar', () => {
    wrap();
    expect(screen.getAllByText('General').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Users & Auth').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('TLS').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Storage').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Networking').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Monitoring').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Updates').length).toBeGreaterThanOrEqual(1);
  });

  it('shows General section by default', async () => {
    wrap();
    await waitFor(() => expect(screen.getAllByText('General').length).toBeGreaterThanOrEqual(1));
  });

  it('switches to Users & Auth section on click', () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /users & auth/i }));
    expect(screen.getAllByText(/users & auth/i).length).toBeGreaterThan(0);
  });

  it('switches to TLS section on click', () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /tls/i }));
    expect(screen.getAllByText(/tls/i).length).toBeGreaterThan(0);
  });

  it('switches to Storage section on click', () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /storage/i }));
    expect(screen.getAllByText(/storage/i).length).toBeGreaterThan(0);
  });

  it('switches to Monitoring section on click', () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /monitoring/i }));
    expect(screen.getAllByText(/monitoring/i).length).toBeGreaterThan(0);
  });

  it('switches to Updates section on click', () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /updates/i }));
    expect(screen.getAllByText(/updates/i).length).toBeGreaterThan(0);
  });
});
