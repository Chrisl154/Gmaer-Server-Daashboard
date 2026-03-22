import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('../hooks/useServers', () => ({
  useTriggerBackup: () => ({ mutate: vi.fn(), isPending: false }),
  useRestoreBackup: () => ({ mutate: vi.fn(), isPending: false }),
}));

import { BackupsPage } from './BackupsPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><BackupsPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('BackupsPage', () => {
  it('renders page heading', async () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    expect(screen.getByText('Backups')).toBeTruthy();
  });

  it('shows empty state when no servers', async () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    await waitFor(() => expect(screen.getByText('No servers found')).toBeTruthy());
  });

  it('shows loading skeletons while fetching', () => {
    mockGet.mockReturnValue(new Promise(() => {})); // never resolves
    const { container } = wrap();
    const skeletons = container.querySelectorAll('.animate-pulse, [style*="pulse"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows Server Backups heading and count badge when servers exist', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [
          { id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'running' },
          { id: 'srv-2', name: 'Beta',  adapter: 'valheim',   state: 'stopped' },
        ],
      },
    });
    wrap();
    await waitFor(() => expect(screen.getByText('Server Backups')).toBeTruthy());
    expect(screen.getByText('2 servers')).toBeTruthy();
  });

  it('renders a Backup Now button per server', async () => {
    mockGet.mockResolvedValue({
      data: { servers: [{ id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'running' }] },
    });
    wrap();
    await waitFor(() => {
      const buttons = screen.getAllByText(/backup now/i);
      expect(buttons.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows singular "1 server" badge', async () => {
    mockGet.mockResolvedValue({
      data: { servers: [{ id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'stopped' }] },
    });
    wrap();
    await waitFor(() => expect(screen.getByText('1 server')).toBeTruthy());
  });

  it('expands a server card on click', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('/backups')) return Promise.resolve({ data: { backups: [] } });
      return Promise.resolve({ data: { servers: [{ id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'running' }] } });
    });
    wrap();
    await waitFor(() => expect(screen.getByText('Alpha')).toBeTruthy());
    fireEvent.click(screen.getByText('Alpha'));
    // No crash after accordion expansion
    await new Promise(r => setTimeout(r, 10));
  });
});
