import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

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

  it('renders resource table with server rows', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [
          {
            id: 's1', name: 'Valheim Server', adapter: 'valheim', state: 'running',
            ports: [], resources: { cpu_cores: 4, ram_gb: 8, disk_gb: 40 },
            cpu_pct: 55, ram_pct: 70, disk_pct: 35,
          },
          {
            id: 's2', name: 'Minecraft', adapter: 'minecraft', state: 'stopped',
            ports: [], resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
            cpu_pct: 0, ram_pct: 0, disk_pct: 45,
          },
        ],
        count: 2,
      },
    });
    wrap();
    await waitFor(() => expect(screen.getByText('Resource Overview')).toBeTruthy());
    expect(screen.getByText('Valheim Server')).toBeTruthy();
    expect(screen.getAllByText('Minecraft').length).toBeGreaterThanOrEqual(1);
    // Column headers
    expect(screen.getByText('CPU')).toBeTruthy();
    expect(screen.getByText('RAM')).toBeTruthy();
    expect(screen.getByText('Disk')).toBeTruthy();
    expect(screen.getByText('Allocated')).toBeTruthy();
    // Status labels (may appear in stats cards too)
    expect(screen.getAllByText('Running').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Stopped').length).toBeGreaterThanOrEqual(1);
    // Allocated resources
    expect(screen.getByText('4 cores')).toBeTruthy();
    expect(screen.getByText('8 GB RAM')).toBeTruthy();
  });

  it('shows disk warning banner when a server has disk_pct >= 85', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [{
          id: 's1', name: 'Valheim', adapter: 'valheim', state: 'running',
          ports: [], resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
          disk_pct: 88,
        }],
        count: 1,
      },
    });
    wrap();
    await waitFor(() => expect(screen.getByText(/disk space running low/i)).toBeTruthy());
    expect(screen.getByText(/Valheim \(88%\)/)).toBeTruthy();
  });

  it('shows critical disk banner when disk_pct >= 95', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [{
          id: 's1', name: 'Minecraft', adapter: 'minecraft', state: 'running',
          ports: [], resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
          disk_pct: 97,
        }],
        count: 1,
      },
    });
    wrap();
    await waitFor(() => expect(screen.getByText(/critical/i)).toBeTruthy());
  });

  it('does not show disk banner when all servers are below 85%', async () => {
    mockGet.mockResolvedValue({
      data: {
        servers: [{
          id: 's1', name: 'Minecraft', adapter: 'minecraft', state: 'running',
          ports: [], resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
          disk_pct: 60,
        }],
        count: 1,
      },
    });
    wrap();
    await waitFor(() => expect(screen.queryByText(/disk space/i)).toBeNull());
  });

  describe('GettingStartedChecklist', () => {
    it('renders the Getting Started checklist by default', async () => {
      mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
      wrap();
      await waitFor(() => expect(screen.getByText('Getting Started')).toBeTruthy());
      expect(screen.getByText('Add your first game server')).toBeTruthy();
      expect(screen.getByText('Take a backup')).toBeTruthy();
      expect(screen.getByText('Set up crash notifications')).toBeTruthy();
      expect(screen.getByText('Invite a user')).toBeTruthy();
    });

    it('hides checklist after dismiss button click', async () => {
      mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
      wrap();
      await waitFor(() => expect(screen.getByTitle('Dismiss')).toBeTruthy());
      fireEvent.click(screen.getByTitle('Dismiss'));
      await waitFor(() => expect(screen.queryByText('Getting Started')).toBeNull());
    });

    it('auto-marks server step done when serverCount > 0', async () => {
      mockGet.mockResolvedValue({
        data: {
          servers: [{ id: 's1', name: 'Test', adapter: 'minecraft', state: 'running', ports: [], resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 } }],
          count: 1,
        },
      });
      wrap();
      await waitFor(() => {
        // "1/4" badge indicates one step done
        expect(screen.getByText('1/4')).toBeTruthy();
      });
    });

    it('does not render checklist when already dismissed via localStorage', async () => {
      localStorage.setItem('gdash_checklist_dismissed', '1');
      mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
      wrap();
      await waitFor(() => expect(screen.queryByText('Getting Started')).toBeNull());
    });

    it('collapses and expands the checklist', async () => {
      mockGet.mockResolvedValue({ data: { servers: [], count: 0 } });
      wrap();
      await waitFor(() => expect(screen.getByText('Getting Started')).toBeTruthy());
      // Step text is visible before collapsing
      expect(screen.getByText('Add your first game server')).toBeTruthy();
      // Click header to collapse
      fireEvent.click(screen.getByText('Getting Started'));
      await waitFor(() => expect(screen.queryByText('Add your first game server')).toBeNull());
      // Click again to expand
      fireEvent.click(screen.getByText('Getting Started'));
      await waitFor(() => expect(screen.getByText('Add your first game server')).toBeTruthy());
    });
  });
});
