import React, { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Settings, Shield, Server, HardDrive, Network, Activity,
  Plus, Trash2, User, QrCode, Info, Loader2, Key, Save,
  Database, RefreshCw, Download, GitBranch, CheckCircle2, AlertCircle, Bell,
  Copy, Check, X,
} from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { toast } from 'react-hot-toast';
import { cn } from '../utils/cn';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { useSystemStatus } from '../hooks/useServers';
import type { DaemonSettings, SettingsPatch } from '../types';

type Section = 'general' | 'users' | 'tls' | 'storage' | 'networking' | 'monitoring' | 'updates' | 'notifications';

const NAV: { id: Section; icon: React.FC<any>; label: string }[] = [
  { id: 'general',       icon: Settings,  label: 'General'       },
  { id: 'users',         icon: User,      label: 'Users & Auth'  },
  { id: 'tls',           icon: Shield,    label: 'TLS'           },
  { id: 'storage',       icon: HardDrive, label: 'Storage'       },
  { id: 'networking',    icon: Network,   label: 'Networking'    },
  { id: 'monitoring',    icon: Activity,  label: 'Monitoring'    },
  { id: 'updates',       icon: Download,  label: 'Updates'       },
  { id: 'notifications', icon: Bell,      label: 'Notifications' },
];

// ── Shared hook ─────────────────────────────────────────────────────────────

function useSettings() {
  return useQuery<DaemonSettings>({
    queryKey: ['settings'],
    queryFn: () => api.get('/api/v1/admin/settings').then(r => r.data),
  });
}

function usePatchSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: SettingsPatch) =>
      api.patch('/api/v1/admin/settings', patch).then(r => r.data),
    onSuccess: () => {
      toast.success('Settings saved');
      qc.invalidateQueries({ queryKey: ['settings'] });
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Save failed'),
  });
}

// ── General section ──────────────────────────────────────────────────────────

function GeneralSection() {
  const { data: status } = useSystemStatus();

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>General</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Daemon status and system information</p>
      </div>

      {status && (
        <div className="card p-5 space-y-4">
          <h3 className="label">System Status</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[
              ['Version', status.version],
              ['Uptime', formatUptime(status.uptime_seconds)],
              ['Status', status.healthy ? 'Healthy' : 'Degraded'],
              ['Start Time', new Date(status.start_time).toLocaleString()],
            ].map(([label, value]) => (
              <div key={label} className="flex items-center justify-between rounded-lg px-3 py-2.5"
                style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{label}</span>
                <span className={cn(
                  'text-sm font-medium',
                  label === 'Status'
                    ? status.healthy ? 'text-green-400' : 'text-red-400'
                    : ''
                )} style={label !== 'Status' ? { color: 'var(--text-primary)' } : {}}>
                  {value}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="card p-4 flex items-start gap-3">
        <Info className="w-4 h-4 text-blue-400 shrink-0 mt-0.5" />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Advanced configuration can be edited in{' '}
          <code className="font-mono text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
            /etc/games-dashboard/daemon.yaml
          </code>.
          Restart the daemon after making changes.
        </p>
      </div>
    </div>
  );
}

// ── Users section ────────────────────────────────────────────────────────────

function UsersSection() {
  const qc = useQueryClient();
  const { user: currentUser, setupTOTP, verifyTOTP } = useAuthStore();
  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<any | null>(null);
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [totpLoading, setTotpLoading] = useState(false);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [regenCode, setRegenCode] = useState('');
  const [regenLoading, setRegenLoading] = useState(false);
  const [showRegen, setShowRegen] = useState(false);
  const [secretCopied, setSecretCopied] = useState(false);

  const { data } = useQuery<{ users: any[] }>({
    queryKey: ['users'],
    queryFn: () => api.get('/api/v1/admin/users').then(r => r.data),
  });

  const { data: serversData } = useQuery<{ servers: any[] }>({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/admin/users/${id}`),
    onSuccess: () => { toast.success('User deleted'); qc.invalidateQueries({ queryKey: ['users'] }); },
    onError: () => toast.error('Delete failed'),
  });

  const users = data?.users ?? [];
  const allServers: any[] = serversData?.servers ?? [];

  const { data: codesData, refetch: refetchCodes } = useQuery<{ remaining: number }>({
    queryKey: ['totp-codes-count'],
    queryFn: () => api.get('/api/v1/auth/totp/recovery-codes').then(r => r.data),
    enabled: !!currentUser?.totp_enabled,
  });

  const handleSetupTOTP = async () => {
    setTotpLoading(true);
    try {
      const result = await setupTOTP();
      setTotpSetup(result);
    } catch {
      toast.error('Failed to setup TOTP');
    } finally {
      setTotpLoading(false);
    }
  };

  const handleVerifyTOTP = async () => {
    try {
      const resp = await verifyTOTP(totpCode);
      setTotpSetup(null);
      setTotpCode('');
      setRecoveryCodes(resp.recovery_codes);
    } catch {
      toast.error('Invalid TOTP code');
    }
  };

  const handleRegen = async () => {
    setRegenLoading(true);
    try {
      const { data } = await api.post('/api/v1/auth/totp/recovery-codes/regenerate', { totp_code: regenCode });
      setRecoveryCodes(data.recovery_codes);
      setShowRegen(false);
      setRegenCode('');
      refetchCodes();
    } catch {
      toast.error('Invalid TOTP code');
    } finally {
      setRegenLoading(false);
    }
  };

  const copySecret = () => {
    if (!totpSetup) return;
    navigator.clipboard.writeText(totpSetup.secret);
    setSecretCopied(true);
    setTimeout(() => setSecretCopied(false), 2000);
  };

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Users & Auth</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Manage user accounts and authentication</p>
      </div>

      <div className="card p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="label mb-0">User Accounts</h3>
          <button
            onClick={() => setShowCreate(true)}
            className="btn-blue py-1.5 px-3 text-xs"
          >
            <Plus className="w-3.5 h-3.5" /> Add User
          </button>
        </div>

        <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
          {users.length === 0 ? (
            <p className="py-4 text-sm text-center" style={{ color: 'var(--text-muted)' }}>No users found.</p>
          ) : (
            users.map((u: any) => (
              <div key={u.id} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-semibold"
                    style={{ background: 'rgba(249,115,22,0.15)', color: 'var(--primary)' }}>
                    {u.username?.[0]?.toUpperCase()}
                  </div>
                  <div>
                    <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{u.username}</div>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      {u.roles?.map((r: string) => (
                        <span key={r} className="badge"
                          style={{
                            background: r === 'admin' ? 'rgba(59,130,246,0.15)' : r === 'operator' ? 'rgba(249,115,22,0.15)' : 'rgba(128,128,168,0.15)',
                            color: r === 'admin' ? '#60a5fa' : r === 'operator' ? 'var(--primary)' : 'var(--text-secondary)',
                          }}>
                          {r}
                        </span>
                      ))}
                      {u.totp_enabled && (
                        <span className="text-xs text-green-400 flex items-center gap-1">
                          <Shield className="w-3 h-3" /> MFA
                        </span>
                      )}
                      {u.allowed_servers?.length > 0 && (
                        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                          {u.allowed_servers.length} server{u.allowed_servers.length !== 1 ? 's' : ''}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  {u.id !== currentUser?.id && (
                    <button
                      onClick={() => setEditUser(u)}
                      className="p-1.5 rounded-lg transition-colors"
                      style={{ color: 'var(--text-muted)' }}
                      title="Edit user"
                      onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
                      onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
                    >
                      <Settings className="w-4 h-4" />
                    </button>
                  )}
                  {u.id !== currentUser?.id && (
                    <button
                      onClick={() => deleteMutation.mutate(u.id)}
                      disabled={deleteMutation.isPending}
                      className="p-1.5 rounded-lg transition-colors disabled:opacity-50"
                      style={{ color: 'var(--text-muted)' }}
                      onMouseEnter={e => (e.currentTarget.style.color = '#f87171')}
                      onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* TOTP Setup */}
      <div className="card p-5 space-y-4">
        <h3 className="label">Two-Factor Authentication</h3>
        {currentUser?.totp_enabled ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-sm text-green-400">
              <Shield className="w-4 h-4" /> TOTP is enabled for your account
            </div>
            {codesData !== undefined && (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                <span className="font-medium" style={{ color: codesData.remaining <= 2 ? 'var(--error)' : 'var(--text-primary)' }}>
                  {codesData.remaining}
                </span> recovery code{codesData.remaining !== 1 ? 's' : ''} remaining
              </p>
            )}
            {!showRegen ? (
              <button onClick={() => setShowRegen(true)} className="btn-ghost text-sm">
                <RefreshCw className="w-3.5 h-3.5" /> Regenerate recovery codes
              </button>
            ) : (
              <div className="space-y-3 p-4 rounded-lg" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                <p className="text-sm font-medium">Confirm with your current TOTP code</p>
                <div className="flex items-center gap-3">
                  <input
                    type="text"
                    value={regenCode}
                    onChange={e => setRegenCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    placeholder="000000"
                    maxLength={6}
                    className="input w-32 font-mono text-center tracking-widest"
                  />
                  <button onClick={handleRegen} disabled={regenCode.length !== 6 || regenLoading} className="btn-blue">
                    {regenLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Generate'}
                  </button>
                  <button onClick={() => { setShowRegen(false); setRegenCode(''); }} className="btn-ghost">Cancel</button>
                </div>
              </div>
            )}
          </div>
        ) : totpSetup ? (
          <div className="space-y-5">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Scan the QR code with your authenticator app (Google Authenticator, Authy, 1Password, etc.)
              then enter the 6-digit code to confirm.
            </p>

            {/* QR code */}
            <div className="flex gap-6 items-start flex-wrap">
              <div className="rounded-xl p-3" style={{ background: '#fff', display: 'inline-block' }}>
                <QRCodeSVG value={totpSetup.qr_code_url} size={180} level="M" />
              </div>
              <div className="space-y-2 flex-1 min-w-0">
                <p className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Can't scan? Enter this key manually:
                </p>
                <div className="flex items-center gap-2">
                  <code className="text-sm font-mono px-2 py-1 rounded break-all"
                    style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)', border: '1px solid var(--border)' }}>
                    {totpSetup.secret.match(/.{1,4}/g)?.join(' ')}
                  </code>
                  <button onClick={copySecret} className="p-1.5 rounded-lg shrink-0"
                    style={{ color: secretCopied ? '#4ade80' : 'var(--text-muted)' }}
                    title="Copy secret key">
                    {secretCopied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              </div>
            </div>

            {/* Verify */}
            <div className="flex items-center gap-3">
              <input
                type="text"
                value={totpCode}
                onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                maxLength={6}
                className="input w-32 font-mono text-center tracking-widest"
                autoFocus
              />
              <button onClick={handleVerifyTOTP} disabled={totpCode.length !== 6} className="btn-blue">
                Verify & Enable
              </button>
              <button onClick={() => { setTotpSetup(null); setTotpCode(''); }} className="btn-ghost">
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button onClick={handleSetupTOTP} disabled={totpLoading} className="btn-ghost">
            {totpLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <QrCode className="w-4 h-4" />}
            Set up TOTP
          </button>
        )}
      </div>

      {/* API Keys */}
      <APIKeysCard />

      {/* Recovery codes modal */}
      {recoveryCodes && (
        <RecoveryCodesModal
          codes={recoveryCodes}
          onClose={() => { setRecoveryCodes(null); refetchCodes(); }}
        />
      )}

      {showCreate && (
        <CreateUserModal
          servers={allServers}
          onClose={() => setShowCreate(false)}
          onCreated={() => qc.invalidateQueries({ queryKey: ['users'] })}
        />
      )}
      {editUser && (
        <EditUserModal
          user={editUser}
          servers={allServers}
          onClose={() => setEditUser(null)}
          onSaved={() => { qc.invalidateQueries({ queryKey: ['users'] }); setEditUser(null); }}
        />
      )}
    </div>
  );
}

function ServerAccessPicker({ servers, selected, onChange }: {
  servers: any[];
  selected: string[];
  onChange: (ids: string[]) => void;
}) {
  const toggle = (id: string) => {
    if (selected.includes(id)) {
      onChange(selected.filter(s => s !== id));
    } else {
      onChange([...selected, id]);
    }
  };
  return (
    <div>
      <div className="flex items-center justify-between mb-1.5">
        <label className="label mb-0">Server Access</label>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {selected.length === 0 ? 'All servers' : `${selected.length} selected`}
        </span>
      </div>
      {servers.length === 0 ? (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>No servers yet — user will have access to all.</p>
      ) : (
        <div className="rounded-lg overflow-hidden divide-y max-h-40 overflow-y-auto"
          style={{ border: '1px solid var(--border)', borderColor: 'var(--border)', background: 'var(--bg-base)' }}>
          {servers.map((sv: any) => (
            <label key={sv.id} className="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-black/10 transition-colors">
              <input
                type="checkbox"
                checked={selected.includes(sv.id)}
                onChange={() => toggle(sv.id)}
                className="w-3.5 h-3.5 accent-orange-500"
              />
              <span className="text-sm" style={{ color: 'var(--text-primary)' }}>{sv.name}</span>
              <span className="text-xs ml-auto" style={{ color: 'var(--text-muted)' }}>{sv.adapter}</span>
            </label>
          ))}
        </div>
      )}
      {selected.length > 0 && (
        <button onClick={() => onChange([])} className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
          Clear (grant all-server access)
        </button>
      )}
    </div>
  );
}

// ── API Keys card ─────────────────────────────────────────────────────────────

interface APIKey {
  id: string;
  name: string;
  prefix: string;
  roles: string[];
  created_at: string;
  last_used?: string;
  expires_at?: string;
}

function APIKeysCard() {
  const { user: currentUser } = useAuthStore();
  const qc = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [newToken, setNewToken] = useState('');
  const [tokenCopied, setTokenCopied] = useState(false);

  const { data, refetch } = useQuery<{ api_keys: APIKey[] }>({
    queryKey: ['api-keys'],
    queryFn: () => api.get('/api/v1/auth/api-keys').then(r => r.data),
  });
  const keys = data?.api_keys ?? [];

  const revoke = async (id: string) => {
    try {
      await api.delete(`/api/v1/auth/api-keys/${id}`);
      refetch();
      toast.success('API key revoked');
    } catch { toast.error('Failed to revoke key'); }
  };

  const copyToken = () => {
    navigator.clipboard.writeText(newToken);
    setTokenCopied(true);
    setTimeout(() => setTokenCopied(false), 2000);
  };

  return (
    <div className="card p-5 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="label mb-0">API Keys</h3>
        <button onClick={() => setShowCreate(true)} className="btn-blue py-1.5 px-3 text-xs">
          <Plus className="w-3.5 h-3.5" /> New key
        </button>
      </div>
      <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
        Long-lived tokens for scripts and external automation. Each key inherits your current roles.
      </p>

      {/* One-time token reveal */}
      {newToken && (
        <div className="rounded-lg p-4 space-y-3" style={{ background: 'rgba(34,197,94,0.08)', border: '1px solid rgba(34,197,94,0.3)' }}>
          <p className="text-sm font-medium text-green-400 flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4" /> Key created — copy it now, it won't be shown again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs font-mono px-3 py-2 rounded break-all"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)', border: '1px solid var(--border)' }}>
              {newToken}
            </code>
            <button onClick={copyToken} className="p-2 rounded-lg shrink-0"
              style={{ color: tokenCopied ? '#4ade80' : 'var(--text-muted)' }}>
              {tokenCopied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
          <button onClick={() => setNewToken('')} className="text-xs" style={{ color: 'var(--text-muted)' }}>
            I've copied it — dismiss
          </button>
        </div>
      )}

      {/* Keys table */}
      {keys.length === 0 && !newToken ? (
        <p className="text-sm text-center py-4" style={{ color: 'var(--text-muted)' }}>No API keys yet.</p>
      ) : (
        <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
          {keys.map(k => (
            <div key={k.id} className="flex items-center justify-between py-3 first:pt-0 last:pb-0 gap-4">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <Key className="w-3.5 h-3.5 shrink-0" style={{ color: 'var(--primary)' }} />
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{k.name}</span>
                  <code className="text-xs font-mono px-1.5 py-0.5 rounded"
                    style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)' }}>
                    {k.prefix}…
                  </code>
                </div>
                <div className="flex items-center gap-3 mt-1 flex-wrap">
                  {k.roles?.map(r => (
                    <span key={r} className="badge text-xs"
                      style={{ background: 'rgba(249,115,22,0.12)', color: 'var(--primary)' }}>{r}</span>
                  ))}
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    Created {new Date(k.created_at).toLocaleDateString()}
                  </span>
                  {k.last_used && (
                    <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                      Last used {new Date(k.last_used).toLocaleDateString()}
                    </span>
                  )}
                  {k.expires_at && (
                    <span className="text-xs" style={{ color: new Date(k.expires_at) < new Date() ? 'var(--error)' : 'var(--text-muted)' }}>
                      {new Date(k.expires_at) < new Date() ? 'Expired' : `Expires ${new Date(k.expires_at).toLocaleDateString()}`}
                    </span>
                  )}
                </div>
              </div>
              <button onClick={() => revoke(k.id)} className="p-1.5 rounded-lg shrink-0 transition-colors"
                style={{ color: 'var(--text-muted)' }}
                title="Revoke key"
                onMouseEnter={e => (e.currentTarget.style.color = '#f87171')}
                onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}>
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      )}

      {showCreate && (
        <CreateAPIKeyModal
          userRoles={currentUser?.roles ?? []}
          onClose={() => setShowCreate(false)}
          onCreated={(token) => { setNewToken(token); refetch(); setShowCreate(false); }}
        />
      )}
    </div>
  );
}

function CreateAPIKeyModal({ userRoles, onClose, onCreated }: {
  userRoles: string[];
  onClose: () => void;
  onCreated: (token: string) => void;
}) {
  const [name, setName] = useState('');
  const [expiry, setExpiry] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async () => {
    if (!name.trim()) return;
    setLoading(true);
    try {
      const body: Record<string, unknown> = { name: name.trim(), roles: userRoles };
      if (expiry) body.expires_at = new Date(expiry).toISOString();
      const { data } = await api.post('/api/v1/auth/api-keys', body);
      onCreated(data.token);
    } catch { toast.error('Failed to create API key'); }
    finally { setLoading(false); }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-sm rounded-2xl p-6 space-y-4"
        style={{ background: 'var(--bg-card)', border: '1px solid var(--border)' }}>
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">New API Key</h2>
          <button onClick={onClose} className="p-1.5 rounded-lg" style={{ color: 'var(--text-muted)' }}>
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="space-y-3">
          <div>
            <label className="label mb-1">Name</label>
            <input value={name} onChange={e => setName(e.target.value)}
              placeholder="e.g. GitHub Actions, backup script"
              className="input w-full" autoFocus />
          </div>
          <div>
            <label className="label mb-1">Expiry <span style={{ color: 'var(--text-muted)' }}>(optional)</span></label>
            <input type="date" value={expiry} onChange={e => setExpiry(e.target.value)}
              className="input w-full"
              min={new Date().toISOString().split('T')[0]} />
          </div>
          <div>
            <p className="label mb-1">Roles</p>
            <div className="flex gap-2 flex-wrap">
              {userRoles.map(r => (
                <span key={r} className="badge text-xs"
                  style={{ background: 'rgba(249,115,22,0.12)', color: 'var(--primary)' }}>{r}</span>
              ))}
            </div>
            <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>Inherits your current roles.</p>
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button onClick={submit} disabled={!name.trim() || loading} className="btn-primary flex-1 justify-center">
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Key className="w-4 h-4" />}
            Create key
          </button>
          <button onClick={onClose} className="btn-ghost px-4">Cancel</button>
        </div>
      </div>
    </div>
  );
}

// ── Recovery Codes Modal ──────────────────────────────────────────────────────

function RecoveryCodesModal({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const [copied, setCopied] = useState(false);

  const copyAll = () => {
    const text = [
      'Games Dashboard — Recovery Codes',
      `Generated: ${new Date().toLocaleString()}`,
      '',
      'Keep these codes in a safe place. Each code can only be used once.',
      '',
      ...codes,
    ].join('\n');
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const download = () => {
    const text = [
      'Games Dashboard — Two-Factor Authentication Recovery Codes',
      `Generated: ${new Date().toLocaleString()}`,
      '',
      'Keep these codes in a secure location.',
      'Each code can only be used once to sign in if you lose access to your authenticator app.',
      '',
      ...codes.map((c, i) => `${(i + 1).toString().padStart(2, ' ')}. ${c}`),
      '',
      'After using a code, regenerate new ones in Settings → Users & Auth.',
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
      style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-md rounded-2xl p-6 space-y-5"
        style={{ background: 'var(--bg-card)', border: '1px solid var(--border)' }}>
        {/* Header */}
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Recovery Codes</h2>
            <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
              Save these codes now. Each can be used once to sign in if you lose your authenticator.
            </p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg shrink-0"
            style={{ color: 'var(--text-muted)' }}
            onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
            onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}>
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Warning banner */}
        <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg text-sm"
          style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: '#f87171' }}>
          <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
          These codes will not be shown again. Store them somewhere safe before closing this dialog.
        </div>

        {/* Codes grid */}
        <div className="grid grid-cols-2 gap-2">
          {codes.map((code) => (
            <code key={code} className="px-3 py-2 rounded-lg text-sm font-mono text-center"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)', border: '1px solid var(--border)' }}>
              {code}
            </code>
          ))}
        </div>

        {/* Actions */}
        <div className="flex gap-2">
          <button onClick={copyAll} className="btn-ghost flex-1 justify-center">
            {copied ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4" />}
            {copied ? 'Copied!' : 'Copy all'}
          </button>
          <button onClick={download} className="btn-ghost flex-1 justify-center">
            <Download className="w-4 h-4" /> Download .txt
          </button>
        </div>
        <button onClick={onClose} className="btn-primary w-full justify-center">
          <CheckCircle2 className="w-4 h-4" /> I've saved my recovery codes
        </button>
      </div>
    </div>
  );
}

function CreateUserModal({ servers, onClose, onCreated }: { servers: any[]; onClose: () => void; onCreated: () => void }) {
  const [form, setForm] = useState({ username: '', password: '', role: 'operator' });
  const [allowedServers, setAllowedServers] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const handleCreate = async () => {
    setLoading(true);
    try {
      await api.post('/api/v1/admin/users', {
        username: form.username,
        password: form.password,
        roles: [form.role],
        allowed_servers: allowedServers.length > 0 ? allowedServers : undefined,
      });
      toast.success('User created');
      onCreated();
      onClose();
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Create failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50 p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="card p-6 w-full max-w-sm space-y-4" style={{ background: 'var(--bg-elevated)' }}>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Create User</h2>
        {[
          { label: 'Username', key: 'username', type: 'text', placeholder: 'operator1' },
          { label: 'Password', key: 'password', type: 'password', placeholder: '••••••••' },
        ].map(({ label, key, type, placeholder }) => (
          <div key={key}>
            <label className="label">{label}</label>
            <input
              type={type}
              value={(form as any)[key]}
              onChange={e => setForm(p => ({ ...p, [key]: e.target.value }))}
              placeholder={placeholder}
              className="input"
            />
          </div>
        ))}
        <div>
          <label className="label">Role</label>
          <select
            value={form.role}
            onChange={e => setForm(p => ({ ...p, role: e.target.value }))}
            className="input"
          >
            <option value="admin">Admin — full access</option>
            <option value="operator">Operator — start/stop/manage servers</option>
            <option value="viewer">Viewer — read-only</option>
          </select>
        </div>
        {form.role !== 'admin' && (
          <ServerAccessPicker servers={servers} selected={allowedServers} onChange={setAllowedServers} />
        )}
        <div className="flex gap-3 pt-2">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={handleCreate}
            disabled={loading || !form.username || !form.password}
            className="btn-primary flex-1 justify-center"
          >
            {loading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            Create
          </button>
        </div>
      </div>
    </div>
  );
}

function EditUserModal({ user, servers, onClose, onSaved }: {
  user: any; servers: any[]; onClose: () => void; onSaved: () => void;
}) {
  const [role, setRole] = useState<string>(user.roles?.[0] ?? 'viewer');
  const [allowedServers, setAllowedServers] = useState<string[]>(user.allowed_servers ?? []);
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSave = async () => {
    setLoading(true);
    try {
      const body: any = { roles: [role], allowed_servers: allowedServers };
      if (password) body.password = password;
      await api.put(`/api/v1/admin/users/${user.id}`, body);
      toast.success('User updated');
      onSaved();
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Update failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50 p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="card p-6 w-full max-w-sm space-y-4" style={{ background: 'var(--bg-elevated)' }}>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
          Edit User — <span style={{ color: 'var(--primary)' }}>{user.username}</span>
        </h2>
        <div>
          <label className="label">Role</label>
          <select value={role} onChange={e => setRole(e.target.value)} className="input">
            <option value="admin">Admin — full access</option>
            <option value="operator">Operator — start/stop/manage servers</option>
            <option value="viewer">Viewer — read-only</option>
          </select>
        </div>
        {role !== 'admin' && (
          <ServerAccessPicker servers={servers} selected={allowedServers} onChange={setAllowedServers} />
        )}
        <div>
          <label className="label">New Password <span style={{ color: 'var(--text-muted)' }}>(leave blank to keep)</span></label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="••••••••"
            className="input"
          />
        </div>
        <div className="flex gap-3 pt-2">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={handleSave}
            disabled={loading}
            className="btn-primary flex-1 justify-center"
          >
            {loading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            Save
          </button>
        </div>
      </div>
    </div>
  );
}

// ── TLS section ──────────────────────────────────────────────────────────────

function TLSSection() {
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>TLS Certificates</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Transport Layer Security configuration</p>
      </div>
      <div className="card p-5 space-y-3">
        <h3 className="label">Certificate Paths</h3>
        {[
          { label: 'Certificate Path', value: '/etc/games-dashboard/tls/server.crt', mono: true },
          { label: 'Key Path',         value: '/etc/games-dashboard/tls/server.key', mono: true },
          { label: 'Auto TLS',         value: 'Disabled', mono: false },
        ].map(({ label, value, mono }) => (
          <div key={label} className="flex items-center justify-between rounded-lg px-3 py-2.5"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{label}</span>
            <span className={cn('text-sm', mono && 'font-mono text-xs')} style={{ color: 'var(--text-primary)' }}>{value}</span>
          </div>
        ))}
      </div>
      <div className="card p-4 flex items-start gap-3">
        <Key className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          To update TLS certificates, replace the cert and key files then run{' '}
          <code className="font-mono text-xs px-1 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
            games-daemon --tls-cert /path/cert --tls-key /path/key
          </code>.
        </p>
      </div>
    </div>
  );
}

// ── Storage section ──────────────────────────────────────────────────────────

function StorageSection() {
  const { data: settings, isLoading } = useSettings();
  const patch = usePatchSettings();

  const [backup, setBackup] = useState({
    default_schedule: '',
    retain_days: 30,
    compression: 'zstd',
  });
  const [backupDirty, setBackupDirty] = useState(false);

  React.useEffect(() => {
    if (settings && !backupDirty) {
      setBackup({
        default_schedule: settings.backup.default_schedule,
        retain_days: settings.backup.retain_days,
        compression: settings.backup.compression,
      });
    }
  }, [settings]);

  const handleBackupChange = (key: string, value: string | number) => {
    setBackup(prev => ({ ...prev, [key]: value }));
    setBackupDirty(true);
  };

  const handleSaveBackup = () => {
    patch.mutate({ backup }, {
      onSuccess: () => setBackupDirty(false),
    });
  };

  if (isLoading) return <SectionSkeleton />;

  const s = settings?.storage;

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Storage</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Data directories, mounts, and backup configuration</p>
      </div>

      {/* Data directory */}
      <div className="card p-5 space-y-3">
        <h3 className="label flex items-center gap-2">
          <Database className="w-3.5 h-3.5" /> Data Directory
        </h3>
        <div className="flex items-center justify-between rounded-lg px-3 py-2.5"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>Path</span>
          <code className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{s?.data_dir ?? '—'}</code>
        </div>
      </div>

      {/* NFS Mounts */}
      <div className="card p-5 space-y-3">
        <h3 className="label flex items-center gap-2">
          <Server className="w-3.5 h-3.5" /> NFS Mounts
        </h3>
        {(s?.nfs_mounts.length ?? 0) === 0 ? (
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No NFS mounts configured.</p>
        ) : (
          <div className="space-y-2">
            {s?.nfs_mounts.map((m, i) => (
              <div key={i} className="rounded-lg p-3 space-y-1.5"
                style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                {[
                  ['Server', m.server],
                  ['Remote path', m.path],
                  ['Mount point', m.mount_point],
                  ...(m.options ? [['Options', m.options]] : []),
                ].map(([label, value]) => (
                  <div key={label} className="flex items-center justify-between text-xs">
                    <span style={{ color: 'var(--text-muted)' }}>{label}</span>
                    <code className="font-mono" style={{ color: 'var(--text-primary)' }}>{value}</code>
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* S3 */}
      {s?.s3 && (
        <div className="card p-5 space-y-3">
          <h3 className="label">S3 / Object Store</h3>
          {[
            ['Endpoint', s.s3.endpoint],
            ['Bucket',   s.s3.bucket],
            ['Region',   s.s3.region],
            ['TLS',      s.s3.use_ssl ? 'Enabled' : 'Disabled'],
          ].map(([label, value]) => (
            <div key={label} className="flex items-center justify-between rounded-lg px-3 py-2.5"
              style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{label}</span>
              <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{value}</span>
            </div>
          ))}
        </div>
      )}

      {/* Backup settings — editable */}
      <div className="card p-5 space-y-4">
        <h3 className="label">Backup Settings</h3>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label">Schedule (cron)</label>
            <input
              className="input font-mono"
              value={backup.default_schedule}
              onChange={e => handleBackupChange('default_schedule', e.target.value)}
              placeholder="0 3 * * *"
            />
          </div>
          <div>
            <label className="label">Retain (days)</label>
            <input
              type="number"
              min={1}
              className="input"
              value={backup.retain_days}
              onChange={e => handleBackupChange('retain_days', parseInt(e.target.value) || 1)}
            />
          </div>
        </div>

        <div>
          <label className="label">Compression</label>
          <select
            className="input"
            value={backup.compression}
            onChange={e => handleBackupChange('compression', e.target.value)}
          >
            <option value="zstd">zstd (faster)</option>
            <option value="gzip">gzip (compatible)</option>
          </select>
        </div>

        <div className="flex items-center justify-between pt-1">
          {backupDirty && <span className="text-xs text-yellow-400">Unsaved changes</span>}
          <button
            onClick={handleSaveBackup}
            disabled={patch.isPending || !backupDirty}
            className="btn-primary ml-auto"
          >
            {patch.isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
            Save
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Networking section ───────────────────────────────────────────────────────

function NetworkingSection() {
  const { data: settings, isLoading } = useSettings();

  if (isLoading) return <SectionSkeleton />;

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Networking</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Bind address and connection settings</p>
      </div>

      <div className="card p-5 space-y-3">
        <h3 className="label">Connection Settings</h3>
        {[
          { label: 'Bind Address',     value: settings?.bind_addr ?? '—',                      mono: true  },
          { label: 'Shutdown Timeout', value: settings ? `${settings.shutdown_timeout_s}s` : '—', mono: false },
        ].map(({ label, value, mono }) => (
          <div key={label} className="flex items-center justify-between rounded-lg px-3 py-2.5"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{label}</span>
            <span className={cn('text-sm', mono && 'font-mono text-xs')} style={{ color: 'var(--text-primary)' }}>{value}</span>
          </div>
        ))}
      </div>

      <div className="card p-4 flex items-start gap-3">
        <Info className="w-4 h-4 text-blue-400 shrink-0 mt-0.5" />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Network settings require a daemon restart to take effect. Edit{' '}
          <code className="font-mono text-xs px-1 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
            daemon.yaml
          </code>{' '}
          to change these values.
        </p>
      </div>
    </div>
  );
}

// ── Monitoring section ───────────────────────────────────────────────────────

function MonitoringSection() {
  const { data: settings, isLoading } = useSettings();
  const patch = usePatchSettings();

  const [logLevel, setLogLevel] = useState('info');
  const [metricsEnabled, setMetricsEnabled] = useState(true);
  const [metricsPath, setMetricsPath] = useState('/metrics');
  const [clusterInterval, setClusterInterval] = useState(30);
  const [clusterTimeout, setClusterTimeout] = useState(90);
  const [dirty, setDirty] = useState(false);

  React.useEffect(() => {
    if (settings && !dirty) {
      setLogLevel(settings.log_level);
      setMetricsEnabled(settings.metrics.enabled);
      setMetricsPath(settings.metrics.path);
      setClusterInterval(settings.cluster.health_check_interval_s);
      setClusterTimeout(settings.cluster.node_timeout_s);
    }
  }, [settings]);

  const handleSave = () => {
    patch.mutate({
      log_level: logLevel,
      metrics: { enabled: metricsEnabled, path: metricsPath },
      cluster: {
        health_check_interval_s: clusterInterval,
        node_timeout_s: clusterTimeout,
      },
    }, {
      onSuccess: () => setDirty(false),
    });
  };

  const mark = () => setDirty(true);

  if (isLoading) return <SectionSkeleton />;

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Monitoring</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Logging, Prometheus metrics, and cluster health checks</p>
      </div>

      {/* Logging */}
      <div className="card p-5 space-y-4">
        <h3 className="label">Logging</h3>
        <div>
          <label className="label">Log Level</label>
          <select
            className="input"
            value={logLevel}
            onChange={e => { setLogLevel(e.target.value); mark(); }}
          >
            {['debug', 'info', 'warn', 'error'].map(l => (
              <option key={l} value={l}>{l}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Metrics */}
      <div className="card p-5 space-y-4">
        <h3 className="label flex items-center gap-2">
          <Activity className="w-3.5 h-3.5" /> Prometheus Metrics
        </h3>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => { setMetricsEnabled(v => !v); mark(); }}
            className={cn(
              'relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors',
              metricsEnabled ? 'bg-orange-500' : 'bg-gray-700'
            )}
          >
            <span className={cn(
              'pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow ring-0 transition-transform',
              metricsEnabled ? 'translate-x-4' : 'translate-x-0'
            )} />
          </button>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {metricsEnabled ? 'Enabled' : 'Disabled'}
          </span>
        </div>
        <div>
          <label className="label">Metrics Path</label>
          <input
            className="input font-mono"
            value={metricsPath}
            onChange={e => { setMetricsPath(e.target.value); mark(); }}
            placeholder="/metrics"
          />
        </div>
      </div>

      {/* Cluster health check */}
      {settings?.cluster.enabled && (
        <div className="card p-5 space-y-4">
          <h3 className="label flex items-center gap-2">
            <RefreshCw className="w-3.5 h-3.5" /> Cluster Health Checks
          </h3>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="label">Check Interval (s)</label>
              <input
                type="number"
                min={5}
                className="input"
                value={clusterInterval}
                onChange={e => { setClusterInterval(parseInt(e.target.value) || 30); mark(); }}
              />
            </div>
            <div>
              <label className="label">Node Timeout (s)</label>
              <input
                type="number"
                min={10}
                className="input"
                value={clusterTimeout}
                onChange={e => { setClusterTimeout(parseInt(e.target.value) || 90); mark(); }}
              />
            </div>
          </div>
        </div>
      )}

      <div className="card p-4 flex items-start gap-3">
        <Info className="w-4 h-4 text-blue-400 shrink-0 mt-0.5" />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Changes are saved to{' '}
          <code className="font-mono text-xs px-1 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
            daemon.yaml
          </code>{' '}
          and take effect on the next daemon restart.
        </p>
      </div>

      <div className="flex items-center justify-end gap-3">
        {dirty && <span className="text-xs text-yellow-400">Unsaved changes</span>}
        <button
          onClick={handleSave}
          disabled={patch.isPending || !dirty}
          className="btn-primary"
        >
          {patch.isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
          Save
        </button>
      </div>
    </div>
  );
}

// ── Updates section ──────────────────────────────────────────────────────────

interface UpdateStatus {
  current_branch: string;
  target_branch: string;
  current_commit: string;
  commits_behind: number;
  update_available: boolean;
  latest_message: string;
  error?: string;
}

function progressStage(p: number): string {
  if (p < 5)  return 'Starting update…';
  if (p < 15) return 'Checking environment (Go, Node)…';
  if (p < 30) return 'Pulling latest code from GitHub…';
  if (p < 60) return 'Building daemon binary…';
  if (p < 70) return 'Building CLI binary…';
  if (p < 90) return 'Building UI (npm + vite build)…';
  if (p < 95) return 'Wrapping up…';
  if (p < 100) return 'Restarting service — page will reload shortly…';
  return 'Complete!';
}

function UpdatesSection() {
  const qc = useQueryClient();
  const [branch, setBranch] = useState<'main' | 'dev'>('main');
  const [applyMsg, setApplyMsg] = useState('');
  const [showLog, setShowLog] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [progress, setProgress] = useState(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Pass selected branch so the status check compares HEAD vs origin/<branch>
  const { data: status, isLoading, isFetching, refetch } = useQuery<UpdateStatus>({
    queryKey: ['update-status', branch],
    queryFn: () => api.get(`/api/v1/admin/update/status?branch=${branch}`).then(r => r.data),
    staleTime: 30_000,
  });

  const { data: logData, refetch: refetchLog } = useQuery<{ lines: string[]; note?: string }>({
    queryKey: ['update-log'],
    queryFn: () => api.get('/api/v1/admin/update/log').then(r => r.data),
    enabled: showLog || updating,
    staleTime: 0,
  });

  // Parse PROGRESS:N markers from the log while update is running
  useEffect(() => {
    if (!updating) return;
    const lines = logData?.lines ?? [];
    let latest = progress;
    for (const line of lines) {
      const m = line.match(/^PROGRESS:(\d+)$/);
      if (m) latest = Math.max(latest, parseInt(m[1], 10));
    }
    if (latest !== progress) setProgress(latest);
    // Stop polling when script signals completion
    if (latest >= 100 || lines.some(l => l.includes('=== Update complete ==='))) {
      setUpdating(false);
      setProgress(100);
      if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; }
    }
  }, [logData]);

  // Auto-reload the page ~35 s after restart signal (progress ≥ 95)
  useEffect(() => {
    if (progress >= 95 && progress < 100) {
      const t = setTimeout(() => window.location.reload(), 35_000);
      return () => clearTimeout(t);
    }
  }, [progress >= 95]);

  // Clean up polling interval on unmount
  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  const startUpdatePolling = () => {
    setUpdating(true);
    setProgress(0);
    setShowLog(true);
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(() => refetchLog(), 2_000);
  };

  const apply = useMutation({
    mutationFn: () => api.post('/api/v1/admin/update/apply', { branch }).then(r => r.data),
    onSuccess: (data: any) => {
      setApplyMsg(data?.msg ?? 'Update started.');
      toast.success('Update launched — dashboard restarting in ~60 s…');
      qc.invalidateQueries({ queryKey: ['update-status'] });
      startUpdatePolling();
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Update failed'),
  });

  // Sync branch picker to current repo branch on first load
  useEffect(() => {
    if (status?.current_branch === 'dev') setBranch('dev');
  }, [status?.current_branch]);

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Updates</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Pull the latest code, rebuild, and restart automatically.
        </p>
      </div>

      {/* Current version card */}
      <div className="card p-5 space-y-4">
        <h3 className="label flex items-center gap-2"><GitBranch className="w-3.5 h-3.5" /> Current Version</h3>
        {isLoading ? (
          <div className="animate-pulse space-y-2">
            <div className="h-4 w-48 rounded" style={{ background: 'var(--bg-elevated)' }} />
            <div className="h-4 w-32 rounded" style={{ background: 'var(--bg-elevated)' }} />
          </div>
        ) : status?.error ? (
          <div className="flex items-start gap-2 text-yellow-400 text-sm">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <span>{status.error}</span>
          </div>
        ) : (
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span style={{ color: 'var(--text-secondary)' }}>Branch</span>
              <code className="font-mono text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
                {status?.current_branch}
              </code>
            </div>
            <div className="flex justify-between">
              <span style={{ color: 'var(--text-secondary)' }}>Commit</span>
              <code className="font-mono text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}>
                {status?.current_commit}
              </code>
            </div>
            <div className="flex justify-between items-center">
              <span style={{ color: 'var(--text-secondary)' }}>Status</span>
              {status?.update_available ? (
                <span className="text-yellow-400 flex items-center gap-1">
                  <AlertCircle className="w-3.5 h-3.5" />
                  {status.commits_behind} commit{status.commits_behind !== 1 ? 's' : ''} behind
                </span>
              ) : (
                <span className="text-green-400 flex items-center gap-1">
                  <CheckCircle2 className="w-3.5 h-3.5" /> Up to date
                </span>
              )}
            </div>
            {status?.update_available && status?.latest_message && (
              <div className="pt-1 flex justify-between">
                <span style={{ color: 'var(--text-secondary)' }}>Latest</span>
                <span className="text-xs max-w-[60%] text-right" style={{ color: 'var(--text-primary)' }}>
                  {status.latest_message}
                </span>
              </div>
            )}
          </div>
        )}

        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="btn-secondary flex items-center gap-2 text-sm"
        >
          <RefreshCw className={cn('w-3.5 h-3.5', isFetching && 'animate-spin')} />
          Check for Updates
        </button>
      </div>

      {/* Branch selector + apply */}
      <div className="card p-5 space-y-4">
        <h3 className="label flex items-center gap-2"><Download className="w-3.5 h-3.5" /> Apply Update</h3>

        <div className="space-y-2">
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Target branch</p>
          <div className="flex gap-3">
            {(['main', 'dev'] as const).map(b => (
              <label key={b} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="branch"
                  value={b}
                  checked={branch === b}
                  onChange={() => setBranch(b)}
                  className="accent-orange-500"
                />
                <span className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{b}</span>
                {b === 'main' && (
                  <span className="text-xs px-1.5 py-0.5 rounded-full bg-green-500/20 text-green-400">stable</span>
                )}
                {b === 'dev' && (
                  <span className="text-xs px-1.5 py-0.5 rounded-full bg-yellow-500/20 text-yellow-400">pre-release</span>
                )}
              </label>
            ))}
          </div>
        </div>

        {applyMsg ? (
          <div className="flex items-start gap-2 text-green-400 text-sm">
            <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
            <span>{applyMsg}</span>
          </div>
        ) : (
          <div className="flex items-start gap-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
            <Info className="w-4 h-4 shrink-0 mt-0.5 text-blue-400" />
            <span>
              The update will pull the latest code, rebuild the daemon and UI, then restart the service.
              Expect ~60 seconds of downtime.
            </span>
          </div>
        )}

        <button
          onClick={() => apply.mutate()}
          disabled={apply.isPending || updating || !!applyMsg}
          className="btn-primary flex items-center gap-2"
        >
          {apply.isPending
            ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
            : <Download className="w-3.5 h-3.5" />}
          {apply.isPending ? 'Launching update…' : `Update to ${branch}`}
        </button>
      </div>

      {/* Progress bar — shown while update script is running */}
      {(updating || (progress > 0 && progress < 100)) && (
        <div className="card p-5 space-y-3">
          <h3 className="label flex items-center gap-2">
            <Loader2 className="w-3.5 h-3.5 animate-spin" /> Update in Progress
          </h3>
          <div>
            <div className="flex justify-between text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              <span>{progressStage(progress)}</span>
              <span className="tabular-nums font-mono" style={{ color: 'var(--text-muted)' }}>{progress}%</span>
            </div>
            {/* Track */}
            <div className="h-2.5 rounded-full overflow-hidden" style={{ background: 'var(--bg-elevated)' }}>
              {/* Fill */}
              <div
                className="h-full rounded-full transition-all duration-700 ease-out"
                style={{
                  width: `${progress}%`,
                  background: progress >= 95
                    ? 'linear-gradient(90deg,#22c55e,#4ade80)'
                    : 'linear-gradient(90deg,#f97316,#fb923c)',
                }}
              />
            </div>
            {/* Stage dots */}
            <div className="flex justify-between mt-1.5 px-0.5">
              {[10, 30, 60, 70, 90, 95].map(tick => (
                <div
                  key={tick}
                  className="w-1.5 h-1.5 rounded-full transition-colors duration-300"
                  style={{ background: progress >= tick ? '#f97316' : 'var(--bg-elevated)' }}
                />
              ))}
            </div>
          </div>
          {progress >= 95 && (
            <p className="text-xs flex items-center gap-1.5 text-green-400">
              <RefreshCw className="w-3 h-3 animate-spin" />
              Service restarting — page reloads automatically in ~35 s
            </p>
          )}
        </div>
      )}
      {progress === 100 && !updating && (
        <div className="card p-4 flex items-center gap-2 text-green-400 text-sm">
          <CheckCircle2 className="w-4 h-4" /> Update complete! Dashboard has restarted.
        </div>
      )}

      {/* Update log viewer */}
      <div className="card overflow-hidden">
        <button
          className="w-full flex items-center justify-between px-5 py-3.5 text-sm"
          style={{ borderBottom: showLog ? '1px solid var(--border)' : undefined }}
          onClick={() => {
            setShowLog(v => !v);
            if (!showLog) refetchLog();
          }}
        >
          <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>Update Log</span>
          <span style={{ color: 'var(--text-muted)' }}>{showLog ? '▲ Hide' : '▼ Show'}</span>
        </button>
        {showLog && (
          <div className="p-4">
            {logData?.note ? (
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>{logData.note}</p>
            ) : (
              <pre
                className="text-xs font-mono whitespace-pre-wrap overflow-auto max-h-80 leading-relaxed"
                style={{ color: 'var(--text-secondary)' }}
              >
                {(logData?.lines ?? []).join('\n') || '(empty)'}
              </pre>
            )}
            <button
              onClick={() => refetchLog()}
              className="mt-3 text-xs px-3 py-1.5 rounded border"
              style={{ borderColor: 'var(--border)', color: 'var(--text-muted)' }}
            >
              Refresh
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Notifications section ─────────────────────────────────────────────────────

function NotificationsSection() {
  const [url, setUrl] = useState('');
  const [format, setFormat] = useState<'discord' | 'slack' | 'generic'>('discord');
  const [events, setEvents] = useState<string[]>([
    'server.crash', 'server.restart', 'disk.warning', 'backup.failed',
  ]);
  // Email state
  const [emailEnabled, setEmailEnabled] = useState(false);
  const [smtpHost, setSmtpHost] = useState('');
  const [smtpPort, setSmtpPort] = useState('587');
  const [smtpUser, setSmtpUser] = useState('');
  const [smtpPass, setSmtpPass] = useState('');
  const [emailFrom, setEmailFrom] = useState('');
  const [emailTo, setEmailTo] = useState('');
  const [useTLS, setUseTLS] = useState(false);
  const [showSmtpPass, setShowSmtpPass] = useState(false);

  const [loaded, setLoaded] = useState(false);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.get('/api/v1/admin/notifications').then(r => {
      setUrl(r.data.webhook_url ?? '');
      setFormat(r.data.webhook_format ?? 'discord');
      setEvents(r.data.events ?? []);
      const em = r.data.email;
      if (em) {
        setEmailEnabled(em.enabled ?? false);
        setSmtpHost(em.smtp_host ?? '');
        setSmtpPort(String(em.smtp_port ?? 587));
        setSmtpUser(em.username ?? '');
        setSmtpPass(em.password ?? '');
        setEmailFrom(em.from ?? '');
        setEmailTo((em.to ?? []).join(', '));
        setUseTLS(em.use_tls ?? false);
      }
      setLoaded(true);
    });
  }, []);

  const ALL_EVENTS = [
    { id: 'server.crash',    label: 'Server crashed' },
    { id: 'server.restart',  label: 'Server auto-restarted' },
    { id: 'disk.warning',    label: 'Disk space warning (>80%)' },
    { id: 'backup.complete', label: 'Backup completed' },
    { id: 'backup.failed',   label: 'Backup failed' },
  ];

  const toggleEvent = (id: string) => {
    setEvents(prev =>
      prev.includes(id) ? prev.filter(e => e !== id) : [...prev, id]
    );
  };

  const save = async () => {
    setSaving(true);
    try {
      await api.patch('/api/v1/admin/notifications', {
        webhook_url: url,
        webhook_format: format,
        events,
        email: {
          enabled: emailEnabled,
          smtp_host: smtpHost,
          smtp_port: parseInt(smtpPort, 10) || 587,
          username: smtpUser,
          password: smtpPass,
          from: emailFrom,
          to: emailTo.split(',').map(s => s.trim()).filter(Boolean),
          use_tls: useTLS,
        },
      });
      toast.success('Notification settings saved');
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    try {
      await api.post('/api/v1/admin/notifications/test', {});
      toast.success('Test notification sent!');
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Test failed');
    } finally {
      setTesting(false);
    }
  };

  if (!loaded) return <SectionSkeleton />;

  return (
    <div className="space-y-6 max-w-xl">
      <div>
        <h2 className="text-lg font-semibold mb-1" style={{ color: 'var(--text-primary)' }}>
          Notifications
        </h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Send alerts via Discord/Slack webhook and/or email when important events occur.
        </p>
      </div>

      {/* Webhook */}
      <div className="card p-5 space-y-4">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Webhook</h3>
        <div>
          <label className="block text-xs font-semibold uppercase tracking-wide mb-1.5"
            style={{ color: 'var(--text-muted)' }}>
            Webhook URL
          </label>
          <input
            className="input w-full font-mono text-sm"
            value={url}
            onChange={e => setUrl(e.target.value)}
            placeholder="https://discord.com/api/webhooks/…"
          />
          <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
            Discord, Slack Incoming Webhook, or any HTTP endpoint that accepts POST JSON.
          </p>
        </div>
        <div>
          <label className="block text-xs font-semibold uppercase tracking-wide mb-1.5"
            style={{ color: 'var(--text-muted)' }}>
            Format
          </label>
          <select className="input w-full" value={format} onChange={e => setFormat(e.target.value as any)}>
            <option value="discord">Discord (rich embed)</option>
            <option value="slack">Slack (text message)</option>
            <option value="generic">Generic JSON (event + server + message)</option>
          </select>
        </div>
      </div>

      {/* Email */}
      <div className="card p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Email (SMTP)</h3>
          <button
            type="button"
            onClick={() => setEmailEnabled(v => !v)}
            className="relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors"
            style={{ background: emailEnabled ? 'var(--primary)' : 'rgba(148,163,184,0.3)' }}
            aria-pressed={emailEnabled}
          >
            <span className="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow transition"
              style={{ transform: emailEnabled ? 'translateX(16px)' : 'translateX(0)' }} />
          </button>
        </div>

        {emailEnabled && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="label">SMTP Host</label>
                <input className="input" value={smtpHost} onChange={e => setSmtpHost(e.target.value)}
                  placeholder="smtp.gmail.com" />
              </div>
              <div>
                <label className="label">Port</label>
                <input className="input" value={smtpPort} onChange={e => setSmtpPort(e.target.value)}
                  placeholder="587" type="number" min={1} max={65535} />
              </div>
            </div>
            <div>
              <label className="label">Username</label>
              <input className="input" value={smtpUser} onChange={e => setSmtpUser(e.target.value)}
                placeholder="you@example.com" autoComplete="off" />
            </div>
            <div>
              <label className="label">Password / App password</label>
              <div className="relative">
                <input className="input pr-10" type={showSmtpPass ? 'text' : 'password'}
                  value={smtpPass} onChange={e => setSmtpPass(e.target.value)}
                  placeholder="••••••••" autoComplete="new-password" />
                <button type="button" tabIndex={-1}
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-1"
                  onClick={() => setShowSmtpPass(v => !v)}
                  style={{ color: 'var(--text-muted)' }}>
                  {showSmtpPass ? '🙈' : '👁'}
                </button>
              </div>
            </div>
            <div>
              <label className="label">From address</label>
              <input className="input" value={emailFrom} onChange={e => setEmailFrom(e.target.value)}
                placeholder="alerts@example.com" />
            </div>
            <div>
              <label className="label">To addresses <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(comma-separated)</span></label>
              <input className="input" value={emailTo} onChange={e => setEmailTo(e.target.value)}
                placeholder="admin@example.com, oncall@example.com" />
            </div>
            <label className="flex items-center gap-2 cursor-pointer select-none">
              <input type="checkbox" checked={useTLS} onChange={e => setUseTLS(e.target.checked)}
                className="w-4 h-4 accent-orange-500" />
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Use implicit TLS (port 465) — uncheck for STARTTLS (port 587)
              </span>
            </label>
          </div>
        )}
      </div>

      {/* Events */}
      <div className="card p-5 space-y-3">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
          Events to notify
        </h3>
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Applies to both webhook and email.
        </p>
        <div className="space-y-2">
          {ALL_EVENTS.map(ev => (
            <label key={ev.id}
              className="flex items-center gap-3 cursor-pointer py-1.5 rounded-lg px-3 transition-colors"
              style={{ background: events.includes(ev.id) ? 'var(--primary-subtle)' : 'transparent' }}
            >
              <input type="checkbox" checked={events.includes(ev.id)} onChange={() => toggleEvent(ev.id)}
                className="w-4 h-4 accent-orange-500" />
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{ev.label}</span>
            </label>
          ))}
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-3">
        <button onClick={save} disabled={saving} className="btn-primary py-2 px-5">
          <Save className="w-4 h-4" />
          {saving ? 'Saving…' : 'Save'}
        </button>
        <button onClick={test} disabled={testing || (!url && !emailEnabled)}
          className="btn-ghost py-2 px-5 disabled:opacity-40">
          {testing ? 'Sending…' : 'Send Test'}
        </button>
      </div>

      <PushNotificationsCard />
    </div>
  );
}

// ── Push Notifications card ──────────────────────────────────────────────────

function PushNotificationsCard() {
  const [permission, setPermission] = useState<NotificationPermission>(
    typeof Notification !== 'undefined' ? Notification.permission : 'denied'
  );
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [endpoint, setEndpoint] = useState('');

  const supported =
    typeof window !== 'undefined' &&
    'Notification' in window &&
    'serviceWorker' in navigator &&
    'PushManager' in window;

  useEffect(() => {
    if (!supported) return;
    navigator.serviceWorker.ready.then((reg) => {
      reg.pushManager.getSubscription().then((sub) => {
        if (sub) { setSubscribed(true); setEndpoint(sub.endpoint); }
      });
    });
  }, [supported]);

  async function enable() {
    setLoading(true); setError('');
    try {
      const perm = await Notification.requestPermission();
      setPermission(perm);
      if (perm !== 'granted') { setError('Notification permission was denied.'); return; }
      const { data } = await api.get<{ public_key: string }>('/api/v1/push/vapid-key');
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(data.public_key),
      });
      await api.post('/api/v1/push/subscribe', sub.toJSON());
      setSubscribed(true); setEndpoint(sub.endpoint);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to enable push notifications.');
    } finally { setLoading(false); }
  }

  async function disable() {
    setLoading(true); setError('');
    try {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        await sub.unsubscribe();
        await api.delete('/api/v1/push/subscribe', { data: { endpoint: sub.endpoint } });
      }
      setSubscribed(false); setEndpoint('');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to disable push notifications.');
    } finally { setLoading(false); }
  }

  if (!supported) return (
    <div className="card p-5 mt-6">
      <p className="font-medium mb-1">Push Notifications</p>
      <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
        Not supported in this browser. Install the dashboard as a PWA on a modern browser to enable push alerts.
      </p>
    </div>
  );

  return (
    <div className="card p-5 mt-6 space-y-4">
      <div>
        <p className="font-medium">Push Notifications</p>
        <p className="text-sm mt-0.5" style={{ color: 'var(--text-secondary)' }}>
          Receive crash and restart alerts on this device even when the browser tab is closed.
        </p>
      </div>
      <div className="flex items-center gap-3">
        <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>Status:</span>
        {subscribed
          ? <span className="badge badge-green text-xs">Active</span>
          : permission === 'denied'
            ? <span className="badge badge-red text-xs">Blocked by browser</span>
            : <span className="badge badge-gray text-xs">Disabled</span>}
      </div>
      {subscribed && endpoint && (
        <p className="text-xs truncate" style={{ color: 'var(--text-tertiary)', maxWidth: 400 }}>{endpoint}</p>
      )}
      {error && <p className="text-sm" style={{ color: 'var(--error)' }}>{error}</p>}
      {permission === 'denied' ? (
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Push is blocked. Open your browser's site settings, allow notifications, then reload.
        </p>
      ) : subscribed ? (
        <button onClick={disable} disabled={loading}
          className="btn-ghost py-2 px-5 disabled:opacity-40 text-sm"
          style={{ color: 'var(--error)' }}>
          {loading ? 'Disabling…' : 'Disable push notifications'}
        </button>
      ) : (
        <button onClick={enable} disabled={loading}
          className="btn-primary py-2 px-5 disabled:opacity-40 text-sm">
          {loading ? 'Enabling…' : 'Enable push notifications'}
        </button>
      )}
    </div>
  );
}

function urlBase64ToUint8Array(base64String: string): ArrayBuffer {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = window.atob(base64);
  const arr = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i++) arr[i] = rawData.charCodeAt(i);
  return arr.buffer as ArrayBuffer;
}

// ── Skeleton ─────────────────────────────────────────────────────────────────

function SectionSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="h-7 w-32 rounded-lg" style={{ background: 'var(--bg-elevated)' }} />
      <div className="card p-5 space-y-3">
        {[1, 2, 3].map(i => (
          <div key={i} className="flex justify-between">
            <div className="h-4 w-28 rounded" style={{ background: 'var(--bg-elevated)' }} />
            <div className="h-4 w-36 rounded" style={{ background: 'var(--bg-elevated)' }} />
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function SettingsPage() {
  const [section, setSection] = useState<Section>('general');

  return (
    <div className="flex h-full min-h-0">
      {/* Left nav */}
      <aside className="w-48 shrink-0 p-4 space-y-1" style={{ borderRight: '1px solid var(--border)' }}>
        <p className="label px-3 mb-3">Settings</p>
        {NAV.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            onClick={() => setSection(id)}
            className={cn(
              'relative w-full flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-sm transition-all text-left',
              section === id
                ? 'nav-active font-medium'
                : 'hover:opacity-100'
            )}
            style={section === id
              ? { background: 'var(--primary-subtle)', color: 'var(--primary)' }
              : { color: 'var(--text-secondary)' }
            }
          >
            <Icon className="w-4 h-4 shrink-0" />
            {label}
          </button>
        ))}
      </aside>

      {/* Content */}
      <main className="flex-1 p-6 md:p-8 overflow-auto">
        {section === 'general'       && <GeneralSection />}
        {section === 'users'         && <UsersSection />}
        {section === 'tls'           && <TLSSection />}
        {section === 'storage'       && <StorageSection />}
        {section === 'networking'    && <NetworkingSection />}
        {section === 'monitoring'    && <MonitoringSection />}
        {section === 'updates'       && <UpdatesSection />}
        {section === 'notifications' && <NotificationsSection />}
      </main>
    </div>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}
