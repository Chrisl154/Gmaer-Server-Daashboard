import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { api } from '../utils/api';
import toast from 'react-hot-toast';

interface User {
  id: string;
  username: string;
  roles: string[];
  totp_enabled: boolean;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  mfaRequired: boolean;
  login: (username: string, password: string, totpCode?: string, recoveryCode?: string) => Promise<{ mfaRequired?: boolean }>;
  loginWithToken: (token: string, user: User) => void;
  logout: () => void;
  checkAuth: () => void;
  setupTOTP: () => Promise<{ secret: string; qr_code_url: string }>;
  verifyTOTP: (code: string) => Promise<{ recovery_codes: string[] }>;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isAuthenticated: false,
      mfaRequired: false,

      login: async (username, password, totpCode, recoveryCode) => {
        try {
          const response = await api.post('/api/v1/auth/login', {
            username,
            password,
            totp_code: totpCode,
            recovery_code: recoveryCode,
          });

          const { token, user, mfa_required } = response.data;

          if (mfa_required) {
            set({ mfaRequired: true });
            return { mfaRequired: true };
          }

          api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
          set({ user, token, isAuthenticated: true, mfaRequired: false });
          return {};
        } catch (err: any) {
          const msg = err.response?.data?.error ?? 'Login failed';
          toast.error(msg);
          throw err;
        }
      },

      loginWithToken: (token, user) => {
        api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
        set({ user, token, isAuthenticated: true, mfaRequired: false });
      },

      logout: () => {
        const { token } = get();
        if (token) {
          api.post('/api/v1/auth/logout').catch(() => {});
        }
        delete api.defaults.headers.common['Authorization'];
        set({ user: null, token: null, isAuthenticated: false, mfaRequired: false });
      },

      checkAuth: () => {
        const { token } = get();
        if (token) {
          api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
        }
      },

      setupTOTP: async () => {
        const response = await api.post('/api/v1/auth/totp/setup');
        return response.data;
      },

      verifyTOTP: async (code) => {
        const response = await api.post('/api/v1/auth/totp/verify', { code });
        set(state => ({
          mfaRequired: false,
          user: state.user ? { ...state.user, totp_enabled: true } : state.user,
        }));
        return response.data;
      },
    }),
    {
      name: 'games-dashboard-auth',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);
