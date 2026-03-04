import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Settings, Shield, Server, HardDrive, Network, Activity,
  Plus, Trash2, User, QrCode, Info, Loader2, Key
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { clsx } from 'clsx';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { useSystemStatus } from '../hooks/useServers';

type Section = 'general' | 'users' | 'tls' | 'storage' | 'networking' | 'monitoring';

const NAV: { id: Section; icon: React.FC<any>; label: string }[] = [
  { id: 'general',    icon: Settings,  label: 'General'     },
  { id: 'users',      icon: User,      label: 'Users & Auth'},
  { id: 'tls',        icon: Shield,    label: 'TLS'         },
  { id: 'storage',    icon: HardDrive, label: 'Storage'     },
  { id: 'networking', icon: Network,   label: 'Networking'  },
  { id: 'monitoring', icon: Activity,  label: 'Monitoring'  },
];

// ── General section ───────────────────────────────────────────────────────────

function GeneralSection() {
  const { data: status } = useSystemStatus();

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold text-gray-200 uppercase tracking-wider">General</h2>

      {status && (
        <div className="bg-[#141414] border border-[#252525] rounded-xl p-4 space-y-3">
          <h3 className="text-xs font-medium text-gray-400 uppercase tracking-wider">System Status</h3>
          <div className="grid grid-cols-2 gap-x-6 gap-y-2">
            {[
              ['Version', status.version],
              ['Uptime', formatUptime(status.uptime_seconds)],
              ['Status', status.healthy ? 'Healthy' : 'Degraded'],
              ['Start Time', new Date(status.start_time).toLocaleString()],
            ].map(([label, value]) => (
              <div key={label} className="flex items-center justify-between text-sm">
                <span className="text-gray-500">{label}</span>
                <span className={clsx('text-gray-200', label === 'Status' && (status.healthy ? 'text-green-400' : 'text-red-400'))}>
                  {value}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="flex items-start gap-3 p-3 bg-[#141414] border border-[#252525] rounded-xl">
        <Info className="w-4 h-4 text-blue-400 shrink-0 mt-0.5" />
        <p className="text-xs text-gray-400">
          Advanced configuration can be edited in{' '}
          <code className="font-mono text-gray-300">/etc/games-dashboard/daemon.yaml</code>.
          Restart the daemon after making changes.
        </p>
      </div>
    </div>
  );
}

// ── Users section ─────────────────────────────────────────────────────────────

function UsersSection() {
  const qc = useQueryClient();
  const { user: currentUser, setupTOTP, verifyTOTP } = useAuthStore();
  const [showCreate, setShowCreate] = useState(false);
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [totpLoading, setTotpLoading] = useState(false);

  const { data } = useQuery<{ users: any[] }>({
    queryKey: ['users'],
    queryFn: () => api.get('/api/v1/auth/users').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/auth/users/${id}`),
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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-200 uppercase tracking-wider">Users</h2>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg"
        >
          <Plus className="w-3 h-3" /> Add User
        </button>
      </div>

      <div className="bg-[#141414] border border-[#252525] rounded-xl divide-y divide-[#1e1e1e]">
        {users.length === 0 ? (
          <div className="px-4 py-3 text-sm text-gray-500">No users found.</div>
        ) : (
          users.map((u: any) => (
            <div key={u.id} className="flex items-center justify-between px-4 py-3">
              <div className="flex items-center gap-3">
                <div className="w-7 h-7 rounded-full bg-blue-600/20 flex items-center justify-center text-xs text-blue-400 font-medium">
                  {u.username?.[0]?.toUpperCase()}
                </div>
                <div>
                  <div className="text-sm text-gray-200">{u.username}</div>
                  <div className="text-xs text-gray-500">
                    {u.roles?.join(', ')}
                    {u.totp_enabled && <span className="ml-2 text-green-400">· MFA</span>}
                  </div>
                </div>
              </div>
              {u.id !== currentUser?.id && (
                <button
                  onClick={() => deleteMutation.mutate(u.id)}
                  disabled={deleteMutation.isPending}
                  className="p-1.5 text-gray-500 hover:text-red-400 hover:bg-red-900/10 rounded-lg transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              )}
            </div>
          ))
        )}
      </div>

      {/* TOTP Setup */}
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4 space-y-3">
        <h3 className="text-xs font-medium text-gray-400 uppercase tracking-wider">Two-Factor Authentication</h3>
        {currentUser?.totp_enabled ? (
          <div className="flex items-center gap-2 text-sm text-green-400">
            <Shield className="w-4 h-4" /> TOTP is enabled for your account
          </div>
        ) : totpSetup ? (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Scan this QR code with your authenticator app, then enter the 6-digit code:
            </p>
            <code className="block text-xs text-gray-300 bg-[#0d0d0d] rounded p-2 font-mono break-all">
              {totpSetup.qr_code_url}
            </code>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={totpCode}
                onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                maxLength={6}
                className="w-32 bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm font-mono text-center text-gray-100 focus:outline-none focus:border-blue-500"
              />
              <button
                onClick={handleVerifyTOTP}
                disabled={totpCode.length !== 6}
                className="px-3 py-2 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded-lg disabled:opacity-50"
              >
                Verify &amp; Enable
              </button>
              <button
                onClick={() => { setTotpSetup(null); setTotpCode(''); }}
                className="px-3 py-2 text-xs text-gray-400 hover:text-gray-200"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button
            onClick={handleSetupTOTP}
            disabled={totpLoading}
            className="flex items-center gap-2 px-3 py-2 text-sm bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg"
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
      await api.post('/api/v1/auth/users', {
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
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-6 w-full max-w-sm space-y-4">
        <h2 className="text-base font-semibold text-gray-100">Create User</h2>
        {[
          { label: 'Username', key: 'username', type: 'text', placeholder: 'operator1' },
          { label: 'Password', key: 'password', type: 'password', placeholder: '••••••••' },
        ].map(({ label, key, type, placeholder }) => (
          <div key={key}>
            <label className="block text-xs text-gray-400 mb-1">{label}</label>
            <input
              type={type}
              value={(form as any)[key]}
              onChange={e => setForm(p => ({ ...p, [key]: e.target.value }))}
              placeholder={placeholder}
              className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
            />
          </div>
        ))}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Role</label>
          <select
            value={form.roles}
            onChange={e => setForm(p => ({ ...p, roles: e.target.value }))}
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          >
            <option value="admin">Admin</option>
            <option value="operator">Operator</option>
            <option value="viewer">Viewer</option>
          </select>
        </div>
        <div className="flex gap-3 pt-2">
          <button onClick={onClose} className="flex-1 px-4 py-2 text-sm text-gray-300 bg-[#1a1a1a] hover:bg-[#252525] rounded-lg">
            Cancel
          </button>
          <button
            onClick={handleCreate}
            disabled={loading || !form.username || !form.password}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
          >
            {loading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            Create
          </button>
        </div>
      </div>
    </div>
  );
}

// ── TLS section ───────────────────────────────────────────────────────────────

function TLSSection() {
  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold text-gray-200 uppercase tracking-wider">TLS Certificates</h2>
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4 space-y-2">
        {[
          { label: 'Certificate Path', value: '/etc/games-dashboard/tls/server.crt', mono: true },
          { label: 'Key Path',         value: '/etc/games-dashboard/tls/server.key', mono: true },
          { label: 'Auto TLS',         value: 'Disabled' },
        ].map(({ label, value, mono }) => (
          <div key={label} className="flex items-center justify-between text-sm">
            <span className="text-gray-500">{label}</span>
            <span className={clsx('text-gray-300', mono && 'font-mono text-xs')}>{value}</span>
          </div>
        ))}
      </div>
      <div className="flex items-start gap-3 p-3 bg-[#141414] border border-[#252525] rounded-xl">
        <Key className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-xs text-gray-400">
          To update TLS certificates, replace the cert and key files then run{' '}
          <code className="font-mono text-gray-300">games-daemon --tls-cert /path/cert --tls-key /path/key</code>.
        </p>
      </div>
    </div>
  );
}

// ── Placeholder sections ──────────────────────────────────────────────────────

function PlaceholderSection({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold text-gray-200 uppercase tracking-wider">{title}</h2>
      <div className="bg-[#141414] border border-[#252525] rounded-xl divide-y divide-[#1e1e1e]">
        {items.map(item => (
          <div key={item} className="flex items-center justify-between px-4 py-3">
            <span className="text-sm text-gray-400">{item}</span>
            <span className="text-xs text-gray-600">Configure in daemon.yaml</span>
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
    <div className="flex h-full">
      {/* Left nav */}
      <aside className="w-48 shrink-0 border-r border-[#1a1a1a] p-4 space-y-1">
        {NAV.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            onClick={() => setSection(id)}
            className={clsx(
              'w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors text-left',
              section === id
                ? 'bg-blue-600/15 text-blue-400'
                : 'text-gray-400 hover:text-gray-200 hover:bg-[#1a1a1a]'
            )}
          >
            <Icon className="w-4 h-4 shrink-0" />
            {label}
          </button>
        ))}
      </aside>

      {/* Content */}
      <main className="flex-1 p-6 overflow-auto">
        {section === 'general'    && <GeneralSection />}
        {section === 'users'      && <UsersSection />}
        {section === 'tls'        && <TLSSection />}
        {section === 'storage'    && (
          <PlaceholderSection
            title="Storage"
            items={['Data Directory', 'NFS Mounts', 'S3 Endpoint', 'Backup Target', 'Retention Days']}
          />
        )}
        {section === 'networking' && (
          <PlaceholderSection
            title="Networking"
            items={['Port Exposure Mode', 'UPnP / NAT-PMP', 'Firewall Rules', 'Bind Address']}
          />
        )}
        {section === 'monitoring' && (
          <PlaceholderSection
            title="Monitoring"
            items={['Prometheus Path (/metrics)', 'Grafana Dashboard', 'Alert Rules', 'Log Level']}
          />
        )}
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
