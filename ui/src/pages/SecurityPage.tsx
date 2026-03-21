import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Shield, Key, Users, Lock, RefreshCw, Plus, CheckCircle, Eye, EyeOff, Copy, X, Download, AlertTriangle } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { cn } from '../utils/cn';

const ROLE_NAMES: Record<string, string> = {
  viewer:   'Viewer',
  operator: 'Operator',
  modder:   'Mod Manager',
  admin:    'Administrator',
};

const ROLE_DESCRIPTIONS: Record<string, string> = {
  viewer:   'Read Only',
  operator: 'Restart & Monitor',
  modder:   'Mods, Restart & Monitor',
  admin:    'Full Access',
};

const ROLE_STYLES: Record<string, React.CSSProperties> = {
  viewer:   { background: 'rgba(148,163,184,0.15)', color: '#94a3b8' },
  operator: { background: 'rgba(59,130,246,0.15)',  color: '#60a5fa' },
  modder:   { background: 'rgba(168,85,247,0.15)',  color: '#c084fc' },
  admin:    { background: 'rgba(249,115,22,0.15)',  color: '#fb923c' },
};

export function SecurityPage() {
  const { user } = useAuthStore();

  return (
    <div className="p-6 md:p-8 space-y-8 animate-page">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Security</h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Authentication, RBAC, audit trail, and secrets management
        </p>
      </div>

      <div className="space-y-6">
        <MFASection />
        <UsersSection />
        <AuditSection />
        <SecretsSection />
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────────────────── OIDCCard ── */

function OIDCCard() {
  const [enabled, setEnabled]             = useState(false);
  const [providerUrl, setProviderUrl]     = useState('');
  const [clientId, setClientId]           = useState('');
  const [clientSecret, setClientSecret]   = useState('');
  const [showSecret, setShowSecret]       = useState(false);
  const [scopes, setScopes]               = useState('openid email profile');
  const [saving, setSaving]               = useState(false);
  const [testResult, setTestResult]       = useState<'idle' | 'ok' | 'fail'>('idle');

  const callbackUrl = typeof window !== 'undefined'
    ? window.location.origin + '/auth/oidc/callback'
    : '/auth/oidc/callback';

  const formData = { enabled, provider_url: providerUrl, client_id: clientId, client_secret: clientSecret, callback_url: callbackUrl, scopes };

  const handleTest = async () => {
    setTestResult('idle');
    try {
      await api.post('/api/v1/admin/auth/oidc/test', formData);
      setTestResult('ok');
    } catch {
      setTestResult('fail');
      toast.error('OIDC test connection failed');
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.post('/api/v1/admin/auth/oidc', formData);
      toast.success('OIDC configuration saved');
    } catch {
      toast.error('Failed to save OIDC configuration');
    } finally {
      setSaving(false);
    }
  };

  const handleCopyCallback = () => {
    navigator.clipboard.writeText(callbackUrl).then(() => toast.success('Copied!'));
  };

  return (
    <div className="rounded-xl p-5 space-y-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>OIDC / Single Sign-On</h3>
          <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>OpenID Connect provider integration</p>
        </div>
        {/* Toggle switch */}
        <button
          type="button"
          onClick={() => setEnabled(v => !v)}
          className="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus:outline-none"
          style={{ background: enabled ? 'var(--primary, #f97316)' : 'rgba(148,163,184,0.3)' }}
          aria-pressed={enabled}
          aria-label="Enable OIDC"
        >
          <span
            className="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200"
            style={{ transform: enabled ? 'translateX(16px)' : 'translateX(0)' }}
          />
        </button>
      </div>

      {/* Form fields — only shown when enabled */}
      {enabled && (
        <div className="space-y-3">
          {/* Provider URL */}
          <div>
            <label className="label">Issuer URL</label>
            <input
              type="url"
              className="input"
              placeholder="https://accounts.google.com"
              value={providerUrl}
              onChange={e => setProviderUrl(e.target.value)}
            />
          </div>

          {/* Client ID */}
          <div>
            <label className="label">Client ID</label>
            <input
              type="text"
              className="input"
              placeholder="your-client-id"
              value={clientId}
              onChange={e => setClientId(e.target.value)}
            />
          </div>

          {/* Client Secret */}
          <div>
            <label className="label">Client Secret</label>
            <div className="relative">
              <input
                type={showSecret ? 'text' : 'password'}
                className="input pr-10"
                placeholder="your-client-secret"
                value={clientSecret}
                onChange={e => setClientSecret(e.target.value)}
              />
              <button
                type="button"
                onClick={() => setShowSecret(v => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors"
                style={{ color: 'var(--text-muted)' }}
                tabIndex={-1}
              >
                {showSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* Callback URL (readonly) */}
          <div>
            <label className="label">Callback URL</label>
            <div className="relative">
              <input
                type="text"
                className="input pr-10 font-mono text-xs"
                value={callbackUrl}
                readOnly
                style={{ color: 'var(--text-secondary)' }}
              />
              <button
                type="button"
                onClick={handleCopyCallback}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors"
                style={{ color: 'var(--text-muted)' }}
                tabIndex={-1}
              >
                <Copy className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Scopes */}
          <div>
            <label className="label">Scopes</label>
            <input
              type="text"
              className="input"
              value={scopes}
              onChange={e => setScopes(e.target.value)}
            />
          </div>

          {/* Test result message */}
          {testResult === 'ok' && (
            <div className="flex items-center gap-2 text-xs text-green-400 rounded-lg px-3 py-2"
              style={{ background: 'rgba(34,197,94,0.1)', border: '1px solid rgba(34,197,94,0.2)' }}>
              <CheckCircle className="w-3.5 h-3.5 shrink-0" /> Connection successful
            </div>
          )}
          {testResult === 'fail' && (
            <div className="flex items-center gap-2 text-xs text-red-400 rounded-lg px-3 py-2"
              style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.2)' }}>
              <X className="w-3.5 h-3.5 shrink-0" /> Connection failed
            </div>
          )}

          {/* Action buttons */}
          <div className="flex gap-2 pt-1">
            <button
              type="button"
              onClick={handleTest}
              className="btn-ghost flex-1 justify-center text-sm"
            >
              Test Connection
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={saving}
              className="btn-primary flex-1 justify-center text-sm"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────── RecoveryCodesModal ── */

function RecoveryCodesModal({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const handleDownload = () => {
    const text = [
      'Games Dashboard — TOTP Recovery Codes',
      'Generated: ' + new Date().toLocaleString(),
      'Each code can only be used once. Store these somewhere safe.',
      '',
      ...codes,
    ].join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'games-dashboard-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}>
      <div className="card w-full max-w-md p-6 space-y-5">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Recovery Codes</h2>
          <button onClick={onClose} className="btn-ghost p-1.5"><X className="w-4 h-4" /></button>
        </div>

        <div className="rounded-lg px-4 py-3 flex items-start gap-2 text-sm text-yellow-400"
          style={{ background: 'rgba(234,179,8,0.1)', border: '1px solid rgba(234,179,8,0.25)' }}>
          <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
          <span>Save these now — they won't be shown again. Each code works once as a backup if you lose your authenticator app.</span>
        </div>

        <div className="rounded-xl p-4 space-y-2" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
          {codes.map((code) => (
            <div key={code} className="font-mono text-sm tracking-wider text-center py-1.5 rounded"
              style={{ color: 'var(--text-primary)', background: 'var(--bg-input)' }}>
              {code}
            </div>
          ))}
        </div>

        <div className="flex gap-3">
          <button onClick={handleDownload} className="btn-ghost flex-1 justify-center">
            <Download className="w-4 h-4" /> Download .txt
          </button>
          <button onClick={onClose} className="btn-primary flex-1 justify-center">
            I've saved these
          </button>
        </div>
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────────────── MFASection ── */

function MFASection() {
  const { setupTOTP, verifyTOTP, user } = useAuthStore();
  const [setupData, setSetupData] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [verifyCode, setVerifyCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [regenCode, setRegenCode] = useState('');
  const [showRegen, setShowRegen] = useState(false);
  const [showReenrollConfirm, setShowReenrollConfirm] = useState(false);
  const [reenrollCode, setReenrollCode] = useState('');

  const { data: rcData } = useQuery({
    queryKey: ['totp-recovery-count'],
    queryFn: () => api.get('/api/v1/auth/totp/recovery-codes').then(r => r.data),
    enabled: !!user?.totp_enabled,
  });

  const regenMutation = useMutation({
    mutationFn: (totpCode: string) => api.post('/api/v1/auth/totp/recovery-codes/regenerate', { totp_code: totpCode }).then(r => r.data),
    onSuccess: (data) => {
      setRecoveryCodes(data.recovery_codes);
      setShowRegen(false);
      setRegenCode('');
    },
    onError: () => toast.error('Invalid TOTP code'),
  });

  const handleSetup = async (currentCode?: string) => {
    setLoading(true);
    try {
      const data = await setupTOTP(currentCode);
      setSetupData(data);
      setShowReenrollConfirm(false);
      setReenrollCode('');
    } catch { toast.error(currentCode ? 'Invalid TOTP code' : 'Failed to setup TOTP'); }
    finally { setLoading(false); }
  };

  const handleReenrollClick = () => {
    if (user?.totp_enabled) {
      setShowReenrollConfirm(true);
    } else {
      handleSetup();
    }
  };

  const handleVerify = async () => {
    setLoading(true);
    try {
      const result = await verifyTOTP(verifyCode);
      setRecoveryCodes(result.recovery_codes);
      setSetupData(null);
      setVerifyCode('');
    } catch { toast.error('Invalid code'); }
    finally { setLoading(false); }
  };

  return (
    <>
      {recoveryCodes && (
        <RecoveryCodesModal codes={recoveryCodes} onClose={() => setRecoveryCodes(null)} />
      )}

      <div className="card p-5 space-y-5">
        {/* Section title */}
        <div className="flex items-center gap-2 mb-1">
          <div className="w-7 h-7 rounded-lg flex items-center justify-center"
            style={{ background: 'var(--primary-subtle)' }}>
            <Shield className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
          </div>
          <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>MFA / Two-Factor Auth</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* TOTP Card */}
          <div className="rounded-xl p-5 space-y-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Authenticator App (TOTP)</h3>
                <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Google Authenticator, Authy, or 1Password</p>
              </div>
              <span className={cn(
                'badge',
                user?.totp_enabled
                  ? 'bg-green-500/15 text-green-400'
                  : 'text-gray-500'
              )} style={!user?.totp_enabled ? { background: 'rgba(128,128,168,0.15)' } : {}}>
                {user?.totp_enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            {!setupData ? (
              <div className="space-y-3">
                {showReenrollConfirm ? (
                  <div className="space-y-2">
                    <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                      Enter your current TOTP code to re-enroll:
                    </p>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={reenrollCode}
                        onChange={e => setReenrollCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                        placeholder="000000"
                        maxLength={6}
                        className="input font-mono tracking-widest text-center text-sm"
                      />
                      <button
                        onClick={() => handleSetup(reenrollCode)}
                        disabled={reenrollCode.length !== 6 || loading}
                        className="btn-primary text-xs px-3"
                      >
                        {loading ? '…' : 'Continue'}
                      </button>
                      <button onClick={() => { setShowReenrollConfirm(false); setReenrollCode(''); }} className="btn-ghost text-xs px-3">
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                <button
                  onClick={handleReenrollClick}
                  disabled={loading}
                  className="btn-ghost w-full justify-center"
                >
                  <Shield className="w-4 h-4" />
                  {user?.totp_enabled ? 'Regenerate TOTP' : 'Enable TOTP'}
                </button>
                )}

                {/* Recovery codes info — only when TOTP is enabled */}
                {user?.totp_enabled && (
                  <div className="rounded-lg px-3 py-2.5 space-y-2" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
                    <div className="flex items-center justify-between">
                      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                        Recovery codes remaining:
                        <span className="ml-1 font-semibold" style={{ color: rcData?.remaining === 0 ? '#f87171' : 'var(--text-primary)' }}>
                          {rcData?.remaining ?? '…'}
                        </span>
                      </span>
                      {rcData?.remaining === 0 && (
                        <span className="text-xs text-red-400 flex items-center gap-1">
                          <AlertTriangle className="w-3 h-3" /> None left
                        </span>
                      )}
                    </div>

                    {!showRegen ? (
                      <button
                        onClick={() => setShowRegen(true)}
                        className="btn-ghost w-full justify-center text-xs py-1.5"
                      >
                        <RefreshCw className="w-3.5 h-3.5" /> Regenerate codes
                      </button>
                    ) : (
                      <div className="space-y-2">
                        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Enter your current TOTP code to regenerate:</p>
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={regenCode}
                            onChange={e => setRegenCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                            placeholder="000000"
                            maxLength={6}
                            className="input font-mono tracking-widest text-center text-sm"
                          />
                          <button
                            onClick={() => regenMutation.mutate(regenCode)}
                            disabled={regenCode.length !== 6 || regenMutation.isPending}
                            className="btn-primary text-xs px-3"
                          >
                            {regenMutation.isPending ? '…' : 'Go'}
                          </button>
                          <button onClick={() => { setShowRegen(false); setRegenCode(''); }} className="btn-ghost text-xs px-3">
                            Cancel
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex justify-center">
                  <div className="p-3 bg-white rounded-xl shadow-lg">
                    <QRCodeSVG value={setupData.qr_code_url} size={160} />
                  </div>
                </div>
                <div className="rounded-lg p-3 space-y-1" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
                  <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Manual entry key:</p>
                  <p className="font-mono text-sm break-all" style={{ color: 'var(--text-primary)' }}>{setupData.secret}</p>
                </div>
                <div>
                  <label className="label">Verify code from app</label>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={verifyCode}
                      onChange={e => setVerifyCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                      placeholder="000000"
                      maxLength={6}
                      className="input font-mono tracking-widest text-center"
                    />
                    <button
                      onClick={handleVerify}
                      disabled={verifyCode.length !== 6 || loading}
                      className="btn-primary"
                    >
                      Verify
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* OIDC Card */}
          <OIDCCard />
        </div>
      </div>
    </>
  );
}

/* ────────────────────────────────────────────────── AddUserModal ── */

function AddUserModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole]         = useState('viewer');

  const mutation = useMutation({
    mutationFn: () => api.post('/api/v1/admin/users', { username, password, roles: [role] }),
    onSuccess: () => {
      toast.success('User created');
      queryClient.invalidateQueries({ queryKey: ['users'] });
      onClose();
    },
    onError: () => toast.error('Failed to create user'),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)' }}
      onClick={e => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="card w-full max-w-md p-6 space-y-5">
        {/* Modal header */}
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Add User</h2>
          <button onClick={onClose} className="btn-ghost p-1.5">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Fields */}
        <div className="space-y-4">
          <div>
            <label className="label">Username</label>
            <input
              type="text"
              className="input"
              placeholder="username"
              value={username}
              onChange={e => setUsername(e.target.value)}
              autoFocus
            />
          </div>

          <div>
            <label className="label">Password</label>
            <input
              type="password"
              className="input"
              placeholder="••••••••"
              value={password}
              onChange={e => setPassword(e.target.value)}
            />
          </div>

          <div>
            <label className="label">Role</label>
            <select
              className="input"
              value={role}
              onChange={e => setRole(e.target.value)}
            >
              {Object.entries(ROLE_NAMES).map(([value, label]) => (
                <option key={value} value={value}>
                  {label} — {ROLE_DESCRIPTIONS[value]}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-3 pt-1">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={() => mutation.mutate()}
            disabled={!username || !password || mutation.isPending}
            className="btn-primary flex-1 justify-center"
          >
            {mutation.isPending ? 'Creating…' : 'Create User'}
          </button>
        </div>
      </div>
    </div>
  );
}

/* ───────────────────────────────────────────────── UsersSection ── */

function UsersSection() {
  const queryClient = useQueryClient();
  const [showAddUser, setShowAddUser] = useState(false);

  const { data } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/v1/admin/users').then(r => r.data),
  });
  const users = data?.users ?? [];

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/admin/users/${id}`),
    onSuccess: () => { toast.success('User deleted'); queryClient.invalidateQueries({ queryKey: ['users'] }); },
  });

  return (
    <>
      {showAddUser && <AddUserModal onClose={() => setShowAddUser(false)} />}

      <div className="card p-5 space-y-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: 'rgba(59,130,246,0.12)' }}>
              <Users className="w-3.5 h-3.5 text-blue-400" />
            </div>
            <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
              User Management
              <span className="ml-2 text-sm font-normal" style={{ color: 'var(--text-muted)' }}>({users.length})</span>
            </h2>
          </div>
          <button className="btn-blue py-1.5 px-3 text-xs" onClick={() => setShowAddUser(true)}>
            <Plus className="w-3.5 h-3.5" /> Add User
          </button>
        </div>

        <div className="rounded-xl overflow-hidden" style={{ border: '1px solid var(--border)' }}>
          <table className="w-full text-sm">
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
                <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>User</th>
                <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>Roles</th>
                <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>MFA</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {users.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
                    No users found.
                  </td>
                </tr>
              ) : users.map((u: any, idx: number) => (
                <tr key={u.id}
                  style={{
                    borderTop: idx > 0 ? '1px solid var(--border)' : undefined,
                  }}
                  className="transition-colors"
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
                  onMouseLeave={e => (e.currentTarget.style.background = '')}
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold shrink-0"
                        style={{ background: 'var(--primary-subtle)', color: 'var(--primary)' }}>
                        {u.username[0].toUpperCase()}
                      </div>
                      <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{u.username}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1 flex-wrap">
                      {u.roles.map((r: string) => (
                        <span key={r} className="badge" style={ROLE_STYLES[r] ?? { background: 'rgba(148,163,184,0.15)', color: 'var(--text-secondary)' }}>
                          {ROLE_NAMES[r] ?? r}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {u.totp_enabled
                      ? <span className="flex items-center gap-1 text-xs text-green-400"><Shield className="w-3.5 h-3.5" /> Enabled</span>
                      : <span className="text-xs" style={{ color: 'var(--text-muted)' }}>—</span>
                    }
                  </td>
                  <td className="px-4 py-3 text-right">
                    {u.username !== 'admin' && (
                      <button
                        onClick={() => deleteMutation.mutate(u.id)}
                        className="text-xs px-2 py-1 rounded transition-colors"
                        style={{ color: '#f87171' }}
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

/* ────────────────────────────────────────────────── AuditSection ── */

function AuditSection() {
  const { data } = useQuery({
    queryKey: ['audit-log'],
    queryFn: () => api.get('/api/v1/admin/audit').then(r => r.data),
  });
  const entries = data?.audit_log ?? [];

  return (
    <div className="card p-5 space-y-5">
      <div className="flex items-center gap-2">
        <div className="w-7 h-7 rounded-lg flex items-center justify-center"
          style={{ background: 'rgba(168,85,247,0.12)' }}>
          <Key className="w-3.5 h-3.5 text-purple-400" />
        </div>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
          Audit Log
          <span className="ml-2 text-sm font-normal" style={{ color: 'var(--text-muted)' }}>({entries.length} entries)</span>
        </h2>
      </div>

      <div className="rounded-xl overflow-hidden" style={{ border: '1px solid var(--border)' }}>
        <div className="max-h-[400px] overflow-y-auto">
          {entries.length === 0 ? (
            <div className="p-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
              No audit events yet.
            </div>
          ) : (
            <div className="relative pl-8">
              {/* Timeline line */}
              <div className="absolute left-4 top-0 bottom-0 w-px" style={{ background: 'var(--border-strong)' }} />

              {entries.map((e: any, idx: number) => (
                <div key={e.id} className="relative flex items-start gap-4 py-3 pr-4"
                  style={{ borderBottom: idx < entries.length - 1 ? '1px solid var(--border)' : undefined }}>
                  {/* Timeline dot */}
                  <div className={cn(
                    'absolute left-[13px] top-4 w-2.5 h-2.5 rounded-full ring-2 shrink-0',
                    e.success
                      ? 'bg-green-400 ring-green-400/20'
                      : 'bg-red-400 ring-red-400/20'
                  )} />

                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-sm" style={{ color: 'var(--text-primary)' }}>{e.username}</span>
                      <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{e.action}</span>
                      {e.resource && (
                        <span className="badge text-xs" style={{ background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
                          {e.resource}
                        </span>
                      )}
                    </div>
                    <div className="text-xs mt-0.5 font-mono" style={{ color: 'var(--text-muted)' }}>
                      {new Date(e.timestamp).toLocaleString()}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────── SecretsSection ── */

function SecretsSection() {
  const [showRotate, setShowRotate] = useState(false);
  const rotateMutation = useMutation({
    mutationFn: () => api.post('/api/v1/admin/secrets/rotate'),
    onSuccess: () => { toast.success('Secrets rotated successfully'); setShowRotate(false); },
    onError: () => toast.error('Rotation failed'),
  });

  return (
    <div className="card p-5 space-y-5">
      <div className="flex items-center gap-2">
        <div className="w-7 h-7 rounded-lg flex items-center justify-center"
          style={{ background: 'rgba(234,179,8,0.12)' }}>
          <Lock className="w-3.5 h-3.5 text-yellow-400" />
        </div>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Secrets</h2>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Encryption at Rest */}
        <div className="rounded-xl p-5 space-y-3" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
          <h3 className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Encryption at Rest</h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            All secrets encrypted with AES-256-GCM. Local KMS by default; HashiCorp Vault supported.
          </p>
          <div className="flex items-center gap-2 text-xs text-green-400">
            <CheckCircle className="w-3.5 h-3.5" /> AES-256-GCM active
          </div>
        </div>

        {/* Key Rotation */}
        <div className="rounded-xl p-5 space-y-3" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
          <h3 className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Key Rotation</h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Rotate the master encryption key. All secrets will be re-encrypted with the new key.
          </p>
          {!showRotate ? (
            <button
              onClick={() => setShowRotate(true)}
              className="btn-ghost w-full justify-center text-yellow-400"
              style={{ borderColor: 'rgba(234,179,8,0.3)' }}
            >
              <RefreshCw className="w-4 h-4" /> Rotate Keys
            </button>
          ) : (
            <div className="space-y-3">
              <div className="rounded-lg px-3 py-2 text-xs text-red-400"
                style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.25)' }}>
                Warning: This will rotate all encryption keys. Ensure you have a backup before proceeding.
              </div>
              <div className="flex gap-2">
                <button onClick={() => setShowRotate(false)} className="btn-ghost flex-1 justify-center">Cancel</button>
                <button
                  onClick={() => rotateMutation.mutate()}
                  disabled={rotateMutation.isPending}
                  className="btn-danger flex-1 justify-center"
                >
                  {rotateMutation.isPending ? 'Rotating...' : 'Confirm Rotate'}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
