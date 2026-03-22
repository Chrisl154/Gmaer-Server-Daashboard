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

import { PortsPage } from './PortsPage';
import { api } from '../utils/api';

const mockGet  = api.get  as ReturnType<typeof vi.fn>;
const mockPost = api.post as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><PortsPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('PortsPage', () => {
  it('renders page heading', () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    expect(screen.getByText('Port Mappings')).toBeTruthy();
  });

  it('renders Validate All button', () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    expect(screen.getByRole('button', { name: /validate all/i })).toBeTruthy();
  });

  it('shows No ports configured when servers have no ports', async () => {
    mockGet.mockResolvedValue({
      data: { servers: [{ id: 'srv-1', name: 'Alpha', adapter: 'minecraft', ports: [] }] },
    });
    wrap();
    await waitFor(() => expect(screen.getByText('No ports configured')).toBeTruthy());
  });

  it('shows ports table when servers have ports', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [
          {
            id: 'srv-1',
            name: 'Alpha',
            adapter: 'minecraft',
            ports: [{ exposed: 25565, internal: 25565, protocol: 'tcp' }],
          },
        ],
      },
    });
    wrap();
    await waitFor(() => expect(screen.getByText('Alpha')).toBeTruthy());
  });

  it('Validate All button is disabled when no ports exist', async () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /validate all/i });
      expect((btn as HTMLButtonElement).disabled).toBe(true);
    });
  });
});
