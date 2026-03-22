import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../utils/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { api } from '../utils/api';
import { toast } from 'react-hot-toast';
import type { Server, ServersResponse } from '../types';

import {
  useServers,
  useServer,
  useCreateServer,
  useDeleteServer,
  useStartServer,
  useStopServer,
  useBackups,
  useValidatePorts,
  useSystemStatus,
} from './useServers';

const mockApi = api as {
  get: ReturnType<typeof vi.fn>;
  post: ReturnType<typeof vi.fn>;
  put: ReturnType<typeof vi.fn>;
  delete: ReturnType<typeof vi.fn>;
};

const mockToast = toast as { success: ReturnType<typeof vi.fn>; error: ReturnType<typeof vi.fn> };

function makeWrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: qc }, children);
}

beforeEach(() => {
  vi.clearAllMocks();
});

// ── useServers ────────────────────────────────────────────────────────────────

describe('useServers', () => {
  it('returns server list from API', async () => {
    const payload: ServersResponse = {
      servers: [{ id: 'srv-1', name: 'Test' } as Server],
      count: 1,
    };
    mockApi.get.mockResolvedValueOnce({ data: payload });

    const { result } = renderHook(() => useServers(0), { wrapper: makeWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.servers[0].id).toBe('srv-1');
  });
});

// ── useServer ─────────────────────────────────────────────────────────────────

describe('useServer', () => {
  it('returns a single server by id', async () => {
    const server: Server = { id: 'srv-42', name: 'Alpha' } as Server;
    mockApi.get.mockResolvedValueOnce({ data: server });

    const { result } = renderHook(() => useServer('srv-42', 0), { wrapper: makeWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.id).toBe('srv-42');
  });
});

// ── useCreateServer ───────────────────────────────────────────────────────────

describe('useCreateServer', () => {
  it('calls POST /api/v1/servers and fires toast.success on success', async () => {
    const newServer: Server = { id: 'srv-new', name: 'New Server' } as Server;
    mockApi.post.mockResolvedValueOnce({ data: newServer });
    // second call for invalidateQueries refetch — resolve with empty list
    mockApi.get.mockResolvedValue({ data: { servers: [], count: 0 } });

    const { result } = renderHook(() => useCreateServer(), { wrapper: makeWrapper() });
    result.current.mutate({ id: 'srv-new', name: 'New Server', adapter: 'minecraft' });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockApi.post).toHaveBeenCalledWith('/api/v1/servers', expect.objectContaining({ id: 'srv-new' }));
    expect(mockToast.success).toHaveBeenCalledWith('Server created');
  });

  it('calls toast.error with API message on error', async () => {
    const err = Object.assign(new Error('conflict'), {
      response: { data: { error: 'already exists' } },
    });
    mockApi.post.mockRejectedValueOnce(err);

    const { result } = renderHook(() => useCreateServer(), { wrapper: makeWrapper() });
    result.current.mutate({ id: 'srv-dup', name: 'Dup', adapter: 'minecraft' });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(mockToast.error).toHaveBeenCalledWith('already exists');
  });
});

// ── useDeleteServer ───────────────────────────────────────────────────────────

describe('useDeleteServer', () => {
  it('calls DELETE /api/v1/servers/:id', async () => {
    mockApi.delete.mockResolvedValueOnce({});
    mockApi.get.mockResolvedValue({ data: { servers: [], count: 0 } });

    const { result } = renderHook(() => useDeleteServer(), { wrapper: makeWrapper() });
    result.current.mutate('srv-1');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockApi.delete).toHaveBeenCalledWith('/api/v1/servers/srv-1');
  });
});

// ── useStartServer ────────────────────────────────────────────────────────────

describe('useStartServer', () => {
  it('calls POST /api/v1/servers/:id/start and fires toast.success', async () => {
    mockApi.post.mockResolvedValueOnce({});
    mockApi.get.mockResolvedValue({ data: {} });

    const { result } = renderHook(() => useStartServer('srv-1'), { wrapper: makeWrapper() });
    result.current.mutate();

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockApi.post).toHaveBeenCalledWith('/api/v1/servers/srv-1/start');
    expect(mockToast.success).toHaveBeenCalledWith('Server starting...');
  });
});

// ── useStopServer ─────────────────────────────────────────────────────────────

describe('useStopServer', () => {
  it('calls POST /api/v1/servers/:id/stop and fires toast.success', async () => {
    mockApi.post.mockResolvedValueOnce({});
    mockApi.get.mockResolvedValue({ data: {} });

    const { result } = renderHook(() => useStopServer('srv-1'), { wrapper: makeWrapper() });
    result.current.mutate();

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockApi.post).toHaveBeenCalledWith('/api/v1/servers/srv-1/stop');
    expect(mockToast.success).toHaveBeenCalledWith('Server stopping...');
  });
});

// ── useBackups ────────────────────────────────────────────────────────────────

describe('useBackups', () => {
  it('returns empty backup list', async () => {
    mockApi.get.mockResolvedValueOnce({ data: { backups: [], count: 0 } });

    const { result } = renderHook(() => useBackups('srv-1'), { wrapper: makeWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.backups).toEqual([]);
  });
});

// ── useValidatePorts ──────────────────────────────────────────────────────────

describe('useValidatePorts', () => {
  it('calls POST /api/v1/ports/validate and returns results', async () => {
    const payload = { results: [{ internal: 25565, external: 25565, protocol: 'tcp', available: true, reachable: true }] };
    mockApi.post.mockResolvedValueOnce({ data: payload });

    const { result } = renderHook(() => useValidatePorts(), { wrapper: makeWrapper() });
    result.current.mutate([{ internal: 25565, external: 25565, protocol: 'tcp' }]);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockApi.post).toHaveBeenCalledWith('/api/v1/ports/validate', expect.any(Object));
    expect(result.current.data?.results[0].available).toBe(true);
  });
});

// ── useSystemStatus ───────────────────────────────────────────────────────────

describe('useSystemStatus', () => {
  it('returns system status including version', async () => {
    const payload = { version: '1.0.0', uptime_seconds: 3600, healthy: true, timestamp: '', start_time: '', components: {} };
    mockApi.get.mockResolvedValueOnce({ data: payload });

    const { result } = renderHook(() => useSystemStatus(0), { wrapper: makeWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.version).toBe('1.0.0');
  });
});
