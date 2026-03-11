import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Shield, Key, Users, Lock, RefreshCw, Plus, CheckCircle } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { cn } from '../utils/cn';

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

function MFASection() {
  const { setupTOTP, verifyTOTP, user } = useAuthStore();
  const [setupData, setSetupData] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [verifyCode, setVerifyCode] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSetup = async () => {
    setLoading(true);
    try { const data = await setupTOTP(); setSetupData(data); }
    catch { toast.error('Failed to setup TOTP'); }
    finally { setLoading(false); }
  };

  const handleVerify = async () => {
    setLoading(true);
    try {
      await verifyTOTP(verifyCode);
      toast.success('TOTP enabled successfully!');
      setSetupData(null);
      setVerifyCode('');
    } catch { toast.error('Invalid code'); }
    finally { setLoading(false); }
  };

  return (
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
            <button
              onClick={handleSetup}
              disabled={loading}
              className="btn-ghost w-full justify-center"
            >
              <Shield className="w-4 h-4" />
              {user?.totp_enabled ? 'Regenerate TOTP' : 'Enable TOTP'}
            </button>
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
        <div className="rounded-xl p-5" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
          <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--text-primary)' }}>OIDC / SSO</h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Configure single sign-on in Settings → Authentication.
          </p>
        </div>
      </div>
    </div>
  );
}

function UsersSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/v1/admin/users').then(r => r.data),
  });
  const users = data?.users ?? [];

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/admin/users/${id}`),
    onSuccess: () => { toast.success('User deleted'); queryClient.invalidateQueries({ queryKey: ['users'] }); },
  });

  const ROLE_STYLES: Record<string, React.CSSProperties> = {
    admin:    { background: 'rgba(59,130,246,0.15)',  color: '#60a5fa'  },
    viewer:   { background: 'rgba(128,128,168,0.15)', color: '#8080a8'  },
    modder:   { background: 'rgba(168,85,247,0.15)',  color: '#c084fc'  },
    operator: { background: 'rgba(59,130,246,0.15)',  color: '#60a5fa'  },
  };

  return (
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
        <button className="btn-blue py-1.5 px-3 text-xs">
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
                      <span key={r} className="badge" style={ROLE_STYLES[r] ?? { background: 'rgba(128,128,168,0.15)', color: 'var(--text-secondary)' }}>
                        {r}
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
  );
}

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
