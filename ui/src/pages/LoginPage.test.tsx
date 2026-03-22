import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import React from 'react';

const mockLogin = vi.fn();
const mockNavigate = vi.fn();

vi.mock('../store/authStore', () => ({
  useAuthStore: vi.fn(() => ({
    isAuthenticated: false,
    login: mockLogin,
    mfaRequired: false,
  })),
}));

vi.mock('react-hot-toast', () => ({
  default: { error: vi.fn(), success: vi.fn() },
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

import { LoginPage } from './LoginPage';
import { useAuthStore } from '../store/authStore';

const mockUseAuthStore = useAuthStore as ReturnType<typeof vi.fn>;

function wrap() {
  return render(<MemoryRouter><LoginPage /></MemoryRouter>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseAuthStore.mockReturnValue({
    isAuthenticated: false,
    login: mockLogin,
    mfaRequired: false,
  });
});

describe('LoginPage', () => {
  it('renders username and password fields', () => {
    wrap();
    expect(screen.getByPlaceholderText('admin')).toBeTruthy();
    expect(screen.getByPlaceholderText('••••••••')).toBeTruthy();
  });

  it('renders Sign in button', () => {
    wrap();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeTruthy();
  });

  it('renders OIDC/SSO option when not in MFA mode', () => {
    wrap();
    expect(screen.getByText('OIDC / SSO')).toBeTruthy();
  });

  it('calls login with username and password on submit', async () => {
    mockLogin.mockResolvedValueOnce({});
    wrap();

    fireEvent.change(screen.getByPlaceholderText('admin'), { target: { value: 'admin' } });
    fireEvent.change(screen.getByPlaceholderText('••••••••'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith('admin', 'secret', undefined);
    });
  });

  it('renders TOTP input when mfaRequired is true', () => {
    mockUseAuthStore.mockReturnValue({
      isAuthenticated: false,
      login: mockLogin,
      mfaRequired: true,
    });
    wrap();
    expect(screen.getByPlaceholderText('000000')).toBeTruthy();
    expect(screen.getByRole('button', { name: /verify code/i })).toBeTruthy();
  });

  it('hides OIDC option in MFA mode', () => {
    mockUseAuthStore.mockReturnValue({
      isAuthenticated: false,
      login: mockLogin,
      mfaRequired: true,
    });
    wrap();
    expect(screen.queryByText('OIDC / SSO')).toBeNull();
  });

  it('toggles password visibility', () => {
    wrap();
    const passwordInput = screen.getByPlaceholderText('••••••••');
    expect(passwordInput.getAttribute('type')).toBe('password');

    const toggleBtn = passwordInput.parentElement!.querySelector('button')!;
    fireEvent.click(toggleBtn);
    expect(passwordInput.getAttribute('type')).toBe('text');
  });
});
