import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Package, Plus, Trash2, FlaskConical, RotateCcw,
  ChevronDown, ChevronRight, Loader2, CheckCircle, XCircle,
} from 'lucide-react';
import { cn } from '../utils/cn';
import { api } from '../utils/api';
import { useMods, useInstallMod, useUninstallMod, useRunModTests, useRollbackMods } from '../hooks/useServers';
import type { Server, Mod, ModTestSuiteResult } from '../types';

const SOURCE_LABELS: Record<string, string> = {
  steam:        'Steam',
  curseforge:   'CurseForge',
  modrinth:     'Modrinth',
  thunderstore: 'Thunderstore',
  git:          'Git',
  local:        'Local',
};

// source → badge style
const SOURCE_STYLES: Record<string, React.CSSProperties> = {
  steam:        { background: 'rgba(59,130,246,0.15)',  color: '#60a5fa'  },
  curseforge:   { background: 'rgba(249,115,22,0.15)',  color: '#fb923c'  },
  thunderstore: { background: 'rgba(34,197,94,0.15)',   color: '#4ade80'  },
  git:          { background: 'rgba(168,85,247,0.15)',  color: '#c084fc'  },
  local:        { background: 'rgba(128,128,168,0.12)', color: '#8080a8'  },
  modrinth:     { background: 'rgba(34,197,94,0.15)',   color: '#4ade80'  },
};

function formatBytes(bytes?: number) {
  if (!bytes) return '—';
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
}

function ServerModCard({ server }: { server: Server }) {
  const [expanded, setExpanded] = useState(false);
  const [showInstall, setShowInstall] = useState(false);
  const [testResult, setTestResult] = useState<ModTestSuiteResult | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['mods', server.id],
    queryFn: () => api.get(`/api/v1/servers/${server.id}/mods`).then(r => r.data),
    enabled: expanded,
  });

  const uninstallMutation = useUninstallMod(server.id);
  const runTestsMutation = useRunModTests(server.id);
  const rollbackMutation = useRollbackMods(server.id);

  const mods: Mod[] = data?.mods ?? [];

  const handleRunTests = async () => {
    const result = await runTestsMutation.mutateAsync();
    setTestResult(result);
  };

  return (
    <div className="card overflow-hidden">
      {/* Header */}
      <div
        className="flex items-center justify-between p-5 cursor-pointer transition-colors"
        style={{ borderBottom: expanded ? '1px solid var(--border)' : undefined }}
        onClick={() => setExpanded(e => !e)}
        onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
        onMouseLeave={e => (e.currentTarget.style.background = '')}
      >
        <div className="flex items-center gap-3">
          <div className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0 transition-transform"
            style={{ background: expanded ? 'var(--primary-subtle)' : 'var(--bg-elevated)' }}>
            {expanded
              ? <ChevronDown className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
              : <ChevronRight className="w-3.5 h-3.5" style={{ color: 'var(--text-muted)' }} />
            }
          </div>
          <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            <Package className="w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
          </div>
          <div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{server.name}</span>
            <span className="ml-2 text-xs capitalize badge"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
              {server.adapter}
            </span>
          </div>
        </div>
        {expanded && mods.length > 0 && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {mods.length} mod{mods.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {expanded && (
        <div className="p-5 space-y-4">
          {/* Actions */}
          <div className="flex items-center gap-2 flex-wrap">
            <button
              onClick={() => setShowInstall(true)}
              className="btn-primary py-1.5 px-3 text-xs"
            >
              <Plus className="w-3.5 h-3.5" /> Add Mod
            </button>
            <button
              onClick={handleRunTests}
              disabled={runTestsMutation.isPending || mods.length === 0}
              className="btn-ghost py-1.5 px-3 text-xs"
            >
              {runTestsMutation.isPending
                ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
                : <FlaskConical className="w-3.5 h-3.5" />
              }
              Run Tests
            </button>
            <button
              onClick={() => rollbackMutation.mutate(undefined)}
              disabled={rollbackMutation.isPending || mods.length === 0}
              className="btn-ghost py-1.5 px-3 text-xs"
            >
              <RotateCcw className="w-3.5 h-3.5" /> Rollback
            </button>
          </div>

          {/* Test results */}
          {testResult && (
            <div className={cn(
              'rounded-xl p-4 space-y-3',
              testResult.passed
                ? 'bg-green-500/5 border border-green-500/20'
                : 'bg-red-500/5 border border-red-500/20'
            )}>
              <div className={cn(
                'flex items-center gap-2 text-sm font-medium',
                testResult.passed ? 'text-green-400' : 'text-red-400'
              )}>
                {testResult.passed
                  ? <><CheckCircle className="w-4 h-4" /> All tests passed</>
                  : <><XCircle className="w-4 h-4" /> Tests failed</>
                }
              </div>
              <div className="space-y-1.5">
                {testResult.tests.map(t => (
                  <div key={t.name} className="flex items-center gap-2 text-xs">
                    {t.passed
                      ? <CheckCircle className="w-3 h-3 text-green-400 shrink-0" />
                      : <XCircle className="w-3 h-3 text-red-400 shrink-0" />
                    }
                    <span className="font-mono" style={{ color: 'var(--text-secondary)' }}>{t.name}</span>
                    {!t.passed && <span className="text-red-400 truncate">{t.message}</span>}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Mod grid */}
          {isLoading ? (
            <div className="flex items-center gap-2 text-sm py-2" style={{ color: 'var(--text-muted)' }}>
              <Loader2 className="w-4 h-4 animate-spin" /> Loading mods...
            </div>
          ) : mods.length === 0 ? (
            <div className="py-8 text-center">
              <Package className="w-8 h-8 mx-auto mb-2" style={{ color: 'var(--text-muted)' }} />
              <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No mods installed.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {mods.map(m => (
                <div key={m.id}
                  className="group relative rounded-xl p-4 transition-colors"
                  style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
                  onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--border-strong)')}
                  onMouseLeave={e => (e.currentTarget.style.borderColor = 'var(--border)')}
                >
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <div className={cn(
                        'w-2 h-2 rounded-full shrink-0 mt-0.5',
                        m.enabled ? 'bg-green-400' : 'bg-gray-600'
                      )} />
                      <span className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                        {m.name}
                      </span>
                    </div>
                    <button
                      onClick={() => uninstallMutation.mutate(m.id)}
                      disabled={uninstallMutation.isPending}
                      className="opacity-0 group-hover:opacity-100 p-1 rounded-lg transition-all disabled:opacity-30"
                      style={{ color: 'var(--text-muted)' }}
                      onMouseEnter={e => (e.currentTarget.style.color = '#f87171')}
                      onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>

                  <div className="flex items-center gap-2 flex-wrap mt-1">
                    <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>{m.version}</span>
                    <span className="badge text-[10px]"
                      style={SOURCE_STYLES[m.source] ?? { background: 'rgba(128,128,168,0.15)', color: 'var(--text-secondary)' }}>
                      {SOURCE_LABELS[m.source] ?? m.source}
                    </span>
                    {m.size_bytes && (
                      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>{formatBytes(m.size_bytes)}</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}

          {showInstall && (
            <InstallModModal
              serverId={server.id}
              onClose={() => setShowInstall(false)}
            />
          )}
        </div>
      )}
    </div>
  );
}

function InstallModModal({ serverId, onClose }: { serverId: string; onClose: () => void }) {
  const [form, setForm] = useState({ source: 'local', mod_id: '', version: '', source_url: '' });
  const installMutation = useInstallMod(serverId);

  const handleInstall = async () => {
    await installMutation.mutateAsync({
      source: form.source as any,
      mod_id: form.mod_id,
      version: form.version || undefined,
      source_url: form.source_url || undefined,
    });
    onClose();
  };

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50 p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-md rounded-2xl p-6 space-y-5"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Install Mod</h2>

        <div>
          <label className="label">Source</label>
          <select
            value={form.source}
            onChange={e => setForm(p => ({ ...p, source: e.target.value }))}
            className="input"
          >
            {Object.entries(SOURCE_LABELS).map(([v, l]) => (
              <option key={v} value={v}>{l}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="label">Mod ID</label>
          <input
            type="text"
            value={form.mod_id}
            onChange={e => setForm(p => ({ ...p, mod_id: e.target.value }))}
            placeholder="e.g. 123456 or namespace-modname"
            className="input"
          />
        </div>

        <div>
          <label className="label">Version (optional)</label>
          <input
            type="text"
            value={form.version}
            onChange={e => setForm(p => ({ ...p, version: e.target.value }))}
            placeholder="latest"
            className="input"
          />
        </div>

        {(form.source === 'local' || form.source === 'git') && (
          <div>
            <label className="label">{form.source === 'git' ? 'Repository URL' : 'Local Path'}</label>
            <input
              type="text"
              value={form.source_url}
              onChange={e => setForm(p => ({ ...p, source_url: e.target.value }))}
              placeholder={form.source === 'git' ? 'https://github.com/...' : '/opt/mods/mymod.zip'}
              className="input"
            />
          </div>
        )}

        <div className="flex gap-3 pt-1">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={handleInstall}
            disabled={installMutation.isPending || !form.mod_id}
            className="btn-primary flex-1 justify-center"
          >
            {installMutation.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            Install
          </button>
        </div>
      </div>
    </div>
  );
}

export function ModsPage() {
  const { data, isLoading } = useQuery<{ servers: Server[] }>({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });

  const servers = data?.servers ?? [];

  return (
    <div className="p-6 md:p-8 animate-page">
      <div className="mb-6">
        <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Mods</h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Manage mods across all servers. Supports Steam, CurseForge, Modrinth, Thunderstore, Git, and local uploads.
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="card h-[68px] animate-pulse" />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <div className="card p-12 flex flex-col items-center text-center">
          <div className="w-14 h-14 rounded-2xl flex items-center justify-center mb-4"
            style={{ background: 'var(--bg-elevated)' }}>
            <Package className="w-7 h-7" style={{ color: 'var(--text-muted)' }} />
          </div>
          <h3 className="font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>No servers yet</h3>
          <p className="text-sm max-w-xs" style={{ color: 'var(--text-secondary)' }}>
            Add a server first to start managing mods.
          </p>
        </div>
      ) : (
        <div className="space-y-3 max-w-4xl">
          {servers.map(s => (
            <ServerModCard key={s.id} server={s} />
          ))}
        </div>
      )}
    </div>
  );
}
