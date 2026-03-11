import React, { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Shield, Eye, EyeOff, Loader2, Gamepad2, Zap, Lock, Users } from 'lucide-react';
import { useAuthStore } from '../store/authStore';
import toast from 'react-hot-toast';
import { cn } from '../utils/cn';

export function LoginPage() {
  const { isAuthenticated, login, mfaRequired } = useAuthStore();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);

  if (isAuthenticated) return <Navigate to="/" replace />;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      await login(username, password, mfaRequired ? totpCode : undefined);
    } catch (err: any) {
      toast.error(err.response?.data?.error ?? 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="min-h-screen flex"
      style={{ background: 'var(--bg-page)' }}
    >
      {/* Left: Login Form */}
      <div className="flex-1 flex flex-col items-center justify-center p-8 relative z-10">
        {/* Logo */}
        <div className="w-full max-w-sm">
          <div className="flex flex-col items-center mb-10">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5"
              style={{
                background: 'linear-gradient(135deg, #f97316, #ea580c)',
                boxShadow: '0 8px 32px rgba(249,115,22,0.35)',
              }}
            >
              <Gamepad2 className="w-8 h-8 text-white" />
            </div>
            <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              Games Dashboard
            </h1>
            <p className="text-sm mt-1.5" style={{ color: 'var(--text-secondary)' }}>
              {mfaRequired ? 'Two-factor authentication required' : 'Sign in to your account'}
            </p>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            {!mfaRequired ? (
              <>
                <div>
                  <label className="label">Username</label>
                  <input
                    type="text"
                    value={username}
                    onChange={e => setUsername(e.target.value)}
                    required
                    autoFocus
                    placeholder="admin"
                    className="input"
                  />
                </div>

                <div>
                  <label className="label">Password</label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      required
                      placeholder="••••••••"
                      className="input pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 transition-colors"
                      style={{ color: 'var(--text-muted)' }}
                      onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-secondary)')}
                      onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
                    >
                      {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
              </>
            ) : (
              <div>
                <div
                  className="flex items-center gap-3 mb-5 p-3.5 rounded-xl"
                  style={{
                    background: 'rgba(249,115,22,0.08)',
                    border: '1px solid rgba(249,115,22,0.2)',
                  }}
                >
                  <Shield className="w-4 h-4 shrink-0" style={{ color: '#fb923c' }} />
                  <span className="text-sm" style={{ color: '#fb923c' }}>
                    Enter your 6-digit authenticator code
                  </span>
                </div>
                <label className="label">TOTP Code</label>
                <input
                  type="text"
                  value={totpCode}
                  onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  required
                  autoFocus
                  placeholder="000000"
                  maxLength={6}
                  className="input font-mono tracking-[0.5em] text-center text-xl"
                />
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="btn-primary w-full justify-center py-2.5 mt-2"
            >
              {loading && <Loader2 className="w-4 h-4 animate-spin" />}
              {mfaRequired ? 'Verify Code' : 'Sign in'}
            </button>
          </form>

          {/* OIDC */}
          {!mfaRequired && (
            <div className="mt-6 space-y-3">
              <div className="relative flex items-center">
                <div className="flex-1 border-t" style={{ borderColor: 'var(--border)' }} />
                <span className="px-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                  or continue with
                </span>
                <div className="flex-1 border-t" style={{ borderColor: 'var(--border)' }} />
              </div>
              <button
                type="button"
                className="btn-ghost w-full justify-center"
                onClick={() => (window.location.href = '/api/v1/auth/oidc')}
              >
                <Shield className="w-4 h-4" />
                OIDC / SSO
              </button>
            </div>
          )}

          {/* Footer */}
          <p className="text-center text-xs mt-8" style={{ color: 'var(--text-muted)' }}>
            Powered by Games Dashboard
          </p>
        </div>
      </div>

      {/* Right: Decorative Panel */}
      <div
        className="hidden lg:flex flex-col items-center justify-center relative overflow-hidden w-[520px] xl:w-[600px] shrink-0"
        style={{
          background: 'linear-gradient(135deg, #1a0a00 0%, #0f0a1a 40%, #030318 100%)',
          borderLeft: '1px solid var(--border)',
        }}
      >
        {/* Background blobs */}
        <div
          className="absolute top-[-80px] right-[-80px] w-[360px] h-[360px] rounded-full opacity-20"
          style={{ background: 'radial-gradient(circle, #f97316 0%, transparent 70%)' }}
        />
        <div
          className="absolute bottom-[-60px] left-[-60px] w-[280px] h-[280px] rounded-full opacity-15"
          style={{ background: 'radial-gradient(circle, #3b82f6 0%, transparent 70%)' }}
        />
        <div
          className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[500px] h-[500px] rounded-full opacity-5"
          style={{ background: 'radial-gradient(circle, #f97316 0%, transparent 60%)' }}
        />

        {/* Floating circles decoration */}
        <div
          className="absolute top-16 left-10 w-10 h-10 rounded-full opacity-30"
          style={{ background: 'linear-gradient(135deg, #f97316, #ea580c)', boxShadow: '0 0 20px rgba(249,115,22,0.4)' }}
        />
        <div
          className="absolute top-32 right-16 w-6 h-6 rounded-full opacity-20"
          style={{ background: '#3b82f6' }}
        />
        <div
          className="absolute bottom-24 right-10 w-14 h-14 rounded-full opacity-20"
          style={{ background: 'linear-gradient(135deg, #3b82f6, #1d4ed8)' }}
        />
        <div
          className="absolute bottom-48 left-20 w-4 h-4 rounded-full opacity-25"
          style={{ background: '#f97316' }}
        />

        {/* Central content */}
        <div className="relative z-10 text-center px-12">
          <div
            className="w-24 h-24 rounded-3xl flex items-center justify-center mx-auto mb-8"
            style={{
              background: 'linear-gradient(135deg, rgba(249,115,22,0.2), rgba(234,88,12,0.1))',
              border: '1px solid rgba(249,115,22,0.25)',
              boxShadow: '0 0 60px rgba(249,115,22,0.15)',
            }}
          >
            <Gamepad2 className="w-12 h-12" style={{ color: '#f97316' }} />
          </div>

          <h2 className="text-3xl font-bold mb-3 text-gradient-orange">
            Game Server Control
          </h2>
          <p className="text-base mb-10" style={{ color: 'var(--text-secondary)' }}>
            Deploy, monitor, and manage your game servers from a single unified dashboard.
          </p>

          {/* Feature list */}
          <div className="space-y-4 text-left">
            {[
              { icon: Zap, text: 'One-click server deployment', color: '#f97316' },
              { icon: Lock, text: 'Secure TLS-encrypted API', color: '#3b82f6' },
              { icon: Users, text: 'Multi-user access control', color: '#22c55e' },
              { icon: Shield, text: 'TOTP two-factor authentication', color: '#a855f7' },
            ].map(({ icon: Icon, text, color }) => (
              <div key={text} className="flex items-center gap-4">
                <div
                  className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                  style={{ background: `${color}18`, border: `1px solid ${color}30` }}
                >
                  <Icon className="w-4 h-4" style={{ color }} />
                </div>
                <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  {text}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
