import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Package, Plus, Trash2, FlaskConical, RotateCcw, ChevronDown, ChevronRight, Loader2, CheckCircle, XCircle } from 'lucide-react';
import { clsx } from 'clsx';
import { api } from '../utils/api';
import { useMods, useInstallMod, useUninstallMod, useRunModTests, useRollbackMods } from '../hooks/useServers';
import type { Server, Mod, ModTestSuiteResult } from '../types';

const SOURCE_LABELS: Record<string, string> = {
  steam:       'Steam',
  curseforge:  'CurseForge',
  modrinth:    'Modrinth',
  thunderstore: 'Thunderstore',
  git:         'Git',
  local:       'Local',
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
    <div className="bg-[#141414] border border-[#252525] rounded-xl overflow-hidden">
      {/* Header */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-[#1a1a1a] transition-colors"
        onClick={() => setExpanded(e => !e)}
      >
        <div className="flex items-center gap-3">
          {expanded ? <ChevronDown className="w-4 h-4 text-gray-500" /> : <ChevronRight className="w-4 h-4 text-gray-500" />}
          <Package className="w-4 h-4 text-gray-400" />
          <div>
            <span className="text-sm font-medium text-gray-100">{server.name}</span>
            <span className="ml-2 text-xs text-gray-500 capitalize">{server.adapter}</span>
          </div>
        </div>
        {expanded && mods.length > 0 && (
          <span className="text-xs text-gray-500">{mods.length} mod{mods.length !== 1 ? 's' : ''}</span>
        )}
      </div>

      {expanded && (
        <div className="border-t border-[#1a1a1a] px-4 pb-4 pt-3 space-y-3">
          {/* Actions */}
          <div className="flex items-center gap-2 flex-wrap">
            <button
              onClick={() => setShowInstall(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg"
            >
              <Plus className="w-3 h-3" /> Add Mod
            </button>
            <button
              onClick={handleRunTests}
              disabled={runTestsMutation.isPending || mods.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg disabled:opacity-50"
            >
              {runTestsMutation.isPending ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <FlaskConical className="w-3 h-3" />
              )}
              Run Tests
            </button>
            <button
              onClick={() => rollbackMutation.mutate(undefined)}
              disabled={rollbackMutation.isPending || mods.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg disabled:opacity-50"
            >
              <RotateCcw className="w-3 h-3" /> Rollback
            </button>
          </div>

          {/* Test results */}
          {testResult && (
            <div className={clsx(
              'rounded-lg p-3 text-xs border',
              testResult.passed
                ? 'bg-green-900/10 border-green-900/30 text-green-300'
                : 'bg-red-900/10 border-red-900/30 text-red-300'
            )}>
              <div className="flex items-center gap-2 font-medium mb-2">
                {testResult.passed ? (
                  <><CheckCircle className="w-3.5 h-3.5" /> All tests passed</>
                ) : (
                  <><XCircle className="w-3.5 h-3.5" /> Tests failed</>
                )}
              </div>
              <div className="space-y-1">
                {testResult.tests.map(t => (
                  <div key={t.name} className="flex items-center gap-2">
                    {t.passed ? (
                      <CheckCircle className="w-3 h-3 text-green-400 shrink-0" />
                    ) : (
                      <XCircle className="w-3 h-3 text-red-400 shrink-0" />
                    )}
                    <span className="text-gray-300 font-mono">{t.name}</span>
                    {!t.passed && <span className="text-red-400 truncate">{t.message}</span>}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Mod list */}
          {isLoading ? (
            <div className="flex items-center gap-2 text-gray-500 text-sm py-2">
              <Loader2 className="w-4 h-4 animate-spin" /> Loading mods...
            </div>
          ) : mods.length === 0 ? (
            <p className="text-gray-500 text-sm">No mods installed.</p>
          ) : (
            <div className="space-y-1.5">
              {mods.map(m => (
                <div
                  key={m.id}
                  className="flex items-center justify-between bg-[#0d0d0d] border border-[#1e1e1e] rounded-lg px-3 py-2"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className={clsx(
                      'w-1.5 h-1.5 rounded-full shrink-0',
                      m.enabled ? 'bg-green-400' : 'bg-gray-600'
                    )} />
                    <div className="min-w-0">
                      <span className="text-sm text-gray-200 truncate block">{m.name}</span>
                      <div className="flex items-center gap-2 text-xs text-gray-500">
                        <span className="font-mono">{m.version}</span>
                        <span>·</span>
                        <span>{SOURCE_LABELS[m.source] ?? m.source}</span>
                        {m.size_bytes && <><span>·</span><span>{formatBytes(m.size_bytes)}</span></>}
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => uninstallMutation.mutate(m.id)}
                    disabled={uninstallMutation.isPending}
                    className="p-1.5 text-gray-500 hover:text-red-400 hover:bg-red-900/10 rounded-lg transition-colors disabled:opacity-50 shrink-0"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
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
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-6 w-full max-w-md space-y-4">
        <h2 className="text-base font-semibold text-gray-100">Install Mod</h2>

        <div>
          <label className="block text-xs text-gray-400 mb-1">Source</label>
          <select
            value={form.source}
            onChange={e => setForm(p => ({ ...p, source: e.target.value }))}
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          >
            {Object.entries(SOURCE_LABELS).map(([v, l]) => (
              <option key={v} value={v}>{l}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-xs text-gray-400 mb-1">Mod ID</label>
          <input
            type="text"
            value={form.mod_id}
            onChange={e => setForm(p => ({ ...p, mod_id: e.target.value }))}
            placeholder="e.g. 123456 or namespace-modname"
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label className="block text-xs text-gray-400 mb-1">Version (optional)</label>
          <input
            type="text"
            value={form.version}
            onChange={e => setForm(p => ({ ...p, version: e.target.value }))}
            placeholder="latest"
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          />
        </div>

        {(form.source === 'local' || form.source === 'git') && (
          <div>
            <label className="block text-xs text-gray-400 mb-1">
              {form.source === 'git' ? 'Repository URL' : 'Local Path'}
            </label>
            <input
              type="text"
              value={form.source_url}
              onChange={e => setForm(p => ({ ...p, source_url: e.target.value }))}
              placeholder={form.source === 'git' ? 'https://github.com/...' : '/opt/mods/mymod.zip'}
              className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
            />
          </div>
        )}

        <div className="flex gap-3 pt-2">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 text-sm text-gray-300 bg-[#1a1a1a] hover:bg-[#252525] rounded-lg"
          >
            Cancel
          </button>
          <button
            onClick={handleInstall}
            disabled={installMutation.isPending || !form.mod_id}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
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
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Mods</h1>
        <p className="text-sm text-gray-400 mt-1">
          Manage mods across all servers. Supports Steam, CurseForge, Modrinth, Thunderstore, Git, and local uploads.
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2].map(i => (
            <div key={i} className="h-16 bg-[#141414] border border-[#252525] rounded-xl animate-pulse" />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-center">
          <div className="w-14 h-14 bg-[#1a1a1a] rounded-2xl flex items-center justify-center mb-4">
            <Package className="w-7 h-7 text-gray-500" />
          </div>
          <h3 className="text-gray-200 font-medium mb-2">No servers yet</h3>
          <p className="text-gray-500 text-sm max-w-xs">
            Add a server first to start managing mods.
          </p>
        </div>
      ) : (
        <div className="space-y-3 max-w-3xl">
          {servers.map(s => (
            <ServerModCard key={s.id} server={s} />
          ))}
        </div>
      )}
    </div>
  );
}
