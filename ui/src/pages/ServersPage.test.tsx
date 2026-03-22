import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => vi.fn() };
});

const mockUseServers = vi.fn();
vi.mock('../hooks/useServers', () => ({
  useServers:       (...args: any[]) => mockUseServers(...args),
  useStartServer:   () => ({ mutate: vi.fn(), isPending: false }),
  useStopServer:    () => ({ mutate: vi.fn(), isPending: false }),
  useRestartServer: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteServer:  () => ({ mutate: vi.fn(), isPending: false }),
}));

import { ServersPage } from './ServersPage';

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><ServersPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('ServersPage', () => {
  it('renders page heading', () => {
    mockUseServers.mockReturnValue({ data: { servers: [], count: 0 }, isLoading: false });
    wrap();
    expect(screen.getByText('Servers')).toBeTruthy();
  });

  it('renders New Server button', () => {
    mockUseServers.mockReturnValue({ data: { servers: [], count: 0 }, isLoading: false });
    wrap();
    expect(screen.getByRole('button', { name: /new server/i })).toBeTruthy();
  });

  it('shows empty state when no servers', () => {
    mockUseServers.mockReturnValue({ data: { servers: [], count: 0 }, isLoading: false });
    wrap();
    expect(screen.getByText('No servers yet')).toBeTruthy();
  });

  it('renders server cards when data is present', () => {
    mockUseServers.mockReturnValue({
      data: {
        servers: [
          { id: 'srv-1', name: 'Alpha', adapter: 'minecraft', state: 'running' },
          { id: 'srv-2', name: 'Beta',  adapter: 'valheim',   state: 'stopped' },
        ],
        count: 2,
      },
      isLoading: false,
    });
    wrap();
    expect(screen.getByLabelText('Open Alpha')).toBeTruthy();
    expect(screen.getByLabelText('Open Beta')).toBeTruthy();
  });

  it('shows server count badge when servers exist', () => {
    mockUseServers.mockReturnValue({
      data: { servers: [{ id: 's1', name: 'A', adapter: 'minecraft', state: 'stopped' }], count: 1 },
      isLoading: false,
    });
    wrap();
    expect(screen.getByText('1')).toBeTruthy();
  });

  it('shows loading skeletons while fetching', () => {
    mockUseServers.mockReturnValue({ data: undefined, isLoading: true });
    const { container } = wrap();
    // Skeletons render as divs with pulse animation
    const skeletons = container.querySelectorAll('[style*="pulse"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('filters servers by search term', () => {
    mockUseServers.mockReturnValue({
      data: {
        servers: [
          { id: 's1', name: 'Alpha',  adapter: 'minecraft', state: 'running' },
          { id: 's2', name: 'Bravo',  adapter: 'valheim',   state: 'stopped' },
        ],
        count: 2,
      },
      isLoading: false,
    });
    wrap();

    const searchInput = screen.getByPlaceholderText(/search servers/i);
    fireEvent.change(searchInput, { target: { value: 'Alpha' } });

    expect(screen.getByLabelText('Open Alpha')).toBeTruthy();
    expect(screen.queryByLabelText('Open Bravo')).toBeNull();
  });

  it('shows no-results state when search has no matches', () => {
    mockUseServers.mockReturnValue({
      data: { servers: [{ id: 's1', name: 'Alpha', adapter: 'minecraft', state: 'running' }], count: 1 },
      isLoading: false,
    });
    wrap();

    fireEvent.change(screen.getByPlaceholderText(/search servers/i), { target: { value: 'zzz' } });
    expect(screen.getByText(/no results for/i)).toBeTruthy();
  });

  it('opens create server modal on New Server click', () => {
    mockUseServers.mockReturnValue({ data: { servers: [], count: 0 }, isLoading: false });
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /new server/i }));
    expect(screen.getByText('Choose a Game')).toBeTruthy();
  });
});
