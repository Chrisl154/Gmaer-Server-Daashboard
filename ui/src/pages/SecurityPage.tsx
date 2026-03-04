import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Shield, Key, Users, Lock, Eye, EyeOff, RefreshCw, Plus } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '../utils/api';
import { useAuthStore } from '../store/authStore';
import { clsx } from 'clsx';

export function SecurityPage() {
  const [activeSection, setActiveSection] = useState<'mfa'|'users'|'audit'|'secrets'>('mfa');
  const { user } = useAuthStore();

  const SECTIONS = [
    { id: 'mfa'     as const, label: 'MFA / 2FA',     icon: Shield },
    { id: 'users'   as const, label: 'Users & RBAC',  icon: Users  },
    { id: 'audit'   as const, label: 'Audit Log',     icon: Key    },
    { id: 'secrets' as const, label: 'Secrets',       icon: Lock   },
  ];

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Security</h1>
        <p className="text-sm text-gray-400 mt-1">Authentication, RBAC, audit trail, and secrets management</p>
      </div>

      <div className="flex gap-2 flex-wrap">
        {SECTIONS.map(({ id, label, icon: Icon }) => (
          <button key={id} onClick={() => setActiveSection(id)}
            className={clsx('flex items-center gap-2 px-4 py-2 rounded-lg text-sm transition-colors',
              activeSection === id ? 'bg-blue-600/20 text-blue-400 border border-blue-600/30' : 'bg-[#141414] border border-[#252525] text-gray-400 hover:text-gray-200')}>
            <Icon className="w-4 h-4" />{label}
          </button>
        ))}
      </div>

      {activeSection === 'mfa'     && <MFASection />}
      {activeSection === 'users'   && <UsersSection />}
      {activeSection === 'audit'   && <AuditSection />}
      {activeSection === 'secrets' && <SecretsSection />}
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
    <div className="max-w-md space-y-4">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h3 className="text-sm font-medium text-gray-200">Authenticator App (TOTP)</h3>
            <p className="text-xs text-gray-500 mt-0.5">Use Google Authenticator, Authy, or 1Password</p>
          </div>
          <div className={clsx('text-xs px-2 py-1 rounded-full', user?.totp_enabled ? 'bg-green-900/30 text-green-400' : 'bg-gray-800 text-gray-500')}>
            {user?.totp_enabled ? 'Enabled' : 'Disabled'}
          </div>
        </div>

        {!setupData ? (
          <button onClick={handleSetup} disabled={loading}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg text-sm transition-colors disabled:opacity-50">
            <Shield className="w-4 h-4" />
            {user?.totp_enabled ? 'Regenerate TOTP' : 'Enable TOTP'}
          </button>
        ) : (
          <div className="space-y-4">
            <div className="flex justify-center">
              <div className="p-3 bg-white rounded-xl">
                <QRCodeSVG value={setupData.qr_code_url} size={180} />
              </div>
            </div>
            <div className="p-3 bg-[#0d0d0d] rounded-lg">
              <p className="text-xs text-gray-400 mb-1">Manual entry key:</p>
              <p className="font-mono text-sm text-gray-200 break-all">{setupData.secret}</p>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Verify code from app</label>
              <div className="flex gap-2">
                <input type="text" value={verifyCode} onChange={e => setVerifyCode(e.target.value.replace(/\D/g,'').slice(0,6))}
                  placeholder="000000" maxLength={6}
                  className="flex-1 bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm font-mono text-gray-100 tracking-widest text-center focus:outline-none focus:border-blue-500" />
                <button onClick={handleVerify} disabled={verifyCode.length !== 6 || loading}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm disabled:opacity-50">
                  Verify
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
        <h3 className="text-sm font-medium text-gray-200 mb-3">OIDC / SSO</h3>
        <p className="text-xs text-gray-500">Configure single sign-on in Settings → Authentication.</p>
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

  const ROLE_COLORS: Record<string, string> = {
    admin:  'bg-red-900/30 text-red-400',
    viewer: 'bg-gray-800 text-gray-400',
    modder: 'bg-purple-900/30 text-purple-400',
    operator: 'bg-blue-900/30 text-blue-400',
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-200">Users ({users.length})</h3>
        <button className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg">
          <Plus className="w-3 h-3" /> Add User
        </button>
      </div>
      <div className="space-y-2">
        {users.map((u: any) => (
          <div key={u.id} className="bg-[#141414] border border-[#252525] rounded-xl p-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-blue-600/20 flex items-center justify-center text-sm font-medium text-blue-400">
                {u.username[0].toUpperCase()}
              </div>
              <div>
                <div className="text-sm text-gray-200">{u.username}</div>
                <div className="flex gap-1 mt-0.5">
                  {u.roles.map((r: string) => (
                    <span key={r} className={clsx('text-xs px-1.5 py-0.5 rounded', ROLE_COLORS[r] ?? 'bg-gray-800 text-gray-400')}>{r}</span>
                  ))}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {u.totp_enabled && <Shield className="w-4 h-4 text-green-400" title="MFA enabled" />}
              {u.username !== 'admin' && (
                <button onClick={() => deleteMutation.mutate(u.id)}
                  className="text-xs text-red-400 hover:text-red-300 px-2 py-1">Delete</button>
              )}
            </div>
          </div>
        ))}
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
    <div className="space-y-3">
      <h3 className="text-sm font-medium text-gray-200">Audit Log ({entries.length} entries)</h3>
      <div className="bg-[#141414] border border-[#252525] rounded-xl overflow-hidden">
        <div className="divide-y divide-[#1a1a1a] max-h-[500px] overflow-y-auto">
          {entries.length === 0 ? (
            <div className="p-6 text-center text-gray-500 text-sm">No audit events yet.</div>
          ) : entries.map((e: any) => (
            <div key={e.id} className="px-4 py-3 text-sm flex items-center gap-4">
              <div className={clsx('w-1.5 h-1.5 rounded-full shrink-0', e.success ? 'bg-green-400' : 'bg-red-400')} />
              <span className="text-gray-400 text-xs font-mono shrink-0">{new Date(e.timestamp).toLocaleString()}</span>
              <span className="text-gray-200">{e.username}</span>
              <span className="text-gray-500">{e.action}</span>
              <span className="text-gray-500 text-xs">{e.resource}</span>
            </div>
          ))}
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
    <div className="max-w-md space-y-4">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
        <h3 className="text-sm font-medium text-gray-200 mb-1">Encryption at Rest</h3>
        <p className="text-xs text-gray-500 mb-3">All secrets encrypted with AES-256-GCM. Local KMS by default; HashiCorp Vault supported.</p>
        <div className="flex items-center gap-2 text-xs text-green-400">
          <Shield className="w-3.5 h-3.5" /> AES-256-GCM active
        </div>
      </div>
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
        <h3 className="text-sm font-medium text-gray-200 mb-1">Key Rotation</h3>
        <p className="text-xs text-gray-500 mb-3">Rotate the master encryption key. All secrets will be re-encrypted with the new key.</p>
        {!showRotate ? (
          <button onClick={() => setShowRotate(true)} className="flex items-center gap-2 px-4 py-2 bg-yellow-900/20 hover:bg-yellow-900/30 text-yellow-400 rounded-lg text-sm">
            <RefreshCw className="w-4 h-4" /> Rotate Keys
          </button>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">
              Warning: This will rotate all encryption keys. Ensure you have a backup before proceeding.
            </p>
            <div className="flex gap-2">
              <button onClick={() => setShowRotate(false)} className="flex-1 px-3 py-2 text-sm text-gray-300 bg-[#1a1a1a] rounded-lg">Cancel</button>
              <button onClick={() => rotateMutation.mutate()} disabled={rotateMutation.isPending}
                className="flex-1 px-3 py-2 text-sm text-white bg-red-700 hover:bg-red-800 rounded-lg disabled:opacity-50">
                {rotateMutation.isPending ? 'Rotating...' : 'Confirm Rotate'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
