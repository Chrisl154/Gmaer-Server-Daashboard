import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { NodesPage } from './NodesPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

const testNode = {
  id: 'node-1',
  name: 'node-alpha',
  hostname: 'node-alpha',
  address: '10.0.0.1:9090',
  status: 'online',
  capacity:  { cpu_cores: 4, memory_gb: 8,  disk_gb: 100 },
  allocated: { cpu_cores: 1, memory_gb: 2,  disk_gb: 20  },
};

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><NodesPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('NodesPage', () => {
  it('renders page heading', () => {
    mockGet.mockResolvedValue({ data: { nodes: [] } });
    wrap();
    expect(screen.getByText('Cluster Nodes')).toBeTruthy();
  });

  it('renders Register Node button', () => {
    mockGet.mockResolvedValue({ data: { nodes: [] } });
    wrap();
    // Button has text "Register Node" (with SVG icon)
    expect(screen.getByRole('button', { name: /register node/i })).toBeTruthy();
  });

  it('shows empty state when no nodes', async () => {
    mockGet.mockResolvedValue({ data: { nodes: [] } });
    wrap();
    await waitFor(() => expect(screen.getByText('No nodes registered')).toBeTruthy());
  });

  it('shows node card when nodes exist', async () => {
    mockGet.mockResolvedValue({ data: { nodes: [testNode] } });
    wrap();
    await waitFor(() => expect(screen.getByText('node-alpha')).toBeTruthy());
  });

  it('opens register dialog on button click', async () => {
    mockGet.mockResolvedValue({ data: { nodes: [] } });
    wrap();
    const btn = screen.getByRole('button', { name: /register node/i });
    fireEvent.click(btn);
    // Modal shows a "Hostname" label unique to the dialog
    await waitFor(() => expect(screen.getByText('Hostname')).toBeTruthy());
  });
});
