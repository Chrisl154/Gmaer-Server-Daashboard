import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('../hooks/useServers', () => ({
  useServers: () => ({
    data: { servers: [{ id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'running' }] },
    isLoading: false,
  }),
}));

import { LogsPage } from './LogsPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><LogsPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGet.mockResolvedValue({ data: { entries: [], logs: [] } });
});

describe('LogsPage', () => {
  it('renders page heading', () => {
    wrap();
    expect(screen.getByText('Logs')).toBeTruthy();
  });

  it('renders all four tabs', () => {
    wrap();
    expect(screen.getByText('Server Logs')).toBeTruthy();
    expect(screen.getByText('Events')).toBeTruthy();
    expect(screen.getByText('Security')).toBeTruthy();
    expect(screen.getByText('Audit Trail')).toBeTruthy();
  });

  it('shows Server Logs tab content by default', () => {
    wrap();
    // Server Logs tab description
    expect(screen.getByText(/tail the console output/i)).toBeTruthy();
  });

  it('switches to Events tab on click', () => {
    wrap();
    fireEvent.click(screen.getByText('Events'));
    // Use a unique substring present only in the Events tab description
    expect(screen.getAllByText(/track lifecycle events/i).length).toBeGreaterThan(0);
  });

  it('switches to Security tab on click', () => {
    wrap();
    fireEvent.click(screen.getByText('Security'));
    expect(screen.getAllByText(/review authentication events/i).length).toBeGreaterThan(0);
  });

  it('switches to Audit Trail tab on click', () => {
    wrap();
    fireEvent.click(screen.getByText('Audit Trail'));
    expect(screen.getAllByText(/daemon audit log/i).length).toBeGreaterThan(0);
  });
});
