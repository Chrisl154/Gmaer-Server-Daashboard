import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('../hooks/useServers', () => ({
  useMods:          () => ({ data: { mods: [] }, isLoading: false }),
  useInstallMod:    () => ({ mutate: vi.fn(), isPending: false }),
  useUninstallMod:  () => ({ mutate: vi.fn(), isPending: false }),
  useRunModTests:   () => ({ mutate: vi.fn(), isPending: false }),
  useRollbackMods:  () => ({ mutate: vi.fn(), isPending: false }),
}));

import { ModsPage } from './ModsPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><ModsPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGet.mockResolvedValue({ data: { servers: [], mods: [] } });
});

describe('ModsPage', () => {
  it('renders page heading', () => {
    wrap();
    expect(screen.getByText('Mods')).toBeTruthy();
  });

  it('shows empty state when no servers', async () => {
    mockGet.mockResolvedValue({ data: { servers: [] } });
    wrap();
    await waitFor(() => expect(screen.getByText('No servers yet')).toBeTruthy());
  });

  it('renders description text', () => {
    wrap();
    expect(screen.getByText(/manage mods/i)).toBeTruthy();
  });
});
