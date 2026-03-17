import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => vi.fn() };
});

// recharts uses ResizeObserver which isn't in jsdom — stub it out
vi.mock('recharts', () => ({
  AreaChart: ({ children }: any) => React.createElement('div', { 'data-testid': 'area-chart' }, children),
  Area: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: any) => React.createElement('div', null, children),
}));

import { api } from '../utils/api';
import { DashboardPage } from './DashboardPage';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><DashboardPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('DashboardPage', () => {
  it('renders the Dashboard heading', () => {
    mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
    wrap();
    expect(screen.getByText('Dashboard')).toBeTruthy();
  });

  it('renders all four stats cards', () => {
    mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
    wrap();
    expect(screen.getByText('Total Servers')).toBeTruthy();
    expect(screen.getByText('Running')).toBeTruthy();
    expect(screen.getByText('Stopped')).toBeTruthy();
    expect(screen.getByText('System Health')).toBeTruthy();
  });

  it('shows 0 total when no servers', () => {
    mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
    wrap();
    // value renders as text; there are multiple "0" from stats cards
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThanOrEqual(1);
  });

  it('shows empty servers state after data loads', async () => {
    mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
    wrap();
    await waitFor(() => expect(screen.getByText('No servers yet')).toBeTruthy());
  });

  it('shows system health as checking initially', () => {
    // api.get never resolves — simulates loading
    mockGet.mockReturnValue(new Promise(() => {}));
    wrap();
    expect(screen.getByText(/checking/i)).toBeTruthy();
  });
});
