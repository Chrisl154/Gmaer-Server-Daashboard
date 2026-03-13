import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Settings, Shield, Server, HardDrive, Network, Activity,
  Plus, Trash2, User, QrCode, Info, Loader2, Key, Save,
  Database, RefreshCw, Download, GitBranch, CheckCircle2, AlertCircle,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { cn } from '../utils/cn';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { useSystemStatus } from '../hooks/useServers';
import type { DaemonSettings, SettingsPatch } from '../types';

type Section = 'general' | 'users' | 'tls' | 'storage' | 'networking' | 'monitoring' | 'updates';

const NAV: { id: Section; icon: React.FC<any>; label: string }[] = [
  { id: 'general',    icon: Settings,  label: 'General'      },
  { id: 'users',      icon: User,      label: 'Users & Auth' },
  { id: 'tls',        icon: Shield,    label: 'TLS'          },
  { id: 'storage',    icon: HardDrive, label: 'Storage'      },
  { id: 'networking', icon: Network,   label: 'Networking'   },
  { id: 'monitoring', icon: Activity,  label: 'Monitoring'   },
  { id: 'updates',    icon: Download,  label: 'Updates'      },
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
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [totpLoading, setTotpLoading] = useState(false);

  const { data } = useQuery<{ users: any[] }>({
    queryKey: ['users'],
    queryFn: () => api.get('/api/v1/admin/users').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/admin/users/${id}`),
    onSuccess: () => { toast.success('User deleted'); qc.invalidateQueries({ queryKey: ['users'] }); },
    onError: () => toast.error('Delete failed'),
  });

  const users = data?.users ?? [];

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
      await verifyTOTP(totpCode);
      toast.success('TOTP enabled successfully');
      setTotpSetup(null);
      setTotpCode('');
    } catch {
      toast.error('Invalid TOTP code');
    }
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
                    <div className="flex items-center gap-2 mt-0.5">
                      {u.roles?.map((r: string) => (
                        <span key={r} className="badge"
                          style={{
                            background: r === 'admin' ? 'rgba(59,130,246,0.15)' : 'rgba(128,128,168,0.15)',
                            color: r === 'admin' ? '#60a5fa' : 'var(--text-secondary)',
                          }}>
                          {r}
                        </span>
                      ))}
                      {u.totp_enabled && (
                        <span className="text-xs text-green-400 flex items-center gap-1">
                          <Shield className="w-3 h-3" /> MFA
                        </span>
                      )}
                    </div>
                  </div>
                </div>
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
            ))
          )}
        </div>
      </div>

      {/* TOTP Setup */}
      <div className="card p-5 space-y-4">
        <h3 className="label">Two-Factor Authentication</h3>
        {currentUser?.totp_enabled ? (
          <div className="flex items-center gap-2 text-sm text-green-400">
            <Shield className="w-4 h-4" /> TOTP is enabled for your account
          </div>
        ) : totpSetup ? (
          <div className="space-y-4">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Scan this QR code with your authenticator app, then enter the 6-digit code:
            </p>
            <div className="rounded-lg p-3 font-mono text-xs break-all"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-primary)', border: '1px solid var(--border)' }}>
              {totpSetup.qr_code_url}
            </div>
            <div className="flex items-center gap-3">
              <input
                type="text"
                value={totpCode}
                onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                maxLength={6}
                className="input w-32 font-mono text-center tracking-widest"
              />
              <button
                onClick={handleVerifyTOTP}
                disabled={totpCode.length !== 6}
                className="btn-blue"
              >
                Verify & Enable
              </button>
              <button
                onClick={() => { setTotpSetup(null); setTotpCode(''); }}
                className="btn-ghost"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button
            onClick={handleSetupTOTP}
            disabled={totpLoading}
            className="btn-ghost"
          >
            {totpLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <QrCode className="w-4 h-4" />}
            Set up TOTP
          </button>
        )}
      </div>

      {showCreate && (
        <CreateUserModal onClose={() => setShowCreate(false)} onCreated={() => qc.invalidateQueries({ queryKey: ['users'] })} />
      )}
    </div>
  );
}

function CreateUserModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [form, setForm] = useState({ username: '', password: '', roles: 'operator' });
  const [loading, setLoading] = useState(false);

  const handleCreate = async () => {
    setLoading(true);
    try {
      await api.post('/api/v1/admin/users', {
        username: form.username,
        password: form.password,
        roles: [form.roles],
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
            value={form.roles}
            onChange={e => setForm(p => ({ ...p, roles: e.target.value }))}
            className="input"
          >
            <option value="admin">Admin</option>
            <option value="operator">Operator</option>
            <option value="viewer">Viewer</option>
          </select>
        </div>
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

function UpdatesSection() {
  const qc = useQueryClient();
  const [branch, setBranch] = useState<'main' | 'dev'>('main');
  const [applyMsg, setApplyMsg] = useState('');
  const [showLog, setShowLog] = useState(false);

  // Pass selected branch so the status check compares HEAD vs origin/<branch>
  const { data: status, isLoading, isFetching, refetch } = useQuery<UpdateStatus>({
    queryKey: ['update-status', branch],
    queryFn: () => api.get(`/api/v1/admin/update/status?branch=${branch}`).then(r => r.data),
    staleTime: 30_000,
  });

  const { data: logData, refetch: refetchLog } = useQuery<{ lines: string[]; note?: string }>({
    queryKey: ['update-log'],
    queryFn: () => api.get('/api/v1/admin/update/log').then(r => r.data),
    enabled: showLog,
    staleTime: 5_000,
  });

  const apply = useMutation({
    mutationFn: () => api.post('/api/v1/admin/update/apply', { branch }).then(r => r.data),
    onSuccess: (data: any) => {
      setApplyMsg(data?.msg ?? 'Update started.');
      toast.success('Update launched — dashboard restarting…');
      qc.invalidateQueries({ queryKey: ['update-status'] });
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Update failed'),
  });

  // Sync branch picker to current repo branch on first load
  React.useEffect(() => {
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
          disabled={apply.isPending || !!applyMsg}
          className="btn-primary flex items-center gap-2"
        >
          {apply.isPending
            ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
            : <Download className="w-3.5 h-3.5" />}
          {apply.isPending ? 'Launching update…' : `Update to ${branch}`}
        </button>
      </div>

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
        {section === 'general'    && <GeneralSection />}
        {section === 'users'      && <UsersSection />}
        {section === 'tls'        && <TLSSection />}
        {section === 'storage'    && <StorageSection />}
        {section === 'networking' && <NetworkingSection />}
        {section === 'monitoring' && <MonitoringSection />}
        {section === 'updates'    && <UpdatesSection />}
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
