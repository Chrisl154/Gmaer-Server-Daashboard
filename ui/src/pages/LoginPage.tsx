import React, { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Shield, Activity, Eye, EyeOff, Loader2 } from 'lucide-react';
import { useAuthStore } from '../store/authStore';
import { clsx } from 'clsx';

export function LoginPage() {
  const { isAuthenticated, login, mfaRequired } = useAuthStore();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  if (isAuthenticated) return <Navigate to="/" replace />;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await login(username, password, mfaRequired ? totpCode : undefined);
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a] flex items-center justify-center p-4">
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="flex flex-col items-center mb-8">
          <div className="w-14 h-14 bg-blue-600/20 border border-blue-600/30 rounded-2xl flex items-center justify-center mb-4">
            <Activity className="w-7 h-7 text-blue-400" />
          </div>
          <h1 className="text-xl font-semibold text-gray-100">Games Dashboard</h1>
          <p className="text-sm text-gray-400 mt-1">Sign in to your account</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {!mfaRequired ? (
            <>
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">Username</label>
                <input
                  type="text"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  required
                  autoFocus
                  placeholder="admin"
                  className="w-full bg-[#141414] border border-[#252525] rounded-lg px-3 py-2.5 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500 transition-colors"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">Password</label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    required
                    placeholder="••••••••"
                    className="w-full bg-[#141414] border border-[#252525] rounded-lg px-3 py-2.5 pr-10 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500 transition-colors"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
                  >
                    {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>
            </>
          ) : (
            <div>
              <div className="flex items-center gap-2 mb-4 p-3 bg-blue-500/10 border border-blue-500/20 rounded-lg">
                <Shield className="w-4 h-4 text-blue-400 shrink-0" />
                <span className="text-xs text-blue-300">Enter your authenticator code</span>
              </div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">
                TOTP Code
              </label>
              <input
                type="text"
                value={totpCode}
                onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                required
                autoFocus
                placeholder="000000"
                maxLength={6}
                className="w-full bg-[#141414] border border-[#252525] rounded-lg px-3 py-2.5 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500 transition-colors font-mono tracking-widest text-center text-lg"
              />
            </div>
          )}

          {error && (
            <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading}
            className={clsx(
              'w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors',
              loading
                ? 'bg-blue-600/50 text-blue-300 cursor-not-allowed'
                : 'bg-blue-600 hover:bg-blue-700 text-white'
            )}
          >
            {loading && <Loader2 className="w-4 h-4 animate-spin" />}
            {mfaRequired ? 'Verify' : 'Sign in'}
          </button>
        </form>

        {/* OIDC options */}
        <div className="mt-6 space-y-2">
          <div className="relative flex items-center">
            <div className="flex-1 border-t border-[#1a1a1a]" />
            <span className="px-3 text-xs text-gray-500">or continue with</span>
            <div className="flex-1 border-t border-[#1a1a1a]" />
          </div>
          <button
            type="button"
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-[#141414] hover:bg-[#1a1a1a] border border-[#252525] rounded-lg text-sm text-gray-300 transition-colors"
            onClick={() => window.location.href = '/api/v1/auth/oidc'}
          >
            <Shield className="w-4 h-4" />
            OIDC / SSO
          </button>
        </div>
      </div>
    </div>
  );
}
