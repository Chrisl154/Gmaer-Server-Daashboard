import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
  getWsUrl: vi.fn((path: string) => `ws://localhost${path}`),
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('recharts', () => ({
  LineChart: ({ children }: any) => <div>{children}</div>,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: any) => <div>{children}</div>,
}));

// Mock WebSocket so ConsoleTab doesn't crash
class MockWebSocket {
  onmessage: any = null;
  onclose: any = null;
  onerror: any = null;
  close() {}
  send() {}
}
(global as any).WebSocket = MockWebSocket;

import { ServerDetailPage } from './ServerDetailPage';
import { api } from '../utils/api';
const mockPut = api.put as ReturnType<typeof vi.fn>;

const mockGet = api.get as ReturnType<typeof vi.fn>;

const serverData = {
  id: 'srv-1',
  name: 'Test Server',
  adapter: 'minecraft',
  state: 'stopped',
  ports: [],
  mods: [],
  config: {},
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/servers/srv-1']}>
        <Routes>
          <Route path="/servers/:id" element={<ServerDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGet.mockImplementation((url: string) => {
    if (url.includes('/metrics'))      return Promise.resolve({ data: { samples: [] } });
    if (url.includes('/logs'))         return Promise.resolve({ data: { logs: [] } });
    if (url.includes('/backups'))      return Promise.resolve({ data: { backups: [] } });
    if (url.includes('/mods'))         return Promise.resolve({ data: { mods: [] } });
    if (url.includes('/config-files')) return Promise.resolve({ data: { files: [] } });
    // default: server detail
    return Promise.resolve({ data: serverData });
  });
  mockPut.mockResolvedValue({ data: { ok: true } });
});

describe('ServerDetailPage', () => {
  it('renders server name after data loads', async () => {
    wrap();
    await waitFor(() => expect(screen.getAllByText('Test Server').length).toBeGreaterThanOrEqual(1));
  });

  it('renders server adapter', async () => {
    wrap();
    await waitFor(() => expect(screen.getAllByText('minecraft').length).toBeGreaterThanOrEqual(1));
  });

  it('renders all 7 tabs', async () => {
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Overview' })).toBeTruthy());
    ['Console', 'Logs', 'Backups', 'Mods', 'Ports', 'Config'].forEach(tab => {
      expect(screen.getByRole('button', { name: tab })).toBeTruthy();
    });
  });

  it('shows Start button when server is stopped', async () => {
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: /start/i })).toBeTruthy());
  });

  it('shows Stop and Restart buttons when server is running', async () => {
    mockGet.mockResolvedValue({ data: { ...serverData, state: 'running' } });
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: /stop/i })).toBeTruthy());
    expect(screen.getByRole('button', { name: /restart/i })).toBeTruthy();
  });

  it('switches to Console tab on click', async () => {
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Console' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: 'Console' }));
    await waitFor(() => expect(screen.getByText(/console streams live output/i)).toBeTruthy());
  });

  it('switches to Logs tab on click', async () => {
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Logs' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: 'Logs' }));
    await waitFor(() => expect(screen.getAllByText(/logs/i).length).toBeGreaterThan(0));
  });

  it('shows Server not found when query returns null', async () => {
    mockGet.mockResolvedValue({ data: null });
    wrap();
    await waitFor(() => expect(screen.getByText('Server not found.')).toBeTruthy());
  });

  it('switches to Config tab and shows empty state when no config files', async () => {
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Config' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: 'Config' }));
    await waitFor(() => expect(screen.getByText('No config files declared')).toBeTruthy());
  });

  it('Config tab shows file list and editor when files exist', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('/metrics'))  return Promise.resolve({ data: { samples: [] } });
      if (url.includes('/logs'))     return Promise.resolve({ data: { logs: [] } });
      if (url.includes('/backups'))  return Promise.resolve({ data: { backups: [] } });
      if (url.includes('/mods'))     return Promise.resolve({ data: { mods: [] } });
      if (url.endsWith('/config-files')) {
        return Promise.resolve({ data: { files: [
          { path: '/data/server.properties', description: 'Main server config', sample: 'max-players=20' },
        ] } });
      }
      if (url.includes('/config-files/')) {
        return Promise.resolve({ data: { content: 'max-players=20\nmotd=Hello' } });
      }
      return Promise.resolve({ data: serverData });
    });
    wrap();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Config' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: 'Config' }));
    await waitFor(() => expect(screen.getByText('Config Files')).toBeTruthy());
    // File entry should appear in sidebar (and also in status bar — use getAllByText)
    await waitFor(() => expect(screen.getAllByText('server.properties').length).toBeGreaterThanOrEqual(1));
    // Editor textarea should be present
    await waitFor(() => {
      const ta = screen.getByRole('textbox');
      expect(ta).toBeTruthy();
    });
  });
});
