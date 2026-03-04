import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock react-hot-toast before importing the store
vi.mock('react-hot-toast', () => ({ default: { error: vi.fn() } }));

// Mock the api module
vi.mock('../utils/api', () => {
  const mockApi = {
    post: vi.fn(),
    defaults: { headers: { common: {} as Record<string, string> } },
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  };
  return { api: mockApi };
});

import { useAuthStore } from './authStore';
import { api } from '../utils/api';

const mockApi = api as unknown as {
  post: ReturnType<typeof vi.fn>;
  defaults: { headers: { common: Record<string, string> } };
};

function resetStore() {
  useAuthStore.setState({
    user: null,
    token: null,
    isAuthenticated: false,
    mfaRequired: false,
  });
  mockApi.defaults.headers.common = {};
}

beforeEach(resetStore);

// ── Initial state ─────────────────────────────────────────────────────────────

describe('authStore initial state', () => {
  it('starts unauthenticated', () => {
    const { user, token, isAuthenticated, mfaRequired } = useAuthStore.getState();
    expect(user).toBeNull();
    expect(token).toBeNull();
    expect(isAuthenticated).toBe(false);
    expect(mfaRequired).toBe(false);
  });
});

// ── login ─────────────────────────────────────────────────────────────────────

describe('login', () => {
  it('sets authenticated state on success', async () => {
    mockApi.post.mockResolvedValueOnce({
      data: {
        token: 'tok-abc',
        user: { id: 'u1', username: 'admin', roles: ['admin'], totp_enabled: false },
        mfa_required: false,
      },
    });

    const result = await useAuthStore.getState().login('admin', 'pass');
    const state = useAuthStore.getState();

    expect(result).toEqual({});
    expect(state.isAuthenticated).toBe(true);
    expect(state.token).toBe('tok-abc');
    expect(state.user?.username).toBe('admin');
    expect(state.mfaRequired).toBe(false);
  });

  it('sets mfaRequired when server signals MFA needed', async () => {
    mockApi.post.mockResolvedValueOnce({
      data: { mfa_required: true },
    });

    const result = await useAuthStore.getState().login('admin', 'pass');
    const state = useAuthStore.getState();

    expect(result).toEqual({ mfaRequired: true });
    expect(state.mfaRequired).toBe(true);
    expect(state.isAuthenticated).toBe(false);
  });

  it('sets Authorization header on success', async () => {
    mockApi.post.mockResolvedValueOnce({
      data: {
        token: 'tok-xyz',
        user: { id: 'u2', username: 'player', roles: [], totp_enabled: false },
        mfa_required: false,
      },
    });

    await useAuthStore.getState().login('player', 'pass');
    expect(mockApi.defaults.headers.common['Authorization']).toBe('Bearer tok-xyz');
  });

  it('throws and does not authenticate on error', async () => {
    const err = Object.assign(new Error('Unauthorized'), {
      response: { data: { error: 'invalid credentials' } },
    });
    mockApi.post.mockRejectedValueOnce(err);

    await expect(useAuthStore.getState().login('admin', 'wrong')).rejects.toThrow();
    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.token).toBeNull();
  });
});

// ── logout ────────────────────────────────────────────────────────────────────

describe('logout', () => {
  it('clears auth state', async () => {
    // Put store into authenticated state
    useAuthStore.setState({
      user: { id: 'u1', username: 'admin', roles: ['admin'], totp_enabled: false },
      token: 'tok-abc',
      isAuthenticated: true,
      mfaRequired: false,
    });
    mockApi.defaults.headers.common['Authorization'] = 'Bearer tok-abc';
    mockApi.post.mockResolvedValue({ data: {} });

    useAuthStore.getState().logout();
    const state = useAuthStore.getState();

    expect(state.user).toBeNull();
    expect(state.token).toBeNull();
    expect(state.isAuthenticated).toBe(false);
    expect(state.mfaRequired).toBe(false);
  });

  it('removes Authorization header', () => {
    useAuthStore.setState({ token: 'tok-abc', isAuthenticated: true });
    mockApi.defaults.headers.common['Authorization'] = 'Bearer tok-abc';
    mockApi.post.mockResolvedValue({ data: {} });

    useAuthStore.getState().logout();
    expect(mockApi.defaults.headers.common['Authorization']).toBeUndefined();
  });

  it('does not call logout API when no token', () => {
    mockApi.post.mockClear();
    useAuthStore.getState().logout();
    expect(mockApi.post).not.toHaveBeenCalled();
  });
});

// ── checkAuth ─────────────────────────────────────────────────────────────────

describe('checkAuth', () => {
  it('sets Authorization header when token exists', () => {
    useAuthStore.setState({ token: 'tok-stored' });
    useAuthStore.getState().checkAuth();
    expect(mockApi.defaults.headers.common['Authorization']).toBe('Bearer tok-stored');
  });

  it('does not set Authorization header when no token', () => {
    delete mockApi.defaults.headers.common['Authorization'];
    useAuthStore.getState().checkAuth();
    expect(mockApi.defaults.headers.common['Authorization']).toBeUndefined();
  });
});

// ── verifyTOTP ────────────────────────────────────────────────────────────────

describe('verifyTOTP', () => {
  it('clears mfaRequired on success', async () => {
    useAuthStore.setState({ mfaRequired: true });
    mockApi.post.mockResolvedValueOnce({ data: {} });

    await useAuthStore.getState().verifyTOTP('123456');
    expect(useAuthStore.getState().mfaRequired).toBe(false);
  });

  it('posts the code to the correct endpoint', async () => {
    mockApi.post.mockResolvedValueOnce({ data: {} });
    await useAuthStore.getState().verifyTOTP('654321');
    expect(mockApi.post).toHaveBeenCalledWith('/api/v1/auth/totp/verify', { code: '654321' });
  });
});

// ── setupTOTP ─────────────────────────────────────────────────────────────────

describe('setupTOTP', () => {
  it('returns secret and qr_code_url from API', async () => {
    mockApi.post.mockResolvedValueOnce({
      data: { secret: 'JBSWY3DPEHPK3PXP', qr_code_url: 'otpauth://totp/...' },
    });

    const result = await useAuthStore.getState().setupTOTP();
    expect(result.secret).toBe('JBSWY3DPEHPK3PXP');
    expect(result.qr_code_url).toContain('otpauth://');
  });
});
