import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../../utils/api', () => ({
  api: { post: vi.fn(), get: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

import { ServerCard } from './ServerCard';

function wrap(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

const base = {
  id: 'srv-1',
  name: 'My Server',
  adapter: 'minecraft',
  state: 'stopped',
  deploy_method: 'docker',
  ports: [{ internal: 25565, external: 25565, protocol: 'tcp' }],
  resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
};

beforeEach(() => vi.clearAllMocks());

describe('ServerCard', () => {
  it('renders the server name', () => {
    wrap(<ServerCard server={base} />);
    expect(screen.getByText('My Server')).toBeTruthy();
  });

  it('renders the deploy method badge', () => {
    wrap(<ServerCard server={base} />);
    expect(screen.getByText('docker')).toBeTruthy();
  });

  it('renders port badge', () => {
    wrap(<ServerCard server={base} />);
    expect(screen.getByText('25565/tcp')).toBeTruthy();
  });

  it('shows Start button when server is stopped', () => {
    wrap(<ServerCard server={base} />);
    expect(screen.getByTitle('Start server')).toBeTruthy();
  });

  it('shows Stop and Restart buttons when server is running', () => {
    wrap(<ServerCard server={{ ...base, state: 'running' }} />);
    expect(screen.getByTitle('Stop server')).toBeTruthy();
    expect(screen.getByTitle('Restart server')).toBeTruthy();
  });

  it('shows Console link', () => {
    wrap(<ServerCard server={base} />);
    expect(screen.getByTitle('Open console')).toBeTruthy();
  });

  it('navigates to server detail on card click', () => {
    wrap(<ServerCard server={base} />);
    const card = screen.getByText('My Server').closest('div[class]')!;
    fireEvent.click(card);
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv-1');
  });

  it('renders +N indicator when more than 3 ports', () => {
    const server = {
      ...base,
      ports: [
        { internal: 25565, external: 25565, protocol: 'tcp' },
        { internal: 25575, external: 25575, protocol: 'tcp' },
        { internal: 19132, external: 19132, protocol: 'udp' },
        { internal: 8080,  external: 8080,  protocol: 'tcp' },
      ],
    };
    wrap(<ServerCard server={server} />);
    expect(screen.getByText('+1')).toBeTruthy();
  });
});
