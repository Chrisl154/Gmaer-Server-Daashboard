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
vi.mock('qrcode.react', () => ({
  QRCodeSVG: () => <div data-testid="qrcode" />,
}));
vi.mock('../store/authStore', () => ({
  useAuthStore: () => ({
    user: { username: 'admin', role: 'admin', totp_enabled: false },
    setupTOTP: vi.fn(),
    verifyTOTP: vi.fn(),
  }),
}));

import { SecurityPage } from './SecurityPage';
import { api } from '../utils/api';

const mockGet = api.get as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><SecurityPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGet.mockImplementation((url: string) => {
    if (url.includes('users')) return Promise.resolve({ data: { users: [] } });
    if (url.includes('audit')) return Promise.resolve({ data: { audit_log: [] } });
    return Promise.resolve({ data: {} });
  });
});

describe('SecurityPage', () => {
  it('renders page heading', () => {
    wrap();
    expect(screen.getByText('Security')).toBeTruthy();
  });

  it('renders page subtitle', () => {
    wrap();
    expect(screen.getByText(/authentication, rbac, audit trail/i)).toBeTruthy();
  });

  it('renders MFA section', () => {
    wrap();
    expect(screen.getByText('MFA / Two-Factor Auth')).toBeTruthy();
  });

  it('renders User Management section', () => {
    wrap();
    expect(screen.getByText('User Management')).toBeTruthy();
  });

  it('renders Audit Log section', () => {
    wrap();
    expect(screen.getByText('Audit Log')).toBeTruthy();
  });

  it('renders Secrets section', () => {
    wrap();
    expect(screen.getByText('Secrets')).toBeTruthy();
  });

  it('renders Add User button', () => {
    wrap();
    expect(screen.getByRole('button', { name: /add user/i })).toBeTruthy();
  });

  it('shows No users found when users list is empty', async () => {
    wrap();
    await waitFor(() => expect(screen.getByText('No users found.')).toBeTruthy());
  });

  it('shows No audit events when audit log is empty', async () => {
    wrap();
    await waitFor(() => expect(screen.getByText('No audit events yet.')).toBeTruthy());
  });

  it('opens Add User modal on button click', async () => {
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /add user/i }));
    await waitFor(() => expect(screen.getByText('Create User')).toBeTruthy());
  });
});
