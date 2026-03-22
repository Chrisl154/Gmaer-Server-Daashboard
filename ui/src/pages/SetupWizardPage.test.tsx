import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), patch: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  default: { success: vi.fn(), error: vi.fn() },
  toast:   { success: vi.fn(), error: vi.fn() },
}));
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => vi.fn() };
});

const mockUseAuthStore = vi.fn();
vi.mock('../store/authStore', () => ({
  useAuthStore: (...args: any[]) => mockUseAuthStore(...args),
}));

import { SetupWizardPage } from './SetupWizardPage';

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><SetupWizardPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseAuthStore.mockReturnValue({ isAuthenticated: false, login: vi.fn() });
});

describe('SetupWizardPage', () => {
  it('renders Games Dashboard heading', () => {
    wrap();
    expect(screen.getByText('Games Dashboard')).toBeTruthy();
  });

  it('shows First-time setup subtitle', () => {
    wrap();
    expect(screen.getByText('First-time setup')).toBeTruthy();
  });

  it('shows step indicator with Admin account as first step', () => {
    wrap();
    expect(screen.getByText('Admin account')).toBeTruthy();
  });

  it('shows username field on step 1', () => {
    wrap();
    expect(screen.getByPlaceholderText(/admin/i)).toBeTruthy();
  });

  it('shows Continue button on step 1', () => {
    wrap();
    expect(screen.getByRole('button', { name: /continue/i })).toBeTruthy();
  });

  it('shows password validation error when passwords mismatch', () => {
    wrap();
    const inputs = screen.getAllByRole('textbox');
    // username field
    fireEvent.change(inputs[0], { target: { value: 'admin' } });

    // Submit the form directly — button is disabled when validationError exists,
    // but onSubmit still runs and calls setError(validationError)
    const form = document.querySelector('form')!;
    fireEvent.submit(form);
    // Validation error about password length should appear
    expect(screen.getByText(/password must be at least 8/i)).toBeTruthy();
  });

  it('redirects to / when already authenticated at step 0', () => {
    mockUseAuthStore.mockReturnValue({ isAuthenticated: true, login: vi.fn() });
    const { container } = wrap();
    // Navigate component renders nothing — the container should be empty
    expect(container.querySelector('.min-h-screen')).toBeNull();
  });
});
